package event

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// DailyReward is a configured override stored in minor units.
type DailyReward struct {
	Date   string
	Reward int64
}

// DailyRewardScheduleItem is one dated amount in the schedule.
type DailyRewardScheduleItem struct {
	Date   string
	Amount value.Money
}

// DailyRewardSchedule contains the default and chronologically ordered overrides.
type DailyRewardSchedule struct {
	DefaultAmount value.Money
	Overrides     []DailyRewardScheduleItem
}

// StealToken is the neutral state of a single-use raid token.
type StealToken struct {
	ID               string
	UserID           string
	Token            string
	IsUsed           bool
	AllowedVictimIDs []string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TokenReward describes the token issued by an alien slot result.
type TokenReward struct {
	Token       string
	ExpiresAt   time.Time
	VictimCount int
	Message     string
}

// CandidatePreview identifies an eligible victim without exposing its balance.
type CandidatePreview struct {
	Index   int
	Name    string
	RoleID  string
	GroupID *string
}

// SpinResult contains the slots and either a coin reward or an optional token.
type SpinResult struct {
	Slots      []string
	Reward     value.Money
	StealToken *TokenReward
	Candidates []CandidatePreview
}

// VictimDetail reports each candidate's pre-raid state and whether it was chosen.
type VictimDetail struct {
	Index         int
	UserID        string
	Name          string
	RoleID        string
	GroupID       *string
	BalanceBefore value.Money
	AmountStolen  value.Money
	WasChosen     bool
}

// StealResult reports the credited raid amount, balance, and candidate details.
type StealResult struct {
	TotalStolen      value.Money
	RaiderNewBalance value.Money
	AllCandidates    []VictimDetail
	Message          string
}
