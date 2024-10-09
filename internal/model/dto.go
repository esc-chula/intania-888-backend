package model

import "time"

type UserDto struct {
	Id            string  `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	NickName      *string `json:"nick_name"`
	RoleId        string  `json:"role_id"`
	GroupId       *string `json:"group_id"`
	RemainingCoin float64 `json:"remaining_coin"`
}

type RoleDto struct {
	Id string `json:"id"`
}

type CredentialDto struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int32  `json:"expires_in"`
}

type OAuthCodeDto struct {
	Code string `json:"code"`
}

type RefreshTokenDto struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshCacheDto struct {
	UserId string
	Role   string
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
	TeamARate  float64   `json:"team_a_rate"`
	TeamBRate  float64   `json:"team_b_rate"`
	WinnerId   string    `json:"winner"`
	TypeId     string    `json:"type"`
	IsDraw     bool      `json:"is_draw"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
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
	TeamAScore int
	TeamBScore int
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
	Id     string         `json:"id"`
	Total  float64        `json:"total"`
	UserId string         `json:"user_id"`
	Lines  []*BillLineDto `json:"lines"` // Nested BillLine DTO
}

type BillLineDto struct {
	BillId    string   `json:"bill_id"`
	MatchId   string   `json:"match_id"`
	Rate      float64  `json:"rate"`
	BettingOn string   `json:"betting_on"`
	Match     MatchDto `json:"match"`
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
	Reward float64
}
