package anomalies_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"order-monitoring/models"
	"order-monitoring/storage"
	"order-monitoring/storage/postgres"
)

// Qoidalar testlari real Postgres'da ishlaydi: TEST_DATABASE_URL berilmasa o'tkazib yuboriladi.
// Har test o'z sxemasida: unga anomaliya qatorlari (kod "BAD-...") va normal qatorlar (kod "OK-...")
// yoziladi, qoida aynan BAD-larni qaytarishi tekshiriladi. OK qatorlar ichida biznes tahlilida
// yolg'on signal deb topilgan holatlar ham bor — ular qayta chiqib qolmasligi uchun.

func newRepo(t *testing.T, seed ...string) storage.AnomaliesRepoI {
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

	// Seed qisqa bo'lishi uchun: raw/raw_hash qoidalarga kerak emas, test sxemasida sukut beriladi.
	for _, table := range []string{"orders", "packages", "handovers", "shelves"} {
		seed = append([]string{fmt.Sprintf(
			`ALTER TABLE %s ALTER COLUMN raw SET DEFAULT '{}', ALTER COLUMN raw_hash SET DEFAULT ''`, table,
		)}, seed...)
	}
	for _, stmt := range seed {
		if _, err := store.Pool().Exec(ctx, stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
	return store.Anomalies()
}

// check qoida natijasini tekshiradi: aynan want orderlar (tartibsiz), Total va bo'sh bo'lmagan sabab.
// Qaytaradi: order kodi → sabab.
func check(t *testing.T, item *models.RuleItem, err error, want ...string) map[string]string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if item.Items == nil {
		t.Fatal("Items nil bo'lmasligi kerak (JSON'da [] chiqishi uchun)")
	}

	reasons := map[string]string{}
	var got []string
	for _, it := range item.Items {
		if it.Reason == "" {
			t.Fatalf("%s: sabab bo'sh", it.OrderCode)
		}
		got = append(got, it.OrderCode)
		reasons[it.OrderCode] = it.Reason
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s: kutilgan %v, bor %v", item.Rule, want, got)
	}
	if item.Total != len(want) {
		t.Fatalf("%s: Total %d, kutilgan %d", item.Rule, item.Total, len(want))
	}
	return reasons
}

func mustContain(t *testing.T, reason string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(reason, p) {
			t.Fatalf("sababda %q yo'q: %s", p, reason)
		}
	}
}

func TestIssuedOrderPackageReturned(t *testing.T) {
	repo := newRepo(t,
		// BAD: order topshirilgan, bitta qutisi return_id bilan omborga qaytgan.
		`INSERT INTO orders (id, order_code, lifecycle, lifecycle_outcome, warehouse_status, location_kind, created_at)
		 VALUES ('o1', 'BAD-1', 'OUT', 'ISSUED_TO_BUYER', 'READY_TO_SHIP', 'STAGING', '2026-11-10 10:00+05')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, return_id) VALUES
		 ('p1', 'PKG-1', 'o1', 'ISSUED_TO_BUYER', NULL),
		 ('p2', 'PKG-2', 'o1', 'RETURNED_TO_WAREHOUSE', 'r1')`,
		// OK: hamma qutisi topshirilgan.
		`INSERT INTO orders (id, order_code, lifecycle, lifecycle_outcome) VALUES ('o2', 'OK-ALL-ISSUED', 'OUT', 'ISSUED_TO_BUYER')`,
		`INSERT INTO packages (id, package_no, order_group_id, status) VALUES ('p3', 'PKG-3', 'o2', 'ISSUED_TO_BUYER')`,
		// OK: normal qaytish — order RETURN bosqichiga o'tgan.
		`INSERT INTO orders (id, order_code, lifecycle, lifecycle_outcome) VALUES ('o3', 'OK-RETURN', 'RETURN', 'RETURNED_TO_WAREHOUSE')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, return_id) VALUES ('p4', 'PKG-4', 'o3', 'RETURNED_TO_WAREHOUSE', 'r2')`,
	)

	item, err := repo.IssuedOrderPackageReturnedRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "PKG-2 RETURNED_TO_WAREHOUSE (return)", "warehouse_status=READY_TO_SHIP", "location=STAGING")
	if strings.Contains(reasons["BAD-1"], "PKG-1") {
		t.Fatalf("topshirilgan PKG-1 sababda bo'lmasligi kerak: %s", reasons["BAD-1"])
	}
}

func TestConflictingPackageStatuses(t *testing.T) {
	repo := newRepo(t,
		// BAD: kuryer qaytarishni olib kelyapti, qaytarilgan quti esa allaqachon sotuvchida.
		`INSERT INTO orders (id, order_code, lifecycle_outcome) VALUES ('o1', 'BAD-1', 'RETURN_TO_HUB')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, return_id) VALUES
		 ('p1', 'PKG-1', 'o1', 'ISSUED_TO_BUYER', NULL),
		 ('p2', 'PKG-2', 'o1', 'RETURNED_TO_SELLERS', 'r1')`,
		// OK: qisman qaytish — bitta quti xaridorda, ikkinchisi sotuvchida (eski ta'rif buni xato derdi).
		`INSERT INTO orders (id, order_code, lifecycle_outcome) VALUES ('o2', 'OK-PARTIAL-RETURN', 'RETURNED_TO_SELLERS')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, return_id) VALUES
		 ('p3', 'PKG-3', 'o2', 'ISSUED_TO_BUYER', NULL),
		 ('p4', 'PKG-4', 'o2', 'RETURNED_TO_SELLERS', 'r2')`,
		// OK: qaytish yo'lda, qaytarilgan quti hali sotuvchiga yetmagan.
		`INSERT INTO orders (id, order_code, lifecycle_outcome) VALUES ('o3', 'OK-ON-THE-WAY', 'RETURN_TO_HUB')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, return_id) VALUES ('p5', 'PKG-5', 'o3', 'REPACKED', 'r3')`,
	)

	item, err := repo.ConflictingPackageStatusesRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "RETURN_TO_HUB", "PKG-2 RETURNED_TO_SELLERS")
}

func TestReturnedToSellerInWarehouse(t *testing.T) {
	repo := newRepo(t,
		// BAD: sotuvchiga qaytarilgan quti hali javonda.
		`INSERT INTO orders (id, order_code) VALUES ('o1', 'BAD-1')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p1', 'PKG-1', 'o1', 'RETURNED_TO_SELLERS', 'RET-01')`,
		// OK: sotuvchiga qaytarilgan, javonsiz; order location STAGING — stend modeli, xato emas.
		`INSERT INTO orders (id, order_code, lifecycle_outcome, location_kind) VALUES ('o2', 'OK-STAGING', 'RETURNED_TO_SELLERS', 'STAGING')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p2', 'PKG-2', 'o2', 'RETURNED_TO_SELLERS', NULL)`,
		// OK: omborga qaytgan quti qaytarishlar javonida — tovar haqiqatan omborda.
		`INSERT INTO orders (id, order_code) VALUES ('o3', 'OK-IN-WAREHOUSE')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p3', 'PKG-3', 'o3', 'RETURNED_TO_WAREHOUSE', 'RET-01')`,
	)

	item, err := repo.ReturnedToSellerInWarehouseRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "PKG-1 RET-01")
}

func TestPackageOnUnknownShelf(t *testing.T) {
	repo := newRepo(t,
		`INSERT INTO shelves (location_id, code, kind) VALUES ('s1', 'RET-01', 'RETURNS'), ('s2', '0', 'SORT_SHELF')`,
		// BAD: omborda turgan quti ro'yxatda yo'q javonda.
		`INSERT INTO orders (id, order_code) VALUES ('o1', 'BAD-1')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p1', 'PKG-1', 'o1', 'RETURNED_TO_WAREHOUSE', 'RET-T')`,
		// OK: mavjud javonda.
		`INSERT INTO orders (id, order_code) VALUES ('o2', 'OK-KNOWN-SHELF')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p2', 'PKG-2', 'o2', 'SORTED', '0')`,
		// OK (bu qoida uchun): ombordan chiqib ketgan quti — uni returned_to_seller_in_warehouse ko'radi.
		`INSERT INTO orders (id, order_code) VALUES ('o3', 'OK-LEFT-WAREHOUSE')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p3', 'PKG-3', 'o3', 'RETURNED_TO_SELLERS', 'RET-T')`,
		// OK: javonsiz quti.
		`INSERT INTO orders (id, order_code) VALUES ('o4', 'OK-NO-SHELF')`,
		`INSERT INTO packages (id, package_no, order_group_id, status) VALUES ('p4', 'PKG-4', 'o4', 'RETURNED_TO_WAREHOUSE')`,
	)

	item, err := repo.PackageOnUnknownShelfRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "PKG-1 RET-T (RETURNED_TO_WAREHOUSE)")
}

func TestDispatchedPackageOnShelf(t *testing.T) {
	repo := newRepo(t,
		// BAD: jo'natilgan quti javonda qolgan.
		`INSERT INTO orders (id, order_code) VALUES ('o1', 'BAD-1')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p1', 'PKG-1', 'o1', 'DISPATCHED', '0')`,
		// OK: jo'natilgan va javonsiz.
		`INSERT INTO orders (id, order_code) VALUES ('o2', 'OK-DISPATCHED')`,
		`INSERT INTO packages (id, package_no, order_group_id, status) VALUES ('p2', 'PKG-2', 'o2', 'DISPATCHED')`,
		// OK: saralangan quti javonda turishi kerak.
		`INSERT INTO orders (id, order_code) VALUES ('o3', 'OK-SORTED')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code) VALUES ('p3', 'PKG-3', 'o3', 'SORTED', '0')`,
	)

	item, err := repo.DispatchedPackageOnShelfRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "status=DISPATCHED", "PKG-1 0")
}

func TestHandoverTimestampsOutOfOrder(t *testing.T) {
	repo := newRepo(t,
		`INSERT INTO orders (id, order_code) VALUES
		 ('o1', 'BAD-RECEIVED'), ('o2', 'BAD-TRANSIT'), ('o3', 'BAD-RECEIVED-TRANSIT'), ('o4', 'OK-ORDERED'), ('o5', 'OK-NULLS')`,
		`INSERT INTO handovers (id, handover_no, order_group_id, created_at, transit_since, received_at) VALUES
		 -- yaratilishidan 13 kun oldin qabul qilingan
		 ('h1', 'HND-1', 'o1', '2026-11-11 10:00+05', NULL,                  '2026-10-29 10:00+05'),
		 -- yaratilishidan 2 soat oldin yo'lga chiqqan
		 ('h2', 'HND-2', 'o2', '2026-11-11 10:00+05', '2026-11-11 08:00+05', NULL),
		 -- yo'lga chiqishidan oldin qabul qilingan
		 ('h3', 'HND-3', 'o3', '2026-11-11 10:00+05', '2026-11-11 12:00+05', '2026-11-11 11:00+05'),
		 -- to'g'ri tartib
		 ('h4', 'HND-4', 'o4', '2026-11-11 10:00+05', '2026-11-11 11:00+05', '2026-11-11 12:00+05'),
		 -- bo'sh vaqtlar solishtirilmaydi
		 ('h5', 'HND-5', 'o5', '2026-11-11 10:00+05', NULL,                  NULL),
		 -- orderi bazada yo'q handover: kod sifatida order_group_id chiqadi
		 ('h6', 'HND-6', 'BAD-ORPHAN', '2026-11-11 10:00+05', NULL,           '2026-11-10 10:00+05')`,
	)

	item, err := repo.HandoverTimestampsOutOfOrderRule(context.Background())
	reasons := check(t, item, err, "BAD-RECEIVED", "BAD-TRANSIT", "BAD-RECEIVED-TRANSIT", "BAD-ORPHAN")
	mustContain(t, reasons["BAD-RECEIVED"], "HND-1", "received_at created_at dan 13 kun 0 soat oldin")
	mustContain(t, reasons["BAD-TRANSIT"], "HND-2", "transit_since created_at dan 2 soat 0 daq oldin")
	mustContain(t, reasons["BAD-RECEIVED-TRANSIT"], "HND-3", "received_at transit_since dan 1 soat 0 daq oldin")
}

func TestActivityBeforeLifecycleStart(t *testing.T) {
	repo := newRepo(t,
		`INSERT INTO orders (id, order_code, lifecycle, lifecycle_since, last_activity_at) VALUES
		 -- 2 kun oldin
		 ('o1', 'BAD-1', 'RETURN', '2026-11-11 10:00+05', '2026-11-09 10:00+05'),
		 -- 30 soniya farq — bitta jarayonda yozilgan, tolerans ichida
		 ('o2', 'OK-WITHIN-TOLERANCE', 'RETURN', '2026-11-11 10:00:30+05', '2026-11-11 10:00:00+05'),
		 -- faollik bosqichdan keyin
		 ('o3', 'OK-AFTER', 'SORTING', '2026-11-11 10:00+05', '2026-11-11 11:00+05'),
		 -- vaqtlar yo'q
		 ('o4', 'OK-NULLS', 'AWAITING', NULL, NULL)`,
	)

	item, err := repo.ActivityBeforeLifecycleStartRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "lifecycle=RETURN", "2 kun 0 soat oldin")
}

func TestHandoverReceivedIncomplete(t *testing.T) {
	repo := newRepo(t,
		`INSERT INTO orders (id, order_code) VALUES
		 ('o1', 'BAD-1'), ('o2', 'OK-FULL'), ('o3', 'OK-OVER'), ('o4', 'OK-IN-TRANSIT')`,
		`INSERT INTO handovers (id, handover_no, order_group_id, status, item_count, expected_qty) VALUES
		 ('h1', 'HND-1', 'o1', 'RECEIVED',   1, 2),
		 ('h2', 'HND-2', 'o2', 'RECEIVED',   2, 2),
		 ('h3', 'HND-3', 'o3', 'RECEIVED',   3, 2),
		 -- yo'lda kamomad normal: qism-qism yuborilmoqda
		 ('h4', 'HND-4', 'o4', 'IN_TRANSIT', 1, 2)`,
	)

	item, err := repo.HandoverReceivedIncompleteRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "HND-1", "kelgan 1 / kutilgan 2")
}

func TestOrderItemCountMismatch(t *testing.T) {
	repo := newRepo(t,
		`INSERT INTO orders (id, order_code, item_count) VALUES
		 ('o1', 'BAD-1', 5), ('o2', 'OK-MATCH', 5), ('o3', 'OK-UNKNOWN', 0)`,
		`INSERT INTO packages (id, package_no, order_group_id, item_count) VALUES
		 ('p1', 'PKG-1', 'o1', 3), ('p2', 'PKG-2', 'o1', 1),
		 ('p3', 'PKG-3', 'o2', 3), ('p4', 'PKG-4', 'o2', 2),
		 -- order soni 0 = noma'lum: stend maydonni to'ldirmagan, xato emas
		 ('p5', 'PKG-5', 'o3', 2)`,
	)

	item, err := repo.OrderItemCountMismatchRule(context.Background())
	reasons := check(t, item, err, "BAD-1")
	mustContain(t, reasons["BAD-1"], "orderda 5 ta tovar", "2 ta qutida jami 4 ta tovar")
}

// Bo'sh bazada hamma qoida xatosiz bo'sh ro'yxat qaytaradi (nil emas).
func TestAllRulesOnEmptyDatabase(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	rules := []func(context.Context) (*models.RuleItem, error){
		repo.IssuedOrderPackageReturnedRule,
		repo.ConflictingPackageStatusesRule,
		repo.ReturnedToSellerInWarehouseRule,
		repo.PackageOnUnknownShelfRule,
		repo.DispatchedPackageOnShelfRule,
		repo.HandoverTimestampsOutOfOrderRule,
		repo.ActivityBeforeLifecycleStartRule,
		repo.HandoverReceivedIncompleteRule,
		repo.OrderItemCountMismatchRule,
	}
	for _, rule := range rules {
		item, err := rule(ctx)
		check(t, item, err)
		if item.Rule == "" || item.Description == "" || item.Severity == "" {
			t.Fatalf("qoida nomi, izohi va severity'si bo'lishi kerak: %+v", item)
		}
	}
}
