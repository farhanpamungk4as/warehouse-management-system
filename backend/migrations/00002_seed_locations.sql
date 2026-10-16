-- +goose Up
INSERT INTO locations (code, zone, type)
VALUES ('RACK-A1', 'A', 'rack'), ('RACK-B1', 'B', 'rack')
ON CONFLICT (code) DO NOTHING;

-- +goose Down
-- Code alone does not establish ownership; opt-in is only for a known disposable DB.
-- +goose StatementBegin
DO $$
BEGIN
    IF current_setting('warehouse.allow_destructive_rollback', true) IS DISTINCT FROM 'on' THEN
        RAISE EXCEPTION 'Seed rollback requires explicit disposable-database opt-in';
    END IF;
    LOCK TABLE locations, stocks IN SHARE ROW EXCLUSIVE MODE;
    IF EXISTS (
        SELECT 1 FROM locations
        WHERE (code = 'RACK-A1' AND (zone <> 'A' OR type <> 'rack'))
           OR (code = 'RACK-B1' AND (zone <> 'B' OR type <> 'rack'))
    ) OR EXISTS (
        SELECT 1 FROM stocks s JOIN locations l ON l.id = s.location_id
        WHERE l.code IN ('RACK-A1', 'RACK-B1')
    ) THEN
        RAISE EXCEPTION 'Refusing seed rollback: locations changed or referenced by stock';
    END IF;
    DELETE FROM locations WHERE code IN ('RACK-A1', 'RACK-B1');
END;
$$;
-- +goose StatementEnd
