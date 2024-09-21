package model

import "time"

type UserDto struct {
	Id        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	RoleId    string    `json:"role_id"`
	GroupId   string    `json:"group_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoleDto struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	Id        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IntaniaGroupDto struct {
	Id        string    `json:"id"`
	ColorId   string    `json:"color_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MatchDto struct {
	Id        string    `json:"id"`
	TeamAId   string    `json:"team_a_id"`
	TeamBId   string    `json:"team_b_id"`
	WinnerId  string    `json:"winner_id"`
	TypeId    string    `json:"type_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TeamA     ColorDto  `json:"team_a"`
	TeamB     ColorDto  `json:"team_b"`
	Winner    ColorDto  `json:"winner"`
	SportType SportType `json:"sport_type"`
}

type BillHeadDto struct {
	Id        string         `json:"id"`
	Total     float64        `json:"total"`
	UserId    string         `json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Lines     []*BillLineDto `json:"lines"` // Nested BillLine DTO
}

type BillLineDto struct {
	BillId    string    `json:"bill_id"`
	MatchId   string    `json:"match_id"`
	Rate      float64   `json:"rate"`
	BettingOn string    `json:"betting_on"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Match     MatchDto  `json:"match"`
	Color     ColorDto  `json:"color"` // For `BettingOn`
}

type GroupHeadDto struct {
	Id        string          `json:"id"`
	Title     string          `json:"title"`
	TypeId    string          `json:"type_id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Lines     []*GroupLineDto `json:"lines"`      // Nested GroupLine DTO
	SportType SportTypeDto    `json:"sport_type"` // Nested SportType DTO
}

type GroupLineDto struct {
	GroupId   string    `json:"group_id"`
	TeamId    string    `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Team      ColorDto  `json:"team"`
}

type SportTypeDto struct {
	Id        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
