package model

import "time"

type UserDto struct {
	Id            string    `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	NickName      *string   `json:"nick_name"`
	RoleId        string    `json:"role_id"`
	GroupId       *string   `json:"group_id"`
	RemainingCoin Money     `json:"remaining_coin" swaggertype:"string" example:"888.88"`
	CreatedAt     time.Time `json:"created_at"`
}

// SessionRecord contains no browser credential or role snapshot.
type SessionRecord struct {
	UserId    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	CSRFToken string `json:"csrf_token"`
}

// OAuthStateRecord contains the server-side PKCE verifier for one OAuth flow.
// The verifier must never be sent to the browser or included in an OAuth URL.
type OAuthStateRecord struct {
	CodeVerifier string `json:"code_verifier"`
}

type RoleDto struct {
	Id string `json:"id"`
}

type ColorDto struct {
	Id         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Won        int64  `json:"won"`
	Drawn      int64  `json:"drawn"`
	Lost       int64  `json:"lost"`
	TotalMatch int64  `json:"total_matches"`
}

type IntaniaGroupDto struct {
	Id      string `json:"id"`
	ColorId string `json:"color_id"`
}

type MatchDto struct {
	Id         string    `json:"id"`
	TeamAId    string    `json:"team_a"`
	TeamBId    string    `json:"team_b"`
	TeamAScore *int      `json:"team_a_score"`
	TeamBScore *int      `json:"team_b_score"`
	TeamARate  Rate      `json:"team_a_rate" swaggertype:"number"`
	TeamBRate  Rate      `json:"team_b_rate" swaggertype:"number"`
	WinnerId   string    `json:"winner"`
	TypeId     string    `json:"type"`
	IsDraw     bool      `json:"is_draw"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
}

type CreateMatchRequest struct {
	TeamAId   string    `json:"team_a" validate:"required"`
	TeamBId   string    `json:"team_b" validate:"required"`
	TypeId    string    `json:"type" validate:"required"`
	StartTime time.Time `json:"start_time" validate:"required"`
	EndTime   time.Time `json:"end_time" validate:"required"`
}

type UpdateMatchRequest struct {
	TeamAId   *string    `json:"team_a"`
	TeamBId   *string    `json:"team_b"`
	TypeId    *string    `json:"type"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
}

type MatchesByType struct {
	SportType string      `json:"sportType"`
	Matches   []*MatchDto `json:"matches"`
}

type MatchesByDate struct {
	Date  time.Time       `json:"date"`
	Types []MatchesByType `json:"types"`
}

type ScoreDto struct {
	TeamAScore    int `json:"team_a_score" validate:"required,min=0"`
	TeamBScore    int `json:"team_b_score" validate:"required,min=0"`
	teamAScoreSet bool
	teamBScoreSet bool
}

type ScheduleFilter string

const (
	Schedule ScheduleFilter = "schedule"
	Result   ScheduleFilter = "result"
)

type MatchFilter struct {
	TypeId   string
	Schedule ScheduleFilter
}

type BillHeadDto struct {
	Id        string         `json:"id"`
	Total     Money          `json:"total" swaggertype:"string" example:"100.00"`
	UserId    string         `json:"user_id"`
	Status    string         `json:"status"`
	Payout    *Money         `json:"payout" swaggertype:"string"`
	SettledAt *time.Time     `json:"settled_at"`
	VoidedAt  *time.Time     `json:"voided_at"`
	Lines     []*BillLineDto `json:"lines"`
}

type BillLineDto struct {
	BillId    string   `json:"bill_id"`
	MatchId   string   `json:"match_id"`
	Rate      Rate     `json:"rate" swaggertype:"number"`
	BettingOn string   `json:"betting_on"`
	Match     MatchDto `json:"match"`
}

type CreateBillRequest struct {
	Total Money                   `json:"total" swaggertype:"string" example:"100.00" validate:"required"`
	Lines []CreateBillLineRequest `json:"lines" validate:"required,min=1,dive"`
}

type CreateBillLineRequest struct {
	MatchId   string `json:"match_id" validate:"required"`
	BettingOn string `json:"betting_on" validate:"required"`
}

type VoidBillRequest struct {
	Reason string `json:"reason" validate:"required"`
}

type MatchResultRequest struct {
	Outcome  string  `json:"outcome" validate:"required,oneof=winner draw"`
	WinnerId *string `json:"winner_id,omitempty"`
}

type GroupHeadDto struct {
	Id        string          `json:"id"`
	Title     string          `json:"title"`
	TypeId    string          `json:"type_id"`
	Lines     []*GroupLineDto `json:"lines"`      // Nested GroupLine DTO
	SportType SportTypeDto    `json:"sport_type"` // Nested SportType DTO
}

type GroupLineDto struct {
	GroupId string   `json:"group_id"`
	TeamId  string   `json:"team_id"`
	Team    ColorDto `json:"team"`
}

type SportTypeDto struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type DailyRewardCacheDto struct {
	UserId string
	Reward Money
}

type SetDailyRewardRequest struct {
	Amount    Money `json:"amount" swaggertype:"string" validate:"required"`
	amountSet bool
}

type DailyRewardScheduleItem struct {
	Date   string `json:"date"`
	Amount Money  `json:"amount" swaggertype:"string"`
}

type DailyRewardScheduleResponse struct {
	DefaultAmount Money                     `json:"default_amount" swaggertype:"string"`
	Overrides     []DailyRewardScheduleItem `json:"overrides"`
}

type UpdateUserDto struct {
	Id       string  `json:"id"`
	Email    string  `json:"email"`
	Name     string  `json:"name" validate:"required"`
	NickName *string `json:"nick_name"`
	RoleId   string  `json:"role_id"`
	GroupId  *string `json:"group_id"`
}

// AdminUpdateUserDto intentionally excludes RoleId. Admin promotion and
// demotion are controlled by the operator database workflow.
type AdminUpdateUserDto struct {
	Name          string  `json:"name" validate:"required"`
	NickName      *string `json:"nick_name"`
	GroupId       *string `json:"group_id"`
	RemainingCoin Money   `json:"remaining_coin" swaggertype:"string" example:"888.88"`
}

// Steal token DTOs
type StealTokenDto struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	VictimCount int       `json:"victim_count"`
	Message     string    `json:"message"`
}

type CandidatePreviewDto struct {
	Index   int     `json:"index"`
	Name    string  `json:"name"`
	RoleId  string  `json:"role_id"`
	GroupId *string `json:"group_id"`
}

type UseStealTokenRequestDto struct {
	Token          string `json:"token" validate:"required"`
	VictimIndex    int    `json:"victim_index" validate:"required,min=0"`
	victimIndexSet bool
}

type VictimDetailDto struct {
	Index         int     `json:"index"`
	UserId        string  `json:"user_id"`
	Name          string  `json:"name"`
	RoleId        string  `json:"role_id"`
	GroupId       *string `json:"group_id"`
	BalanceBefore Money   `json:"balance_before" swaggertype:"string"`
	AmountStolen  Money   `json:"amount_stolen" swaggertype:"string"`
	WasChosen     bool    `json:"was_chosen"`
}

type UseStealTokenResponseDto struct {
	TotalStolen      Money             `json:"total_stolen" swaggertype:"string"`
	RaiderNewBalance Money             `json:"raider_new_balance" swaggertype:"string"`
	AllCandidates    []VictimDetailDto `json:"all_candidates"`
	Message          string            `json:"message"`
}

type CreateMineGameRequest struct {
	BetAmount Money  `json:"bet_amount" swaggertype:"string" validate:"required"`
	RiskLevel string `json:"risk_level" validate:"required,oneof=low medium high"`
}

type RevealMineTileRequest struct {
	Index    int `json:"index" validate:"required,min=0,max=15"`
	indexSet bool
}

// Response DTOs
type MineTileDto struct {
	Index    int    `json:"index"`
	Type     string `json:"type"` // diamond, bomb, hidden
	Revealed bool   `json:"revealed"`
}

type MineGameDto struct {
	Id            string        `json:"id"`
	UserId        string        `json:"user_id"`
	BetAmount     Money         `json:"bet_amount" swaggertype:"string"`
	RiskLevel     string        `json:"risk_level"`
	Grid          []MineTileDto `json:"grid"`
	RevealedCount int           `json:"revealed_count"`
	CurrentPayout Money         `json:"current_payout" swaggertype:"string"`
	Multiplier    Rate          `json:"multiplier" swaggertype:"number"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
}

type MineGameStatsDto struct {
	TotalGames          int         `json:"total_games"`
	GamesWon            int         `json:"games_won"`
	GamesLost           int         `json:"games_lost"`
	GamesCashedOut      int         `json:"games_cashed_out"`
	TotalWagered        Money       `json:"total_wagered" swaggertype:"string"`
	TotalWinnings       Money       `json:"total_winnings" swaggertype:"string"`
	NetProfit           SignedMoney `json:"net_profit" swaggertype:"string"`
	ActiveWagered       *Money      `json:"active_wagered" swaggertype:"string"`
	ActiveCurrentPayout *Money      `json:"active_current_payout" swaggertype:"string"`
	WinRate             float64     `json:"win_rate"`
}

type MineGameHistoryDto struct {
	GameId        string     `json:"game_id"`
	BetAmount     Money      `json:"bet_amount" swaggertype:"string"`
	RiskLevel     string     `json:"risk_level"`
	Status        string     `json:"status"`
	FinalPayout   Money      `json:"final_payout" swaggertype:"string"`
	Multiplier    Rate       `json:"multiplier" swaggertype:"number"`
	RevealedCount int        `json:"revealed_count"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// External API DTOs
type DeductCoinRequest struct {
	Amount Money `json:"amount" swaggertype:"string" validate:"required"`
}

type DeductCoinResponse struct {
	Success          bool  `json:"success"`
	DeductedAmount   Money `json:"deducted_amount" swaggertype:"string"`
	RemainingBalance Money `json:"remaining_balance" swaggertype:"string"`
}
