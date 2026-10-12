package teamcoin

import "context"

// Repository reads bet tallies and ledger balances and appends ledger events. It must
// be bound to the caller's transaction so the ledger commits with the change that caused it.
type Repository interface {
	// TeamVoteTallies counts each color's bets on a match that has a winner; drawn and undecided matches have none.
	TeamVoteTallies(ctx context.Context, matchID string) ([]Tally, error)
	// TeamCoinBalances sums the ledger per color for a match.
	TeamCoinBalances(ctx context.Context, matchID string) ([]Balance, error)
	// CreateTeamCoinEvents appends ledger rows and applies their deltas to the colors totals.
	CreateTeamCoinEvents(ctx context.Context, events []Event) error
}
