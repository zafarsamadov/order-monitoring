-- Har jadvalda: alohida ustunlar (qoidalar uchun), to'liq xom JSON (raw),
-- raw_hash (o'zgarishni aniqlash), first_seen_at / last_synced_at.
-- ID'lar text: stend UUID beradi, lekin format kafolatlanmagan.

CREATE TABLE IF NOT EXISTS orders (
    id                text PRIMARY KEY,
    order_code        text,
    group_no          text,
    warehouse_status  text,
    lifecycle         text,
    lifecycle_outcome text,
    lifecycle_clock   text,
    lifecycle_since   timestamptz,
    current_step_code text,
    delivery_status   text,
    status_summary    text,
    payment_status    text,
    item_count        int,
    seller_count      int,
    total_uzs         bigint,
    open_exceptions   int,
    is_dev            boolean,
    is_backorder      boolean,
    location_kind     text,
    location_label    text,
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
    status         text,
    shelf_code     text,
    packed_at      timestamptz,
    held_since     timestamptz,
    held_by        text,
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
    status             text,
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
    code            text,
    kind            text,
    name            text,
    active          boolean,
    status          text,
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
    shelf_code     text,
    package_no     text,
    order_group_id text,
    area_code      text,
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
    status      text        NOT NULL,
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
