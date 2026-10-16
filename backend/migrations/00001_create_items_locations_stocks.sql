-- +goose Up
CREATE TABLE items (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sku varchar(64) NOT NULL,
    name varchar(200) NOT NULL,
    category varchar(64) NOT NULL,
    unit varchar(32) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT items_id_positive CHECK (id > 0),
    CONSTRAINT items_sku_key UNIQUE (sku),
    CONSTRAINT items_sku_valid CHECK (length(sku) > 0 AND sku = btrim(sku) AND sku !~ E'[\x01-\x1F\x7F]'),
    CONSTRAINT items_name_valid CHECK (length(name) > 0 AND name = btrim(name) AND name !~ E'[\x01-\x1F\x7F]'),
    CONSTRAINT items_category_valid CHECK (length(category) > 0 AND category = btrim(category) AND category !~ E'[\x01-\x1F\x7F]'),
    CONSTRAINT items_unit_valid CHECK (length(unit) > 0 AND unit = btrim(unit) AND unit !~ E'[\x01-\x1F\x7F]')
);

CREATE TABLE locations (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code varchar(64) NOT NULL,
    zone varchar(64) NOT NULL,
    type varchar(32) NOT NULL,
    CONSTRAINT locations_id_positive CHECK (id > 0),
    CONSTRAINT locations_code_key UNIQUE (code),
    CONSTRAINT locations_code_valid CHECK (length(code) > 0 AND code = btrim(code) AND code !~ E'[\x01-\x1F\x7F]'),
    CONSTRAINT locations_zone_valid CHECK (length(zone) > 0 AND zone = btrim(zone) AND zone !~ E'[\x01-\x1F\x7F]'),
    CONSTRAINT locations_type_valid CHECK (length(type) > 0 AND type = btrim(type) AND type !~ E'[\x01-\x1F\x7F]')
);

CREATE TABLE stocks (
    id integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_id integer NOT NULL,
    location_id integer NOT NULL,
    qty integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT stocks_id_positive CHECK (id > 0),
    CONSTRAINT stocks_item_id_fkey FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT stocks_location_id_fkey FOREIGN KEY (location_id) REFERENCES locations(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT stocks_qty_nonnegative CHECK (qty >= 0),
    CONSTRAINT stocks_item_location_key UNIQUE (item_id, location_id)
);

CREATE INDEX items_active_category_id_idx ON items (category, id) WHERE deleted_at IS NULL;

-- +goose Down
-- Rollback is opt-in, restricted to an empty disposable database, and transactional.
-- +goose StatementBegin
DO $$
BEGIN
    IF current_setting('warehouse.allow_destructive_rollback', true) IS DISTINCT FROM 'on' THEN
        RAISE EXCEPTION 'Rollback requires explicit disposable-database opt-in';
    END IF;
    LOCK TABLE items, locations, stocks IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM items) OR EXISTS (SELECT 1 FROM stocks) OR EXISTS (SELECT 1 FROM locations) THEN
        RAISE EXCEPTION 'Refusing schema rollback: database contains business data';
    END IF;
END;
$$;
-- +goose StatementEnd
DROP TABLE stocks;
DROP TABLE locations;
DROP TABLE items;
