-- Har jadvalda: alohida ustunlar (qoidalar uchun), to'liq xom JSON (raw),
-- raw_hash (o'zgarishni aniqlash), first_seen_at / last_synced_at.
-- ID'lar text: stend UUID beradi, lekin format kafolatlanmagan.

CREATE TABLE IF NOT EXISTS orders (
    id                text PRIMARY KEY,
    order_code        text,
    group_no          text,
    warehouse_status  text, -- READY_FOR_ASSEMBLY, READY_TO_SHIP, RETURNED_TO_SELLERS, RETURNED_TO_WAREHOUSE, WAITING_FOR_SELLER_ITEMS
    lifecycle         text, -- ACCEPTED, AWAITING, OUT, RETURN, SORTING
    lifecycle_outcome text, -- AT_HUB_UNSCANNED, ISSUED_TO_BUYER, RETURN_TO_HUB, RETURNED_TO_SELLERS, RETURNED_TO_WAREHOUSE, NULL
    lifecycle_clock   text, -- awaiting_courier, awaiting_seller, return_inspect, to_assemble, NULL
    lifecycle_since   timestamptz,
    current_step_code text, -- in_transit_to_warehouse, ready_for_packing, ready_to_ship, returned_to_sellers, returned_to_warehouse
    delivery_status   text, -- AT_HUB, AT_ORIGIN, BACK_AT_ORIGIN, TO_HUB, NULL
    status_summary    text, -- MIXED, WAITING_FOR_SELLER
    payment_status    text, -- paid, unpaid, NULL
    item_count        int,
    seller_count      int,
    total_uzs         bigint,
    open_exceptions   int,
    is_dev            boolean,
    is_backorder      boolean,
    location_kind     text, -- COURIER, SHELF, STAGING, NULL
    location_label    text, -- 0, 1, DNX courier, RET-01, RET-T, NULL
    location_since    timestamptz,
    created_at        timestamptz,
    last_activity_at  timestamptz,
    raw               jsonb       NOT NULL,
    raw_hash          text        NOT NULL,
    first_seen_at     timestamptz NOT NULL DEFAULT now(),
    last_synced_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS orders_order_code_idx ON orders (order_code);
CREATE INDEX IF NOT EXISTS orders_group_no_idx ON orders (group_no);

CREATE TABLE IF NOT EXISTS packages (
    id             text PRIMARY KEY,
    package_no     text,
    order_group_id text,
    order_code     text,
    group_no       text,
    status         text, -- DISPATCHED, ISSUED_TO_BUYER, REPACKED, RETURNED_TO_SELLERS, RETURNED_TO_WAREHOUSE, SORTED
    shelf_code     text, -- 0, 1, RET-01, RET-T, NULL
    packed_at      timestamptz,
    held_since     timestamptz,
    held_by        text, -- Полка 0, Полка 1, Полка RET-T, DNX courier, Returned to original sellers, Seed Bot, System, NULL
    item_count     int,
    label_printed  boolean,
    return_id      text,
    raw            jsonb       NOT NULL,
    raw_hash       text        NOT NULL,
    first_seen_at  timestamptz NOT NULL DEFAULT now(),
    last_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS packages_order_group_id_idx ON packages (order_group_id);

CREATE TABLE IF NOT EXISTS handovers (
    id                 text PRIMARY KEY,
    handover_no        text,
    order_group_id     text,
    order_code         text,
    group_no           text,
    seller_id          text,
    shop_name          text,
    status             text, -- IN_TRANSIT, RECEIVED
    item_count         int,
    expected_qty       int,
    location_confirmed boolean,
    created_at         timestamptz,
    transit_since      timestamptz,
    received_at        timestamptz,
    raw                jsonb       NOT NULL,
    raw_hash           text        NOT NULL,
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    last_synced_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS handovers_order_group_id_idx ON handovers (order_group_id);

-- /shelves snapshot'i: javonlar va ulardagi qutilar (tiles).
CREATE TABLE IF NOT EXISTS shelves (
    location_id     text PRIMARY KEY,
    code            text, -- 0, 1, 2, 3, 4, CNL-01, QRT-01, RET-01, RTY-01, X
    kind            text, -- CANCEL_HOLD, QUARANTINE, RETRY, RETURNS, SORT_SHELF
    name            text,
    active          boolean,
    status          text, -- FILLING, READY
    box_count       int,
    shippable_count int,
    unpacked_count  int,
    cancelled_count int,
    raw             jsonb       NOT NULL,
    raw_hash        text        NOT NULL,
    first_seen_at   timestamptz NOT NULL DEFAULT now(),
    last_synced_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS shelf_tiles (
    package_id     text PRIMARY KEY,
    shelf_code     text, -- 0, 1
    package_no     text,
    order_group_id text,
    area_code      text, -- 012000, 101000
    cancelled      boolean,
    packed_at      timestamptz,
    sorted_at      timestamptz,
    raw            jsonb       NOT NULL,
    raw_hash       text        NOT NULL,
    first_seen_at  timestamptz NOT NULL DEFAULT now(),
    last_synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shelf_tiles_order_group_id_idx ON shelf_tiles (order_group_id);

CREATE TABLE IF NOT EXISTS sync_runs (
    id          bigserial PRIMARY KEY,
    status      text        NOT NULL, -- running, success, failed
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    inserted    int         NOT NULL DEFAULT 0,
    updated     int         NOT NULL DEFAULT 0,
    unchanged   int         NOT NULL DEFAULT 0,
    last_entity text        NOT NULL DEFAULT '',
    last_offset int         NOT NULL DEFAULT 0,
    entities    jsonb       NOT NULL DEFAULT '[]',
    error       text        NOT NULL DEFAULT ''
);