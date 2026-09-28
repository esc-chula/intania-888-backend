package model

import "time"

// User maps the users table. RemainingCoin stores non-negative hundredth
// units; nil NickName or GroupID represents SQL NULL.
type User struct {
	ID            string    `gorm:"primaryKey;type:varchar(100)"`
	Email         string    `gorm:"type:varchar(100);not null"`
	Name          string    `gorm:"type:varchar(100);not null"`
	NickName      *string   `gorm:"type:varchar(100);"`
	RoleID        string    `gorm:"type:varchar(100);not null"`
	GroupID       *string   `gorm:"type:varchar(100);"`
	RemainingCoin int64     `gorm:"column:remaining_coin;type:bigint;not null;default:0"`
	CreatedAt     time.Time ``
	UpdatedAt     time.Time ``

	Role  Role         `gorm:"foreignKey:RoleID"`
	Group IntaniaGroup `gorm:"foreignKey:GroupID"`
	Bills []BillHead   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Role maps a stored account role and its optional loaded user association.
type Role struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Users []User `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Color maps a team color. Leaderboard queries also populate its aggregate
// match, win, and draw counts.
type Color struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Members   []IntaniaGroup `gorm:"foreignKey:ColorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	BillLines []BillLine     `gorm:"foreignKey:BettingOn;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamA     []Match        `gorm:"foreignKey:TeamAID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TeamB     []Match        `gorm:"foreignKey:TeamBID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Won        []Match        `gorm:"foreignKey:WinnerID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	GroupLines []GroupLine `gorm:"foreignKey:TeamID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	TotalMatches int `gorm:"type:int;"`
	Won          int `gorm:"type:int;"`
	Drawn        int `gorm:"type:int;"`
}

// IntaniaGroup maps account membership groups and their team color.
type IntaniaGroup struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	ColorID   string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Color   Color  `gorm:"foreignKey:ColorID"`
	Members []User `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Match maps scheduled teams, nullable scores, and the terminal outcome.
// Nil team or score pointers preserve SQL NULL; nil WinnerID with IsDraw false
// means no winner has been recorded, while IsDraw true identifies a draw.
type Match struct {
	ID         string  `gorm:"primaryKey;type:varchar(100)"`
	TeamAID    *string `gorm:"column:teama_id;type:varchar(100);"`
	TeamBID    *string `gorm:"column:teamb_id;type:varchar(100);"`
	TeamAScore *int    `gorm:"column:teama_score;type:int;"`
	TeamBScore *int    `gorm:"column:teamb_score;type:int;"`

	WinnerID  *string   `gorm:"column:winner_id;type:varchar(100);"`
	TypeID    string    `gorm:"column:type_id;type:varchar(100);not null"`
	IsDraw    bool      `gorm:"column:is_draw;type:boolean;default:false"`
	StartTime time.Time `gorm:"column:start_time"`
	EndTime   time.Time `gorm:"column:end_time"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`

	BillLines []BillLine `gorm:"foreignKey:MatchID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SportType SportType  `gorm:"foreignKey:TypeID"`
	TeamA     Color      `gorm:"foreignKey:TeamAID"`
	TeamB     Color      `gorm:"foreignKey:TeamBID"`
	Winner    Color      `gorm:"foreignKey:WinnerID"`
}

// BillHead maps a bill lifecycle. Total and Payout use hundredth units.
// Pending bills have nil Payout, SettledAt, and VoidedAt; settled bills have a
// payout and settlement time, and voided bills have a refund and void time.
type BillHead struct {
	ID        string     `gorm:"primaryKey;type:varchar(100)"`
	Total     int64      `gorm:"column:total;type:bigint;not null"`
	UserID    string     `gorm:"type:varchar(100);not null"`
	Status    string     `gorm:"type:varchar(20);not null"`
	Payout    *int64     `gorm:"type:bigint"`
	SettledAt *time.Time ``
	VoidedAt  *time.Time ``
	CreatedAt time.Time  ``
	UpdatedAt time.Time  ``

	User  User       `gorm:"foreignKey:UserID"`
	Lines []BillLine `gorm:"foreignKey:BillID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// BillLine maps a selection using the composite bill/match primary key.
// Rate is the authoritative rate captured at placement, in millionth units.
type BillLine struct {
	BillID    string    `gorm:"primaryKey;type:varchar(100)"`
	MatchID   string    `gorm:"primaryKey;type:varchar(100)"`
	Rate      int64     `gorm:"column:rate;type:bigint;not null"`
	BettingOn string    `gorm:"type:varchar(100);not null"` // color
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Match Match    `gorm:"foreignKey:MatchID"`
	Head  BillHead `gorm:"foreignKey:BillID"`
	Color Color    `gorm:"foreignKey:BettingOn"`
}

// GroupHead maps a tournament group and its optional loaded team lines.
type GroupHead struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	TypeID    string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Lines     []GroupLine `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SportType SportType   `gorm:"foreignKey:TypeID"`
}

// GroupLine maps a team in a tournament group using a composite primary key.
type GroupLine struct {
	GroupID   string    `gorm:"primaryKey;type:varchar(100)"`
	TeamID    string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Head GroupHead `gorm:"foreignKey:GroupID"`
	Team Color     `gorm:"foreignKey:TeamID"`
}

// GroupStage maps group-stage membership by stage, sport type, and color.
type GroupStage struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	TypeID    string    `gorm:"primaryKey;type:varchar(100)"`
	ColorID   string    `gorm:"primaryKey;type:varchar(100)"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	SportType SportType `gorm:"foreignKey:TypeID"`
	Color     Color     `gorm:"foreignKey:ColorID"`
}

// SportType maps a sport and its optional loaded matches and tournament groups.
type SportType struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	Title     string    `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``

	Matches         []Match     `gorm:"foreignKey:TypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	TournamentGroup []GroupHead `gorm:"foreignKey:TypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

// DailyReward maps a date-specific daily reward override. Reward uses
// hundredth units; application dates use the Bangkok calendar and DD-MM-YYYY format.
type DailyReward struct {
	Date      string    `gorm:"primaryKey;type:varchar(100)"` // DD-MM-YY eg. 31-10-24
	Reward    int64     `gorm:"column:reward;type:bigint;not null"`
	CreatedAt time.Time ``
	UpdatedAt time.Time ``
}

// DailyRewardClaim maps a credited daily claim. Its user/date primary key
// enforces one claim per date, and Reward records the credited hundredth units.
type DailyRewardClaim struct {
	UserID    string    `gorm:"column:user_id;primaryKey;type:varchar(100)"`
	Date      string    `gorm:"column:reward_date;primaryKey;type:varchar(100)"`
	Reward    int64     `gorm:"column:reward;type:bigint;not null"`
	CreatedAt time.Time ``

	User User `gorm:"foreignKey:UserID"`
}

// StealToken maps an expiring, single-use raid token. AllowedVictimIDs
// preserves the ordered comma-separated candidates used by the victim index.
type StealToken struct {
	ID               string    `gorm:"primaryKey;type:varchar(100)"`
	UserID           string    `gorm:"type:varchar(100);not null;index"`
	Token            string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	IsUsed           bool      `gorm:"type:boolean;default:false"`
	AllowedVictimIDs string    `gorm:"type:text;not null"`
	ExpiresAt        time.Time `gorm:"not null;index"`
	CreatedAt        time.Time ``
	UpdatedAt        time.Time ``

	User User `gorm:"foreignKey:UserID"`
}

// MineGame maps a mines round. BetAmount and CurrentPayout use hundredth
// units, and Multiplier uses millionth units. GridData preserves the full JSON
// grid; CompletedAt is nil while the round is active.
type MineGame struct {
	ID            string     `gorm:"primaryKey;type:varchar(100)"`
	UserID        string     `gorm:"type:varchar(100);not null"`
	BetAmount     int64      `gorm:"column:bet_amount;type:bigint;not null"`
	RiskLevel     string     `gorm:"type:varchar(20);not null"` // low, medium, high
	Status        string     `gorm:"type:varchar(20);not null"` // active, won, lost, cashed_out
	RevealedCount int        `gorm:"type:int;default:0"`
	CurrentPayout int64      `gorm:"column:current_payout;type:bigint;not null"`
	Multiplier    int64      `gorm:"column:multiplier;type:bigint;not null;default:1000000"`
	GridData      string     `gorm:"type:text;not null"` // JSON string of the grid
	CreatedAt     time.Time  ``
	UpdatedAt     time.Time  ``
	CompletedAt   *time.Time ``

	User User `gorm:"foreignKey:UserID"`
}

// MineGameHistory maps a tile-reveal record. Multiplier uses millionth
// units and PayoutAtHit captures the payout in hundredth units at that reveal.
type MineGameHistory struct {
	ID          string    `gorm:"primaryKey;type:varchar(100)"`
	GameID      string    `gorm:"type:varchar(100);not null"`
	TileIndex   int       `gorm:"type:int;not null"`
	TileType    string    `gorm:"type:varchar(20);not null"` // diamond, bomb
	Multiplier  int64     `gorm:"column:multiplier;type:bigint;not null"`
	PayoutAtHit int64     `gorm:"column:payout_at_hit;type:bigint;not null"`
	CreatedAt   time.Time ``

	Game MineGame `gorm:"foreignKey:GameID"`
}

// BillTerminalEvent maps the unique terminal audit event for a bill.
// Amount uses hundredth units. Voids require ActorID and Reason; settlement
// events leave those optional audit fields nil.
type BillTerminalEvent struct {
	ID        string    `gorm:"primaryKey;type:varchar(100)"`
	BillID    string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	Kind      string    `gorm:"type:varchar(20);not null"`
	Amount    int64     `gorm:"type:bigint;not null"`
	ActorID   *string   `gorm:"type:varchar(100)"`
	Reason    *string   `gorm:"type:varchar(500)"`
	CreatedAt time.Time ``
}
