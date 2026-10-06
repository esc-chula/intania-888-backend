-- +goose Up
-- Team ("color") coin totals live on colors, like users.remaining_coin. The
-- append-only ledger below records each change to them: a match's vote result is
-- written once (SETTLED) and corrected by later ADJUSTED rows when a bill is voided.
-- Every ledger insert updates the colors totals in the same transaction.
ALTER TABLE colors
    ADD COLUMN team_coin bigint NOT NULL DEFAULT 0 CHECK (team_coin >= 0),
    ADD COLUMN bets_right bigint NOT NULL DEFAULT 0 CHECK (bets_right >= 0),
    ADD COLUMN bets_wrong bigint NOT NULL DEFAULT 0 CHECK (bets_wrong >= 0);

CREATE TABLE team_coin_events (
    id varchar(100) PRIMARY KEY,
    match_id varchar(100) NOT NULL REFERENCES matches(id) ON UPDATE CASCADE,
    color_id varchar(100) NOT NULL REFERENCES colors(id) ON UPDATE CASCADE,
    kind varchar(20) NOT NULL CHECK (kind IN ('SETTLED','ADJUSTED')),
    bill_id varchar(100) REFERENCES bill_heads(id) ON UPDATE CASCADE,
    amount_delta bigint NOT NULL,
    bets_right_delta integer NOT NULL,
    bets_wrong_delta integer NOT NULL,
    vote_right integer NOT NULL CHECK (vote_right >= 0),
    vote_wrong integer NOT NULL CHECK (vote_wrong >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'SETTLED' AND bill_id IS NULL) OR (kind = 'ADJUSTED' AND bill_id IS NOT NULL))
);
CREATE UNIQUE INDEX team_coin_events_settled_once ON team_coin_events(match_id, color_id) WHERE kind = 'SETTLED';
CREATE UNIQUE INDEX team_coin_events_adjusted_once ON team_coin_events(bill_id, match_id, color_id) WHERE kind = 'ADJUSTED';
CREATE INDEX team_coin_events_color_id_idx ON team_coin_events(color_id);
CREATE INDEX team_coin_events_match_id_idx ON team_coin_events(match_id);

-- +goose Down
DROP TABLE IF EXISTS team_coin_events;
ALTER TABLE colors DROP COLUMN IF EXISTS team_coin, DROP COLUMN IF EXISTS bets_right, DROP COLUMN IF EXISTS bets_wrong;
