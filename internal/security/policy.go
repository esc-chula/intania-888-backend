package security

import (
	"context"
	"errors"
	"strings"
)

const (
	// RoleUser is the ordinary account role stored in the database.
	RoleUser = "USER"
	// RoleAdmin is the administrator role required by privileged routes.
	RoleAdmin = "ADMIN"
)

// ErrPolicyUnavailable indicates access policy could not be evaluated safely.
var ErrPolicyUnavailable = errors.New("access policy is unavailable")

// PolicyDecision is the result of evaluating an identity against the
// database-backed access policy.
type PolicyDecision struct {
	Allowed     bool
	Blacklisted bool
}

// AccessPolicyChecker is the policy boundary required by authentication and
// request middleware. The persistence implementation lives in the policy
// domain package so these consumers do not depend on its adapters.
type AccessPolicyChecker interface {
	// EvaluateLogin evaluates allowlist and blacklist rules for a login identity.
	EvaluateLogin(ctx context.Context, email, googleSubject, role string) (PolicyDecision, error)
	// IsBlacklisted checks request-time denial rules for an existing account.
	IsBlacklisted(ctx context.Context, email, userID string) (bool, error)
}

// DefaultPolicyChecker keeps isolated unit tests useful when they do not need
// a persistence-backed policy. Production wiring always supplies the policy
// service.
type DefaultPolicyChecker struct{}

// EvaluateLogin admits student email addresses or existing administrator roles.
// This isolated checker does not consult persistence or blacklist rules.
func (DefaultPolicyChecker) EvaluateLogin(_ context.Context, email, _ string, role string) (PolicyDecision, error) {
	return PolicyDecision{
		Allowed: IsStudentEmail(email) || IsAdminRole(role),
	}, nil
}

// IsBlacklisted always returns false for this isolated test checker.
func (DefaultPolicyChecker) IsBlacklisted(context.Context, string, string) (bool, error) {
	return false, nil
}

// NormalizeEmail trims surrounding whitespace and lowercases an email identity.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeGoogleSubject trims whitespace without changing the subject case.
func NormalizeGoogleSubject(subject string) string {
	return strings.TrimSpace(subject)
}

// IsStudentEmail checks the normalized student.chula.ac.th email suffix.
func IsStudentEmail(email string) bool {
	return strings.HasSuffix(NormalizeEmail(email), "@student.chula.ac.th")
}

// IsAdminRole checks the case-sensitive administrator role after trimming whitespace.
func IsAdminRole(role string) bool {
	return strings.TrimSpace(role) == RoleAdmin
}
