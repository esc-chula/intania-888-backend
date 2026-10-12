-- +goose Up
CREATE TABLE daily_reward_claims (
    user_id varchar(100) NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    reward_date varchar(100) NOT NULL,
    reward bigint NOT NULL CHECK (reward >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, reward_date)
);

-- +goose Down
DROP TABLE IF EXISTS daily_reward_claims;
