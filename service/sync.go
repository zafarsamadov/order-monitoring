package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"order-monitoring/logger"
	"order-monitoring/models"
	"order-monitoring/pkg"
	"order-monitoring/storage"
)

var ErrAlreadyRunning = errors.New("sync allaqachon ketmoqda")

// Fetcher — stenddan o'qish. dona.Client shuni bajaradi; testda soxta server ishlatiladi.
type Fetcher interface {
	Get(ctx context.Context, path string, query url.Values, out any) error
}

// syncService stenddan ma'lumotni sahifalab tortib, bazaga upsert qiladi.
type syncService struct {
	client  Fetcher
	store   storage.SyncRepoI
	opt     models.SyncOptions
	running atomic.Bool
}

func NewSyncService(client Fetcher, store storage.SyncRepoI, opt models.SyncOptions) *syncService {
	return &syncService{client: client, store: store, opt: opt}
}

// LastRun oxirgi sync natijasi (sync_runs). Faqat o'qiydi.
func (s *syncService) LastRun(ctx context.Context) (*models.SyncRun, error) {
	return s.store.LastRun(ctx, "")
}

// LastSuccess oxirgi muvaffaqiyatli sync — oxirgisi xato bo'lsa ham bazadagi ma'lumot qaysi sync'dan ekanini ko'rsatadi.
func (s *syncService) LastSuccess(ctx context.Context) (*models.SyncRun, error) {
	return s.store.LastRun(ctx, models.SyncSuccess)
}

// Running hozir sync ketayotganini bildiradi.
func (s *syncService) Running() bool {
	return s.running.Load()
}

func (s *syncService) pagedEntities() []models.PagedEntity {
	wh := url.Values{}
	if s.opt.Warehouse != "" {
		wh.Set("warehouse", s.opt.Warehouse)
	}
	return []models.PagedEntity{
		{Name: "orders", Path: "/orders", ItemsKey: "orders", ToRecord: orderRecord},
		{Name: "packages", Path: "/packages", ItemsKey: "packages", Query: wh, ToRecord: packageRecord},
		{Name: "handovers", Path: "/inbound/handovers", ItemsKey: "handovers", ToRecord: handoverRecord},
	}
}

// Run to'liq sync: orders, packages, handovers, keyin shelves snapshot.
// Bir vaqtda faqat bitta Run ishlaydi, ikkinchisi ErrAlreadyRunning oladi.
// Xato bo'lsa ham, shu paytgacha saqlangan sahifalar bazada qoladi.
func (s *syncService) Run(ctx context.Context) (*models.SyncRun, error) {
	if !s.running.CompareAndSwap(false, true) {
		return nil, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	run, err := s.store.StartRun(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync_runs: %w", err)
	}
	logger.Infof("sync #%d boshlandi", run.ID)

	runErr := s.runAll(ctx, run)

	now := time.Now()
	run.FinishedAt = &now
	run.Status = models.SyncSuccess
	if runErr != nil {
		run.Status = models.SyncFailed
		run.Error = runErr.Error()
	}
	run.Recount()

	// Run so'rovi bekor qilingan bo'lsa ham yakuniy holat yozilsin.
	if err := s.store.FinishRun(context.WithoutCancel(ctx), run); err != nil {
		logger.Errorf("sync #%d holatini saqlab bo'lmadi: %v", run.ID, err)
	}

	if runErr != nil {
		logger.Errorf("sync #%d xato bilan tugadi: %v", run.ID, runErr)
	} else {
		logger.Successf("sync #%d tugadi: yangi %d, yangilangan %d, o'zgarmagan %d",
			run.ID, run.Inserted, run.Updated, run.Unchanged)
	}
	return run, runErr
}

func (s *syncService) runAll(ctx context.Context, run *models.SyncRun) error {
	first := true
	for _, e := range s.pagedEntities() {
		if err := s.syncPaged(ctx, run, e, &first); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
	}
	if err := s.syncShelves(ctx, run, &first); err != nil {
		return fmt.Errorf("shelves: %w", err)
	}
	return nil
}

func (s *syncService) syncPaged(ctx context.Context, run *models.SyncRun, e models.PagedEntity, first *bool) error {
	run.Entities = append(run.Entities, models.EntityResult{Entity: e.Name})
	res := func() *models.EntityResult { return &run.Entities[len(run.Entities)-1] }

	offset := 0
	for {
		if err := s.pause(ctx, first); err != nil {
			return err
		}

		q := url.Values{}
		for k, v := range e.Query {
			q[k] = v
		}
		q.Set("limit", strconv.Itoa(s.opt.PageSize))
		q.Set("offset", strconv.Itoa(offset))

		var page map[string]json.RawMessage
		if err := s.client.Get(ctx, e.Path, q, &page); err != nil {
			return err
		}

		var items []json.RawMessage
		if raw, ok := page[e.ItemsKey]; ok && string(raw) != "null" {
			if err := json.Unmarshal(raw, &items); err != nil {
				return fmt.Errorf("%q massiv emas: %w", e.ItemsKey, err)
			}
		}
		if raw, ok := page["total"]; ok {
			var total int
			if json.Unmarshal(raw, &total) == nil {
				res().APITotal = &total
			}
		}

		records := make([]models.Record, 0, len(items))
		for _, item := range items {
			rec, err := e.ToRecord(item)
			if err != nil {
				res().Skipped++
				logger.Warnf("%s: yozuv o'tkazib yuborildi (offset %d): %v", e.Name, offset, err)
				continue
			}
			records = append(records, rec)
		}

		res().Fetched += len(items)
		res().LastOffset = offset
		run.LastEntity, run.LastOffset = e.Name, offset
		if _, err := s.store.SavePage(ctx, run, records); err != nil {
			return fmt.Errorf("offset %d saqlash: %w", offset, err)
		}

		offset += len(items)
		// To'xtash: bo'sh sahifa yoki total'ga yetdik. Qisqa sahifa faqat total yo'q bo'lsa tugash belgisi:
		// server limit'ni o'zi kesishi mumkin (50 so'rasak 20 beradi), unda davom etish kerak.
		total := res().APITotal
		if len(items) == 0 || (total != nil && offset >= *total) || (total == nil && len(items) < s.opt.PageSize) {
			break
		}
	}

	// Sync paytida yozuvlar qo'shilsa/o'chsa sahifalar siljiydi. Upsert dublikat yaratmaydi,
	// lekin farqni ko'rish uchun logga yozamiz.
	if t := res().APITotal; t != nil && res().Fetched != *t {
		logger.Warnf("%s: olingan %d ta, API total %d ta (sahifalar siljigan bo'lishi mumkin)",
			e.Name, res().Fetched, *t)
	}
	return nil
}

// specialShelfSuffix: /shelves javobida asosiy ro'yxatdan tashqari returns_shelf,
// retry_shelf, quarantine_shelf, cancel_hold_shelf kabi alohida obyektlar bor.
const specialShelfSuffix = "_shelf"

// syncShelves /shelves snapshot'ini (pagination yo'q) javonlar va qutilar (tiles) jadvaliga yozadi.
func (s *syncService) syncShelves(ctx context.Context, run *models.SyncRun, first *bool) error {
	run.Entities = append(run.Entities, models.EntityResult{Entity: "shelves"})
	res := &run.Entities[len(run.Entities)-1]

	if err := s.pause(ctx, first); err != nil {
		return err
	}
	q := url.Values{}
	if s.opt.Warehouse != "" {
		q.Set("warehouse", s.opt.Warehouse)
	}
	var page map[string]json.RawMessage
	if err := s.client.Get(ctx, "/shelves", q, &page); err != nil {
		return err
	}

	var shelvesRaw []json.RawMessage
	if raw, ok := page["shelves"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &shelvesRaw); err != nil {
			return fmt.Errorf(`"shelves" massiv emas: %w`, err)
		}
	}
	keys := make([]string, 0)
	for k := range page {
		if strings.HasSuffix(k, specialShelfSuffix) && string(page[k]) != "null" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		shelvesRaw = append(shelvesRaw, page[k])
	}

	var records []models.Record
	for _, raw := range shelvesRaw {
		var shelf models.DonaShelf
		if err := json.Unmarshal(raw, &shelf); err != nil {
			res.Skipped++
			logger.Warnf("shelves: javon o'tkazib yuborildi: %v", err)
			continue
		}
		rec, err := shelfRecord(raw, shelf)
		if err != nil {
			res.Skipped++
			logger.Warnf("shelves: javon %q o'tkazib yuborildi: %v", shelf.Code, err)
			continue
		}
		records = append(records, rec)

		for _, tile := range shelf.Tiles {
			rec, err := tileRecord(tile, shelf.Code)
			if err != nil {
				res.Skipped++
				logger.Warnf("shelves: javon %q qutisi o'tkazib yuborildi: %v", shelf.Code, err)
				continue
			}
			records = append(records, rec)
		}
	}

	res.Fetched = len(records) + res.Skipped
	run.LastEntity, run.LastOffset = "shelves", 0
	if _, err := s.store.SavePage(ctx, run, records); err != nil {
		return fmt.Errorf("saqlash: %w", err)
	}
	return nil
}

// pause so'rovlar orasida REQUEST_DELAY_MS kutadi (birinchi so'rovdan oldin emas):
// stend tez-tez so'rovdan yiqilib qolgan edi.
func (s *syncService) pause(ctx context.Context, first *bool) error {
	if *first {
		*first = false
		return nil
	}
	return pkg.Sleep(ctx, s.opt.RequestDelay)
}
