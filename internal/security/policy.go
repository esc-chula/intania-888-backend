package security

import (
	"errors"
	"strings"
)

const (
	RoleUser  = "USER"
	RoleAdmin = "ADMIN"
)

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
	EvaluateLogin(email, googleSubject, role string) (PolicyDecision, error)
	IsBlacklisted(email, userID string) (bool, error)
}

// DefaultPolicyChecker keeps isolated unit tests useful when they do not need
// a persistence-backed policy. Production wiring always supplies the policy
// service.
type DefaultPolicyChecker struct{}

func (DefaultPolicyChecker) EvaluateLogin(email, _ string, role string) (PolicyDecision, error) {
	return PolicyDecision{
		Allowed: IsStudentEmail(email) || IsAdminRole(role),
	}, nil
}

func (DefaultPolicyChecker) IsBlacklisted(string, string) (bool, error) {
	return false, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizeGoogleSubject(subject string) string {
	return strings.TrimSpace(subject)
}

func IsStudentEmail(email string) bool {
	return strings.HasSuffix(NormalizeEmail(email), "@student.chula.ac.th")
}

func IsAdminRole(role string) bool {
	return strings.TrimSpace(role) == RoleAdmin
}
