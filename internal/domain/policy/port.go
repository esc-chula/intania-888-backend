package policy

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
)

const (
	KindAllowlist = "allowlist"
	KindBlacklist = "blacklist"

	PrincipalEmail         = "email"
	PrincipalGoogleSubject = "google_subject"

	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusAll      = "all"
)

type AccessPolicy struct {
	ID            string     `gorm:"primaryKey;type:varchar(100)" json:"id"`
	Kind          string     `gorm:"type:varchar(20);not null" json:"kind"`
	PrincipalType string     `gorm:"column:principal_type;type:varchar(20);not null" json:"principal_type"`
	Principal     string     `gorm:"type:varchar(320);not null" json:"principal"`
	Reason        string     `gorm:"type:varchar(500);not null" json:"reason"`
	Enabled       bool       `gorm:"not null;default:true" json:"enabled"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (AccessPolicy) TableName() string { return "auth_access_policies" }

type CreateInput struct {
	Kind          string     `json:"kind"`
	PrincipalType string     `json:"principal_type"`
	Principal     string     `json:"principal"`
	Reason        string     `json:"reason"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

type BootstrapFile struct {
	Entries []CreateInput `json:"entries"`
}

type UpdateInput struct {
	Reason       string
	ReasonSet    bool
	ExpiresAt    *time.Time
	ExpiresAtSet bool
	Enabled      bool
	EnabledSet   bool
}

type ListFilter struct {
	Kind          string
	PrincipalType string
	Status        string
	Limit         int
	Offset        int
}

type ListResult struct {
	Items   []*AccessPolicy
	HasMore bool
}

type Repository interface {
	ListActive(now time.Time) ([]*AccessPolicy, error)
	List(filter ListFilter) (ListResult, error)
	FindByID(id string) (*AccessPolicy, error)
	FindByIdentity(kind, principalType, principal string) (*AccessPolicy, error)
	Create(policy *AccessPolicy) error
	Update(policy *AccessPolicy) error
}

type Cache interface {
	SetValue(key string, value interface{}, ttl int) error
	GetValue(key string, value interface{}) error
	DeleteValue(key string) error
}

type Service interface {
	security.AccessPolicyChecker
	List(filter ListFilter) (ListResult, error)
	Create(input CreateInput) (*AccessPolicy, error)
	Update(id string, input UpdateInput) (*AccessPolicy, error)
	Disable(id string) (*AccessPolicy, error)
	RefreshCache() error
}
