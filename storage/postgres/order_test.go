package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"order-monitoring/storage"
)

// TEST_DATABASE_URL berilmasa o'tkazib yuboriladi. Har test o'z sxemasida ishlaydi.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL berilmagan")
	}
	ctx := context.Background()

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	admin, err := NewPostgres(ctx, dsn)
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
	store, err := NewPostgres(ctx, dsn+sep+"search_path="+schema)
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

// seedTimeline: o1 (order_code bor, javonda, 1 quti + tile, 1 handover), o2 (faqat group_no, joylashuvsiz).
// o2 ning qutisi va handover'i o1 timeline'iga tushmasligi kerak.
func seedTimeline(t *testing.T, store *Store) {
	t.Helper()
	stmts := []string{
		`INSERT INTO orders (id, order_code, group_no, lifecycle, warehouse_status, location_kind, location_label,
			created_at, lifecycle_since, location_since, last_activity_at, raw, raw_hash)
		 VALUES ('o1', 'AB123', 'TESTPM-AB123', 'SORTING', 'READY_TO_SHIP', 'SHELF', '1',
			'2026-11-11 10:00+05', '2026-11-11 10:30+05', '2026-11-11 10:30+05', '2026-11-11 10:30+05',
			'{"current_step":"Ready to ship"}', 'h')`,
		`INSERT INTO orders (id, group_no, lifecycle, created_at, raw, raw_hash)
		 VALUES ('o2', 'GRP-2', 'AWAITING', '2026-11-11 09:00+05', '{}', 'h')`,
		`INSERT INTO packages (id, package_no, order_group_id, status, shelf_code, packed_at, raw, raw_hash)
		 VALUES ('p1', 'PKG-1', 'o1', 'SORTED', '1', '2026-11-11 10:10+05', '{}', 'h'),
		        ('p2', 'PKG-2', 'o2', 'SORTED', '0', '2026-11-11 09:10+05', '{}', 'h')`,
		`INSERT INTO shelf_tiles (package_id, shelf_code, order_group_id, sorted_at, raw, raw_hash)
		 VALUES ('p1', '1', 'o1', '2026-11-11 10:20+05', '{}', 'h')`,
		`INSERT INTO handovers (id, handover_no, order_group_id, status, item_count, expected_qty, created_at, received_at, raw, raw_hash)
		 VALUES ('h1', 'HND-1', 'o1', 'RECEIVED', 1, 1, '2026-11-11 10:01+05', '2026-11-11 10:05+05', '{}', 'h'),
		        ('h2', 'HND-2', 'o2', 'IN_TRANSIT', 1, 1, '2026-11-11 09:01+05', NULL, '{}', 'h')`,
	}
	for _, s := range stmts {
		if _, err := store.Pool().Exec(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetByCode(t *testing.T) {
	store := newTestStore(t)
	seedTimeline(t, store)
	ctx := context.Background()

	// order_code, group_no va id — hammasi bitta orderni topadi.
	for _, code := range []string{"AB123", "TESTPM-AB123", "o1"} {
		o, err := store.Order().GetByCode(ctx, code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if o.ID != "o1" || o.Code != "AB123" || o.CurrentStep != "Ready to ship" || o.LocationLabel != "1" {
			t.Fatalf("%s: noto'g'ri order %+v", code, o)
		}
	}

	// order_code yo'q orderda kod sifatida group_no qaytadi.
	o, err := store.Order().GetByCode(ctx, "GRP-2")
	if err != nil || o.Code != "GRP-2" || o.LocationKind != "" {
		t.Fatalf("GRP-2: %+v, %v", o, err)
	}
}

func TestGetByCodeNotFound(t *testing.T) {
	store := newTestStore(t)
	seedTimeline(t, store)

	_, err := store.Order().GetByCode(context.Background(), "NOPE")
	if !errors.Is(err, storage.ErrOrderNotFound) {
		t.Fatalf("ErrOrderNotFound kutilgan, bor %v", err)
	}
}

// Faqat shu orderning qutilari va handover'lari qaytadi.
func TestGetPackagesAndHandovers(t *testing.T) {
	store := newTestStore(t)
	seedTimeline(t, store)
	ctx := context.Background()

	packages, err := store.Order().GetPackages(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].PackageNo != "PKG-1" || packages[0].TileShelf != "1" || packages[0].SortedAt == nil {
		t.Fatalf("faqat PKG-1 (tile bilan) kutilgan, bor %+v", packages)
	}

	handovers, err := store.Order().GetHandovers(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if len(handovers) != 1 || handovers[0].HandoverNo != "HND-1" || handovers[0].ReceivedAt == nil {
		t.Fatalf("faqat HND-1 kutilgan, bor %+v", handovers)
	}
}
