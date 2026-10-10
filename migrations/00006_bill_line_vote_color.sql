-- +goose Up
-- Capture the bettor's group color when each selection is accepted. There is
-- intentionally no backfill: existing rows have no reliable historical color.
ALTER TABLE bill_lines
    ADD COLUMN vote_color_id varchar(100) REFERENCES colors(id) ON UPDATE CASCADE ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE bill_lines DROP COLUMN IF EXISTS vote_color_id;
