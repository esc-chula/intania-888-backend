-- +goose Up
CREATE TABLE locations (
    id varchar(100) PRIMARY KEY,
    title varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE matches ADD COLUMN location_id varchar(100) NOT NULL;
ALTER TABLE matches
    ADD CONSTRAINT matches_location_id_fkey
    FOREIGN KEY (location_id) REFERENCES locations(id)
    ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX matches_location_id_idx ON matches(location_id);

-- +goose Down
DROP INDEX IF EXISTS matches_location_id_idx;
ALTER TABLE matches DROP CONSTRAINT IF EXISTS matches_location_id_fkey;
ALTER TABLE matches DROP COLUMN IF EXISTS location_id;
DROP TABLE IF EXISTS locations;
