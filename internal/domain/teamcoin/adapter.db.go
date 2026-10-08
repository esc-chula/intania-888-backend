package teamcoin

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository implements Repository on a database handle, normally a transaction.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository binds the ledger queries to db. Pass the caller's transaction handle.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// TeamVoteTallies counts bets per color on a match that has a winner. Each user
// casts one vote based on which side they staked more on across all their bills;
// ties count as wrong. Only USER accounts that belong to a group have a color.
// Voided bills are excluded, but colors whose bets were all voided still appear
// so a stale ledger row gets corrected.
func (s *GORMRepository) TeamVoteTallies(ctx context.Context, matchID string) ([]Tally, error) {
	var rows []Tally
	// The query builds the tally in separate stages:
	//  1. Load every bill line for this decided match together with its bill
	//     stake, bettor, bettor color, and the match winner. Keeping voided lines
	//     here lets the final result include colors whose bets were all voided.
	//  2. Exclude voided bills and sum each user's stake by selected side. A bill
	//     contributes its total stake to the selection on this match.
	//  3. Compare each user's total stake on the winning side with their total
	//     stake on the other side. Count the user once as right only when the
	//     winning-side total is greater; a larger opposing total or an exact tie
	//     counts once as wrong.
	//  4. Count right and wrong bets separately from the individual non-voided
	//     lines, since these accuracy totals are per bet rather than per user.
	//  5. Return every bettor color found in the match lines, left joining the
	//     vote and bet totals so all-voided colors receive zero tallies and can
	//     have any stale ledger balance corrected by the caller.
	err := s.db.WithContext(ctx).Raw(`
		WITH bet_lines AS (
			SELECT g.color_id, u.id AS user_id, bl.betting_on, m.winner_id,
				bh.status, bh.total AS bet_amount
			FROM bill_lines bl
			JOIN bill_heads bh ON bh.id = bl.bill_id
			JOIN matches m ON m.id = bl.match_id AND m.winner_id IS NOT NULL
			JOIN users u ON u.id = bh.user_id AND u.role_id = 'USER'
			JOIN intania_groups g ON g.id = u.group_id
			WHERE bl.match_id = ?
		), user_side_totals AS (
			SELECT color_id, user_id, betting_on, winner_id,
				SUM(bet_amount) AS bet_amount
			FROM bet_lines
			WHERE status <> 'VOIDED'
			GROUP BY color_id, user_id, betting_on, winner_id
		), user_votes AS (
			SELECT color_id, user_id,
				COALESCE(SUM(bet_amount) FILTER (WHERE betting_on = winner_id), 0) AS winner_amount,
				COALESCE(SUM(bet_amount) FILTER (WHERE betting_on <> winner_id), 0) AS other_amount
			FROM user_side_totals
			GROUP BY color_id, user_id
		), vote_tallies AS (
			SELECT color_id,
				COUNT(*) FILTER (WHERE winner_amount > other_amount) AS vote_right,
				COUNT(*) FILTER (WHERE winner_amount <= other_amount) AS vote_wrong
			FROM user_votes
			GROUP BY color_id
		), bet_tallies AS (
			SELECT color_id,
				COUNT(*) FILTER (WHERE status <> 'VOIDED' AND betting_on = winner_id) AS bets_right,
				COUNT(*) FILTER (WHERE status <> 'VOIDED' AND betting_on <> winner_id) AS bets_wrong
			FROM bet_lines
			GROUP BY color_id
		), colors_with_bets AS (
			SELECT DISTINCT color_id
			FROM bet_lines
		)
		SELECT colors_with_bets.color_id,
			COALESCE(vote_tallies.vote_right, 0) AS vote_right,
			COALESCE(vote_tallies.vote_wrong, 0) AS vote_wrong,
			COALESCE(bet_tallies.bets_right, 0) AS bets_right,
			COALESCE(bet_tallies.bets_wrong, 0) AS bets_wrong
		FROM colors_with_bets
		LEFT JOIN vote_tallies ON vote_tallies.color_id = colors_with_bets.color_id
		LEFT JOIN bet_tallies ON bet_tallies.color_id = colors_with_bets.color_id
	`, matchID).Scan(&rows).Error

	return rows, err
}

// TeamCoinBalances sums the ledger rows per color for a match.
func (s *GORMRepository) TeamCoinBalances(ctx context.Context, matchID string) ([]Balance, error) {
	var rows []Balance
	err := s.db.WithContext(ctx).Raw(`
		SELECT color_id AS color_id,
			COALESCE(SUM(amount_delta), 0)::bigint AS amount,
			COALESCE(SUM(bets_right_delta), 0)::bigint AS bets_right,
			COALESCE(SUM(bets_wrong_delta), 0)::bigint AS bets_wrong
		FROM team_coin_events
		WHERE match_id = ?
		GROUP BY color_id
	`, matchID).Scan(&rows).Error

	return rows, err
}

// CreateTeamCoinEvents inserts the ledger rows and applies their deltas to the
// colors totals. Callers run it inside a transaction so the two never diverge.
func (s *GORMRepository) CreateTeamCoinEvents(ctx context.Context, events []Event) error {
	rows := make([]persistence.TeamCoinEvent, len(events))
	for i, event := range events {
		var billID *string
		if event.BillID != "" {
			id := event.BillID
			billID = &id
		}
		rows[i] = persistence.TeamCoinEvent{
			ID:             event.ID,
			MatchID:        event.MatchID,
			ColorID:        event.ColorID,
			Kind:           event.Kind,
			BillID:         billID,
			AmountDelta:    event.AmountDelta,
			BetsRightDelta: int(event.BetsRightDelta),
			BetsWrongDelta: int(event.BetsWrongDelta),
			VoteRight:      int(event.VoteRight),
			VoteWrong:      int(event.VoteWrong),
			CreatedAt:      event.CreatedAt,
		}
	}

	if err := s.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return err
	}

	for _, event := range events {
		result := s.db.WithContext(ctx).Exec(`
			UPDATE colors
			SET team_coin = team_coin + ?, bets_right = bets_right + ?, bets_wrong = bets_wrong + ?
			WHERE id = ?
		`, event.AmountDelta, event.BetsRightDelta, event.BetsWrongDelta, event.ColorID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update team coin totals: color %q not found", event.ColorID)
		}
	}

	return nil
}
