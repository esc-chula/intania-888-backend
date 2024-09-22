package model

import "time"

type User struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Email     string    `gorm:"type:varchar(100);not null"`
	Name      string    `gorm:"type:varchar(100);not null"`
	RoleId    string    `gorm:"type:varchar(100);not null"`
	GroupId   *string   `gorm:"type:varchar(100);"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

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

	Members    []IntaniaGroup `gorm:"foreignKey:ColorId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	BillLines  []BillLine     `gorm:"foreignKey:BettingOn;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamA      []Match        `gorm:"foreignKey:TeamA_Id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamB      []Match        `gorm:"foreignKey:TeamB_Id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Won        []Match        `gorm:"foreignKey:WinnerId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	GroupLines []GroupLine    `gorm:"foreignKey:TeamId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
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
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	TeamA_Id  string    `gorm:"type:varchar(100);not null"`
	TeamB_Id  string    `gorm:"type:varchar(100);not null"`
	WinnerId  string    `gorm:"type:varchar(100);not null"`
	TypeId    string    `gorm:"type:varchar(100);not null"`
	StartTime time.Time ``
	EndTime   time.Time ``
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

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

type SportType struct {
	Id        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Matches         []Match     `gorm:"foreignKey:TypeId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TournamentGroup []GroupHead `gorm:"foreignKey:TypeId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
