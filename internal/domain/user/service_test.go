package user

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type fakeAccountRepository struct {
	Repository
	user         *identity.User
	seenContext  context.Context
	savedActorID string
	savedProfile *ProfilePatch
	calls        []string
	deductError  error
	lookupError  error
}

func (r *fakeAccountRepository) GetByID(ctx context.Context, _ string) (*identity.User, error) {
	r.seenContext = ctx
	if r.lookupError != nil {
		return nil, r.lookupError
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeAccountRepository) PatchProfile(ctx context.Context, actorID string, input ProfilePatch) error {
	r.seenContext = ctx
	r.savedActorID = actorID
	r.savedProfile = &input
	if input.Name != nil {
		r.user.Name = *input.Name
	}
	if input.NickNameSet {
		r.user.NickName = input.NickName
	}
	if input.GroupIDSet {
		r.user.GroupID = input.GroupID
	}
	return nil
}

func (r *fakeAccountRepository) WithinTransaction(ctx context.Context, fn func(Transaction) error) error {
	r.seenContext = ctx
	r.calls = append(r.calls, "begin")
	err := fn(r)
	if err != nil {
		r.calls = append(r.calls, "rollback")
		return err
	}
	r.calls = append(r.calls, "commit")
	return nil
}

func (r *fakeAccountRepository) LockByID(ctx context.Context, _ string) (*identity.User, error) {
	r.seenContext = ctx
	r.calls = append(r.calls, "lock")
	if r.lookupError != nil {
		return nil, r.lookupError
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeAccountRepository) DeductBalance(ctx context.Context, _ string, _ int64) error {
	r.seenContext = ctx
	r.calls = append(r.calls, "deduct")
	return r.deductError
}

func TestDeductCoinUsesTransactionAndPreservesContextAndFailures(t *testing.T) {
	storageError := errors.New("database unavailable")
	tests := []struct {
		name          string
		balance       int64
		deduction     int64
		lookupError   error
		deductError   error
		wantError     error
		wantCalls     []string
		wantRemaining int64
	}{
		{name: "exact balance", balance: 10000, deduction: 10000, wantRemaining: 0, wantCalls: []string{"begin", "lock", "deduct", "commit"}},
		{name: "insufficient balance", balance: 9999, deduction: 10000, wantError: ErrInsufficientBalance, wantCalls: []string{"begin", "lock", "rollback"}},
		{name: "missing user", lookupError: ErrUserNotFound, deduction: 10000, wantError: ErrUserNotFound, wantCalls: []string{"begin", "lock", "rollback"}},
		{name: "write failure", balance: 20000, deduction: 10000, deductError: storageError, wantError: storageError, wantCalls: []string{"begin", "lock", "deduct", "rollback"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			repo := &fakeAccountRepository{
				user:        &identity.User{ID: "user", RemainingCoin: test.balance},
				lookupError: test.lookupError,
				deductError: test.deductError,
			}
			remaining, err := NewService(repo, zap.NewNop()).DeductCoin(ctx, "user", value.MustMoneyFromMinor(test.deduction))
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
			if !reflect.DeepEqual(repo.calls, test.wantCalls) {
				t.Fatalf("calls = %v, want %v", repo.calls, test.wantCalls)
			}
			if repo.seenContext != ctx {
				t.Fatal("request context was not propagated")
			}
			if err == nil && remaining.MinorUnits() != test.wantRemaining {
				t.Fatalf("remaining = %d, want %d", remaining.MinorUnits(), test.wantRemaining)
			}
		})
	}
}

func TestUpdateOwnProfilePreservesBalanceAndResponseTimestamp(t *testing.T) {
	repo := &fakeAccountRepository{user: &identity.User{ID: "actor", RemainingCoin: 12345, CreatedAt: time.Now()}}
	ctx := context.Background()
	name := "Updated"
	result, err := NewService(repo, zap.NewNop()).UpdateOwnProfile(ctx, "actor", ProfilePatch{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingCoin.MinorUnits() != 12345 {
		t.Fatalf("profile response changed the observed balance: result=%v", result)
	}
	if repo.savedActorID != "actor" || repo.savedProfile == nil ||
		repo.savedProfile.Name == nil || *repo.savedProfile.Name != "Updated" {
		t.Fatalf("profile update was not saved for actor: actor=%q input=%+v", repo.savedActorID, repo.savedProfile)
	}
	if !result.CreatedAt.IsZero() {
		t.Fatalf("single-user response timestamp changed: %v", result.CreatedAt)
	}
}
