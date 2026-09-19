-- +goose Up
CREATE TABLE roles (
    id varchar(100) PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO roles (id) VALUES ('USER'), ('ADMIN') ON CONFLICT DO NOTHING;

CREATE TABLE colors (
    id varchar(100) PRIMARY KEY,
    title varchar(100) NOT NULL,
    total_matches integer NOT NULL DEFAULT 0,
    won integer NOT NULL DEFAULT 0,
    drawn integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE intania_groups (
    id varchar(100) PRIMARY KEY,
    color_id varchar(100) NOT NULL REFERENCES colors(id) ON UPDATE CASCADE ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE users (
    id varchar(100) PRIMARY KEY,
    email varchar(100) NOT NULL,
    name varchar(100) NOT NULL,
    nick_name varchar(100),
    role_id varchar(100) NOT NULL REFERENCES roles(id) ON UPDATE CASCADE,
    group_id varchar(100) REFERENCES intania_groups(id) ON UPDATE CASCADE,
    remaining_coin bigint NOT NULL DEFAULT 0 CHECK (remaining_coin >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sport_types (
    id varchar(100) PRIMARY KEY,
    title varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE matches (
    id varchar(100) PRIMARY KEY,
    teama_id varchar(100) REFERENCES colors(id) ON UPDATE CASCADE,
    teamb_id varchar(100) REFERENCES colors(id) ON UPDATE CASCADE,
    teama_score integer,
    teamb_score integer,
    winner_id varchar(100) REFERENCES colors(id) ON UPDATE CASCADE,
    type_id varchar(100) NOT NULL REFERENCES sport_types(id) ON UPDATE CASCADE ON DELETE CASCADE,
    is_draw boolean NOT NULL DEFAULT false,
    start_time timestamptz NOT NULL,
    end_time timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (teama_id IS NULL OR teamb_id IS NULL OR teama_id <> teamb_id),
    CHECK (NOT is_draw OR winner_id IS NULL),
    CHECK (winner_id IS NULL OR winner_id = teama_id OR winner_id = teamb_id)
);
CREATE TABLE bill_heads (
    id varchar(100) PRIMARY KEY,
    total bigint NOT NULL CHECK (total > 0),
    user_id varchar(100) NOT NULL REFERENCES users(id) ON UPDATE CASCADE,
    status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','WON','LOST','VOIDED')),
    payout bigint CHECK (payout >= 0),
    settled_at timestamptz,
    voided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'PENDING' AND payout IS NULL AND settled_at IS NULL AND voided_at IS NULL)
        OR (status IN ('WON','LOST') AND payout IS NOT NULL AND settled_at IS NOT NULL AND voided_at IS NULL)
        OR (status = 'VOIDED' AND payout = total AND settled_at IS NULL AND voided_at IS NOT NULL))
);
CREATE TABLE bill_lines (
    bill_id varchar(100) NOT NULL REFERENCES bill_heads(id) ON UPDATE CASCADE ON DELETE CASCADE,
    match_id varchar(100) NOT NULL REFERENCES matches(id) ON UPDATE CASCADE,
    rate bigint NOT NULL CHECK (rate >= 0),
    betting_on varchar(100) NOT NULL REFERENCES colors(id) ON UPDATE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bill_id, match_id)
);
CREATE INDEX bill_lines_match_id_idx ON bill_lines(match_id);
CREATE TABLE bill_terminal_events (
    id varchar(100) PRIMARY KEY,
    bill_id varchar(100) NOT NULL UNIQUE REFERENCES bill_heads(id) ON UPDATE CASCADE ON DELETE CASCADE,
    kind varchar(20) NOT NULL CHECK (kind IN ('SETTLED','VOIDED')),
    amount bigint NOT NULL CHECK (amount >= 0),
    actor_id varchar(100) REFERENCES users(id) ON UPDATE CASCADE,
    reason varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'VOIDED' AND actor_id IS NOT NULL AND length(btrim(reason)) BETWEEN 1 AND 500)
        OR (kind = 'SETTLED' AND actor_id IS NULL AND reason IS NULL))
);
CREATE TABLE group_heads (
    id varchar(100) PRIMARY KEY, title varchar(100) NOT NULL,
    type_id varchar(100) NOT NULL REFERENCES sport_types(id) ON UPDATE CASCADE ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE group_lines (
    group_id varchar(100) NOT NULL REFERENCES group_heads(id) ON UPDATE CASCADE ON DELETE CASCADE,
    team_id varchar(100) NOT NULL REFERENCES colors(id) ON UPDATE CASCADE ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(group_id, team_id)
);
CREATE TABLE group_stages (
    id varchar(100) NOT NULL, type_id varchar(100) NOT NULL REFERENCES sport_types(id),
    color_id varchar(100) NOT NULL REFERENCES colors(id),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(id, type_id, color_id)
);
CREATE TABLE daily_rewards (
    date varchar(100) PRIMARY KEY, reward bigint NOT NULL CHECK (reward >= 0),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE steal_tokens (
    id varchar(100) PRIMARY KEY, user_id varchar(100) NOT NULL REFERENCES users(id),
    token varchar(100) NOT NULL UNIQUE, is_used boolean NOT NULL DEFAULT false,
    allowed_victim_ids text NOT NULL, expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX steal_tokens_user_id_idx ON steal_tokens(user_id);
CREATE INDEX steal_tokens_expires_at_idx ON steal_tokens(expires_at);
CREATE TABLE mine_games (
    id varchar(100) PRIMARY KEY, user_id varchar(100) NOT NULL REFERENCES users(id),
    bet_amount bigint NOT NULL CHECK (bet_amount >= 0),
    risk_level varchar(20) NOT NULL CHECK (risk_level IN ('low','medium','high')),
    status varchar(20) NOT NULL CHECK (status IN ('active','won','lost','cashed_out')),
    revealed_count integer NOT NULL DEFAULT 0 CHECK (revealed_count BETWEEN 0 AND 16),
    current_payout bigint NOT NULL CHECK (current_payout >= 0),
    multiplier bigint NOT NULL DEFAULT 1000000 CHECK (multiplier >= 0),
    grid_data text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz
);
CREATE UNIQUE INDEX mine_games_one_active_per_user ON mine_games(user_id) WHERE status = 'active';
CREATE TABLE mine_game_histories (
    id varchar(100) PRIMARY KEY, game_id varchar(100) NOT NULL REFERENCES mine_games(id) ON DELETE CASCADE,
    tile_index integer NOT NULL CHECK (tile_index BETWEEN 0 AND 15), tile_type varchar(20) NOT NULL,
    multiplier bigint NOT NULL CHECK (multiplier >= 0), payout_at_hit bigint NOT NULL CHECK (payout_at_hit >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS mine_game_histories, mine_games, steal_tokens, daily_rewards, group_stages,
    group_lines, group_heads, bill_terminal_events, bill_lines, bill_heads, matches, sport_types,
    users, intania_groups, colors, roles CASCADE;
