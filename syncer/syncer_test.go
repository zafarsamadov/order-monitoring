package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"order-monitoring/dona"
	"order-monitoring/models"
	"order-monitoring/storage/postgres"
)

// Bu testlar real Postgres talab qiladi: TEST_DATABASE_URL berilmasa o'tkazib yuboriladi.
// Har test o'z sxemasida ishlaydi va oxirida uni o'chiradi.
func newTestStore(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL berilmagan")
	}
	ctx := context.Background()

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	admin, err := postgres.NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Pool().Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	store, err := postgres.NewPostgres(ctx, dsn+sep+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		store.CloseDB()
		_, _ = admin.Pool().Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.CloseDB()
	})
	return store
}

// fakeDona — limit/offset bilan sahifalaydigan soxta stend.
type fakeDona struct {
	mu         sync.Mutex
	orders     []map[string]any
	ordersHits int
	fail500    bool          // birinchi /orders so'roviga 500 qaytarish
	block      chan struct{} // berilsa, /orders shu yopilguncha kutadi
	started    chan struct{}
}

func (f *fakeDona) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.URL.Path != "/auth/login" {
		http.Error(w, "faqat GET", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	switch r.URL.Path {
	case "/auth/login":
		writeJSON(w, map[string]any{"token": "tok"})
	case "/orders":
		f.mu.Lock()
		f.ordersHits++
		hit := f.ordersHits
		f.mu.Unlock()
		if f.block != nil {
			if hit == 1 {
				close(f.started)
			}
			<-f.block
		}
		if f.fail500 && hit == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		f.mu.Lock()
		writeJSON(w, map[string]any{"orders": page(f.orders, limit, offset), "total": len(f.orders)})
		f.mu.Unlock()
	case "/packages":
		pkgs := []map[string]any{
			{"id": "p1", "package_no": "PKG-1", "order_group_id": "o1", "status": "RETURNED_TO_WAREHOUSE", "shelf_code": "RET-01", "packed_at": "2026-11-11T10:00:00+05:00"},
			{"id": "p2", "package_no": "PKG-2", "order_group_id": "o2", "status": "PACKED", "packed_at": ""},
		}
		writeJSON(w, map[string]any{"packages": page(pkgs, limit, offset), "total": len(pkgs)})
	case "/inbound/handovers":
		hs := []map[string]any{
			{"id": "h1", "handover_no": "HND-1", "order_group_id": "o1", "status": "RECEIVED", "item_count": 1, "expected_qty": 1, "received_at": "2026-11-11T10:36:37+05:00"},
		}
		writeJSON(w, map[string]any{"handovers": page(hs, limit, offset), "total": len(hs)})
	case "/shelves":
		writeJSON(w, map[string]any{
			"shelves": []map[string]any{{
				"location_id": "s1", "code": "1", "kind": "SORT_SHELF", "oldest_age_seconds": time.Now().UnixNano(),
				"tiles": []map[string]any{{"package_id": "p1", "order_group_id": "o1", "sorted_at": "2026-11-11T16:00:00+05:00"}},
			}},
			"returns_shelf": map[string]any{"location_id": "s-ret", "code": "RET-01", "kind": "RETURNS"},
		})
	default:
		http.NotFound(w, r)
	}
}

func page(items []map[string]any, limit, offset int) []map[string]any {
	if offset >= len(items) {
		return []map[string]any{}
	}
	end := min(offset+limit, len(items))
	return items[offset:end]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func makeOrders(n int) []map[string]any {
	out := make([]map[string]any, n)
	for i := range out {
		out[i] = map[string]any{
			"id":               fmt.Sprintf("o%d", i+1),
			"group_no":         fmt.Sprintf("GRP-%d", i+1),
			"warehouse_status": "READY_FOR_ASSEMBLY",
			"lifecycle":        "ACCEPTED",
			"created_at":       "2026-11-11T12:00:00+05:00",
			"location":         nil,
		}
	}
	return out
}

func newTestSyncer(t *testing.T, f *fakeDona, store *postgres.Store) *Syncer {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	client := dona.New(dona.Options{BaseURL: srv.URL, Email: "e", Password: "p", Timeout: 5 * time.Second, MaxRetries: 3})
	return New(client, store.Sync(), Options{PageSize: 3})
}

func countRows(t *testing.T, store *postgres.Store, table string) int {
	t.Helper()
	var n int
	if err := store.Pool().QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Pagination, 500 dan keyin retry, takroriy sync'da dublikat yo'qligi va o'zgargan yozuv yangilanishi.
func TestRepeatedSync(t *testing.T) {
	store := newTestStore(t)
	f := &fakeDona{orders: makeOrders(7), fail500: true}
	s := newTestSyncer(t, f, store)
	ctx := context.Background()

	// 1-sync: hammasi yangi. 7 order (3 sahifa) + 2 package + 1 handover + 2 javon + 1 tile.
	run, err := s.Run(ctx)
	if err != nil {
		t.Fatalf("1-sync: %v", err)
	}
	if run.Inserted != 13 || run.Updated != 0 {
		t.Fatalf("1-sync: kutilgan inserted=13, bo'ldi %+v", run.UpsertStats)
	}
	// 500 + 3 sahifa (3+3+1 order, oxirgisi to'liq bo'lmagani uchun to'xtaydi).
	if f.ordersHits != 4 {
		t.Fatalf("500 dan keyin retry va 3 sahifa kutilgan edi, /orders so'rovlari: %d", f.ordersHits)
	}
	if got := countRows(t, store, "orders"); got != 7 {
		t.Fatalf("orders: 7 kutilgan, bor %d", got)
	}

	// 2-sync: hech narsa o'zgarmagan — dublikat yo'q, hammasi unchanged.
	// (javonning oldest_age_seconds maydoni o'zgaradi, lekin u hash'ga kirmaydi.)
	run, err = s.Run(ctx)
	if err != nil {
		t.Fatalf("2-sync: %v", err)
	}
	if run.Inserted != 0 || run.Updated != 0 || run.Unchanged != 13 {
		t.Fatalf("2-sync: kutilgan 0/0/13, bo'ldi %+v", run.UpsertStats)
	}
	for table, want := range map[string]int{"orders": 7, "packages": 2, "handovers": 1, "shelves": 2, "shelf_tiles": 1} {
		if got := countRows(t, store, table); got != want {
			t.Fatalf("%s: %d kutilgan, bor %d", table, want, got)
		}
	}

	// 3-sync: bitta order o'zgardi — faqat u yangilanadi, first_seen_at saqlanadi.
	var firstSeenBefore time.Time
	if err := store.Pool().QueryRow(ctx, "SELECT first_seen_at FROM orders WHERE id = 'o2'").Scan(&firstSeenBefore); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.orders[1]["warehouse_status"] = "READY_TO_SHIP"
	f.mu.Unlock()

	run, err = s.Run(ctx)
	if err != nil {
		t.Fatalf("3-sync: %v", err)
	}
	if run.Inserted != 0 || run.Updated != 1 || run.Unchanged != 12 {
		t.Fatalf("3-sync: kutilgan 0/1/12, bo'ldi %+v", run.UpsertStats)
	}
	var status string
	var firstSeenAfter time.Time
	if err := store.Pool().QueryRow(ctx,
		"SELECT warehouse_status, first_seen_at FROM orders WHERE id = 'o2'").Scan(&status, &firstSeenAfter); err != nil {
		t.Fatal(err)
	}
	if status != "READY_TO_SHIP" || !firstSeenAfter.Equal(firstSeenBefore) {
		t.Fatalf("o2: status=%s (READY_TO_SHIP kutilgan), first_seen o'zgarmasligi kerak", status)
	}
	if got := countRows(t, store, "orders"); got != 7 {
		t.Fatalf("orders: 7 kutilgan, bor %d", got)
	}

	// sync_runs'da oxirgi run saqlangan.
	last, err := store.Sync().LastRun(ctx)
	if err != nil || last == nil || last.Status != models.SyncSuccess || last.Updated != 1 {
		t.Fatalf("LastRun: %+v, %v", last, err)
	}
}

func TestSecondSyncWhileRunningIsRejected(t *testing.T) {
	store := newTestStore(t)
	f := &fakeDona{orders: makeOrders(1), block: make(chan struct{}), started: make(chan struct{})}
	s := newTestSyncer(t, f, store)
	ctx := context.Background()

	done := make(chan error)
	go func() {
		_, err := s.Run(ctx)
		done <- err
	}()
	<-f.started

	if _, err := s.Run(ctx); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("ErrAlreadyRunning kutilgan edi, bo'ldi: %v", err)
	}

	close(f.block)
	if err := <-done; err != nil {
		t.Fatalf("1-sync: %v", err)
	}
}
