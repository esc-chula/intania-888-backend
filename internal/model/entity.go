package model

import "time"

type User struct {
	Id            string    `gorm:"primaryKey;type:varchar(100)"`
	Email         string    `gorm:"type:varchar(100);not null"`
	Name          string    `gorm:"type:varchar(100);not null"`
	NickName      *string   `gorm:"type:varchar(100);"`
	RoleId        string    `gorm:"type:varchar(100);not null"`
	GroupId       *string   `gorm:"type:varchar(100);"`
	RemainingCoin float64   `gorm:"type:decimal(10,2);"`
	CreatedAt     time.Time ``
	UpdatedAt     time.Time ``

	Role  Role         `gorm:"foreignKey:RoleId"`
	Group IntaniaGroup `gorm:"foreignKey:GroupId"`
	Bills []BillHead   `gorm:"foreignKey:UserId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Role struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Users []User `gorm:"foreignKey:RoleId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Color struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Members   []IntaniaGroup `gorm:"foreignKey:ColorId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	BillLines []BillLine     `gorm:"foreignKey:BettingOn;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamA     []Match        `gorm:"foreignKey:TeamA_Id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamB     []Match        `gorm:"foreignKey:TeamB_Id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Won        []Match        `gorm:"foreignKey:WinnerId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	GroupLines []GroupLine `gorm:"foreignKey:TeamId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	TotalMatches int `gorm:"type:int;"`
	Won          int `gorm:"type:int;"`
	Drawn        int `gorm:"type:int;"`
}

type IntaniaGroup struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	ColorId   string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Color   Color  `gorm:"foreignKey:ColorId"`
	Members []User `gorm:"foreignKey:GroupId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Match struct {
	Id          string  `gorm:"primaryKey;type:varchar(100)"`
	TeamA_Id    *string `gorm:"column:teama_id;type:varchar(100);"`
	TeamB_Id    *string `gorm:"column:teamb_id;type:varchar(100);"`
	TeamA_Score *int    `gorm:"column:teama_score;type:int;"`
	TeamB_Score *int    `gorm:"column:teamb_score;type:int;"`

	WinnerId  *string   `gorm:"column:winner_id;type:varchar(100);"`
	TypeId    string    `gorm:"column:type_id;type:varchar(100);not null"`
	IsDraw    bool      `gorm:"column:is_draw;type:boolean;default:false"`
	StartTime time.Time `gorm:"column:start_time"`
	EndTime   time.Time `gorm:"column:end_time"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`

	BillLines []BillLine `gorm:"foreignKey:MatchId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SportType SportType  `gorm:"foreignKey:TypeId"`
	TeamA     Color      `gorm:"foreignKey:TeamA_Id"`
	TeamB     Color      `gorm:"foreignKey:TeamB_Id"`
	Winner    Color      `gorm:"foreignKey:WinnerId"`
}

type BillHead struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Total     float64   `gorm:"type:decimal(10,2);not null"`
	UserId    string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	User  User       `gorm:"foreignKey:UserId"`
	Lines []BillLine `gorm:"foreignKey:BillId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type BillLine struct {
	BillId    string    `gorm:"primaryKey;type:varchar(100)"`
	MatchId   string    `gorm:"primaryKey;type:varchar(100)"`
	Rate      float64   `gorm:"type:decimal(10,2);not null"`
	IsPaid    bool      `gorm:"type:boolean;default:false"`
	BettingOn string    `gorm:"type:varchar(100);not null"` // color
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Match Match    `gorm:"foreignKey:MatchId"`
	Head  BillHead `gorm:"foreignKey:BillId"`
	Color Color    `gorm:"foreignKey:BettingOn"`
}

type GroupHead struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	TypeId    string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Lines     []GroupLine `gorm:"foreignKey:GroupId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SportType SportType   `gorm:"foreignKey:TypeId"`
}

type GroupLine struct {
	GroupId   string    `gorm:"primaryKey;type:varchar(100)"`
	TeamId    string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Head GroupHead `gorm:"foreignKey:GroupId"`
	Team Color     `gorm:"foreignKey:TeamId"`
}

type GroupStage struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	TypeId    string    `gorm:"primaryKey;type:varchar(100)"`
	ColorId   string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	SportType SportType `gorm:"foreignKey:TypeId"`
	Color     Color     `gorm:"foreignKey:ColorId"`
}

type SportType struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Matches         []Match     `gorm:"foreignKey:TypeId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TournamentGroup []GroupHead `gorm:"foreignKey:TypeId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type DailyReward struct {
	Date      string    `gorm:"primaryKey;type:varchar(100)"` // DD-MM-YY eg. 31-10-24
	Reward    float64   `gorm:"type:decimal(10,2);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``
}

type StealToken struct {
	Id               string    `gorm:"primaryKey;type:varchar(100)"`
	UserId           string    `gorm:"type:varchar(100);not null;index"`
	Token            string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	IsUsed           bool      `gorm:"type:boolean;default:false"`
	AllowedVictimIds string    `gorm:"type:text;not null"`
	ExpiresAt        time.Time `gorm:"not null;index"`
	CreatedAt        time.Time ``
	UpdatedAt        time.Time ``

	User User `gorm:"foreignKey:UserId"`
}
type MineGame struct {
	Id            string     `gorm:"primaryKey;type:varchar(100)"`
	UserId        string     `gorm:"type:varchar(100);not null"`
	BetAmount     float64    `gorm:"type:decimal(10,2);not null"`
	RiskLevel     string     `gorm:"type:varchar(20);not null"` // low, medium, high
	Status        string     `gorm:"type:varchar(20);not null"` // active, won, lost, cashed_out
	RevealedCount int        `gorm:"type:int;default:0"`
	CurrentPayout float64    `gorm:"type:decimal(10,2);not null"`
	Multiplier    float64    `gorm:"type:decimal(10,2);default:1.0"`
	GridData      string     `gorm:"type:text;not null"` // JSON string of the grid
	CreatedAt     time.Time  ``
	UpdatedAt     time.Time  ``
	CompletedAt   *time.Time ``

	User User `gorm:"foreignKey:UserId"`
}

type MineGameHistory struct {
	Id          string    `gorm:"primaryKey;type:varchar(100)"`
	GameId      string    `gorm:"type:varchar(100);not null"`
	TileIndex   int       `gorm:"type:int;not null"`
	TileType    string    `gorm:"type:varchar(20);not null"` // diamond, bomb
	Multiplier  float64   `gorm:"type:decimal(10,2);not null"`
	PayoutAtHit float64   `gorm:"type:decimal(10,2);not null"`
	CreatedAt   time.Time ``

	Game MineGame `gorm:"foreignKey:GameId"`
}
