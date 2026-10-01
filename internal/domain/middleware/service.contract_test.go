package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type sessionContractConfig struct{ config.Config }

func (sessionContractConfig) GetSession() config.Session {
	return config.Session{IdleTTLSeconds: 1234}
}

type sessionContractStore struct {
	SessionStore
	record      *security.Session
	err         error
	context     context.Context
	key         string
	idle        int
	invocations int
}

func (s *sessionContractStore) ReadAndRenewSession(ctx context.Context, key string, _ int64, idle int) (*security.Session, error) {
	s.context, s.key, s.idle = ctx, key, idle
	s.invocations++

	return s.record, s.err
}

type accountContractRepository struct {
	user    *identity.User
	err     error
	context context.Context
}

func (r *accountContractRepository) GetByID(ctx context.Context, _ string) (*identity.User, error) {
	r.context = ctx

	return r.user, r.err
}

func TestGetSessionPreservesValidationContextAndDependencyFailures(t *testing.T) {
	validID := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	future := time.Now().Add(time.Hour).Unix()
	tests := []struct {
		name      string
		id        string
		record    *security.Session
		err       error
		wantError error
		wantCalls int
	}{
		{"valid", validID, &security.Session{
			UserID:    "user",
			CSRFToken: "csrf",
			ExpiresAt: future,
		}, nil, nil, 1},
		{"invalid id", "invalid", nil, nil, ErrSessionMissing, 0},
		{"wrong length", base64.RawURLEncoding.EncodeToString(make([]byte, 31)), nil, nil, ErrSessionMissing, 0},
		{"missing user", validID, &security.Session{
			CSRFToken: "csrf",
			ExpiresAt: future,
		}, nil, ErrSessionMissing, 1},
		{"missing csrf", validID, &security.Session{
			UserID:    "user",
			ExpiresAt: future,
		}, nil, ErrSessionMissing, 1},
		{"expired", validID, &security.Session{
			UserID:    "user",
			CSRFToken: "csrf",
			ExpiresAt: 1,
		}, nil, ErrSessionMissing, 1},
		{"missing record", validID, nil, ErrSessionMissing, ErrSessionMissing, 1},
		{"dependency timeout", validID, nil, context.DeadlineExceeded, context.DeadlineExceeded, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &sessionContractStore{
				record: test.record,
				err:    test.err,
			}
			record, err := NewService(nil, store, sessionContractConfig{}, nil).GetSession(ctx, test.id)
			if !errors.Is(err, test.wantError) || store.invocations != test.wantCalls {
				t.Fatalf("error/calls = %v/%d, want %v/%d", err, store.invocations, test.wantError, test.wantCalls)
			}
			if test.wantCalls != 0 &&
				(store.context != ctx ||
					store.idle != 1234 ||
					store.key != security.ToSessionCacheKey(test.id)) {
				t.Fatalf("session persistence inputs changed: %+v", store)
			}
			if err == nil && record != test.record {
				t.Fatal("session result changed")
			}
			if errors.Is(test.err, context.DeadlineExceeded) && errors.Is(err, ErrSessionMissing) {
				t.Fatal("dependency timeout was misclassified as a missing session")
			}
		})
	}
}

func TestGetMePreservesMissingAccountAndDependencyCauses(t *testing.T) {
	for _, cause := range []error{identity.ErrUserNotFound, context.DeadlineExceeded} {
		ctx, cancel := context.WithCancel(context.Background())
		repo := &accountContractRepository{err: cause}
		_, err := NewService(repo, nil, nil, nil).GetMe(ctx, "user")
		cancel()
		if !errors.Is(err, cause) || repo.context != ctx {
			t.Fatalf("account lookup lost cause/context: %v", err)
		}
		if errors.Is(cause, context.DeadlineExceeded) && errors.Is(err, identity.ErrUserNotFound) {
			t.Fatal("dependency timeout was misclassified as a missing account")
		}
	}
}

func TestGetMePreservesProfileValueAndTimestampContract(t *testing.T) {
	repo := &accountContractRepository{user: &identity.User{
		ID:            "user",
		Email:         "user@example.test",
		Name:          "User",
		RoleID:        "USER",
		RemainingCoin: 12345,
		CreatedAt:     time.Now(),
	}}
	profile, err := NewService(repo, nil, nil, nil).GetMe(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "user" ||
		profile.RoleID != "USER" ||
		profile.RemainingCoin.MinorUnits() != 12345 ||
		!profile.CreatedAt.IsZero() {
		t.Fatalf("profile mapping changed: %+v", profile)
	}
}
