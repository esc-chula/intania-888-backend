package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type externalGrantConfig struct{ config.Config }

func (externalGrantConfig) GetServer() config.Server { return config.Server{Name: "888"} }
func (externalGrantConfig) GetJWT() config.JWT       { return config.JWT{AccessTokenSecret: "secret"} }
func (externalGrantConfig) GetOAuth() config.OAuth {
	return config.OAuth{Registry: &config.AuthRegistry{
		Lifetimes:    config.AuthLifetimes{Idle: 600},
		Applications: []config.AuthApplication{{ID: "games", Mode: config.CodeApplication, AllowedScopes: []string{config.ScopeProfileRead}}},
	}}
}

type externalGrantStore struct {
	SessionStore
	grant *security.Delegation
	err   error
	calls int
}

func (s *externalGrantStore) ReadDelegation(context.Context, string, int) (*security.Delegation, error) {
	s.calls++
	return s.grant, s.err
}

type externalAccountRepository struct{ calls int }

func (r *externalAccountRepository) GetByID(context.Context, string) (*identity.User, error) {
	r.calls++
	return &identity.User{ID: "user", Email: "u@student.chula.ac.th", RoleID: "USER"}, nil
}

func TestExternalGrantVerificationPrecedesAccountQueries(t *testing.T) {
	grant := security.Delegation{
		ID: "grant", UserID: "user", ClientID: "games", Scopes: []string{config.ScopeProfileRead},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := security.SignDelegatedToken(grant, "secret", "888", 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		record *security.Delegation
		cause  error
		scope  string
		want   error
	}{
		{"valid", &grant, nil, config.ScopeProfileRead, nil},
		{"revoked", nil, ErrExternalMissing, config.ScopeProfileRead, ErrExternalMissing},
		{"ownership mismatch", &security.Delegation{UserID: "other", ClientID: "games"}, nil, config.ScopeProfileRead, ErrExternalMissing},
		{"scope mismatch", &grant, nil, config.ScopeCoinsSpend, ErrExternalScope},
		{"storage outage", nil, context.DeadlineExceeded, config.ScopeProfileRead, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &externalGrantStore{grant: tc.record, err: tc.cause}
			repo := &externalAccountRepository{}
			service := NewService(repo, store, externalGrantConfig{}, security.DefaultPolicyChecker{})
			id, err := service.VerifyExternalGrant(context.Background(), token, tc.scope)
			if !errors.Is(err, tc.want) || repo.calls != 0 {
				t.Fatalf("grant error=%v, account calls=%d", err, repo.calls)
			}
			if err == nil {
				profile, err := service.GetExternalProfile(context.Background(), id)
				if err != nil || profile.ID != "user" || repo.calls != 1 {
					t.Fatalf("profile=%+v, error=%v, calls=%d", profile, err, repo.calls)
				}
			}
		})
	}
}
