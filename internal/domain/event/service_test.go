package event

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type memoryEventRepository struct {
	users      map[string]identity.User
	rewards    map[string]DailyReward
	claims     map[string]value.Money
	tokens     map[string]StealToken
	candidates []identity.User
	fail       string
	calls      []string
}

func newMemoryEventRepository() *memoryEventRepository {
	return &memoryEventRepository{
		users:   make(map[string]identity.User),
		rewards: make(map[string]DailyReward),
		claims:  make(map[string]value.Money),
		tokens:  make(map[string]StealToken),
	}
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	clone := make(map[K]V, len(source))
	for key, item := range source {
		clone[key] = item
	}

	return clone
}

func (r *memoryEventRepository) WithinTransaction(ctx context.Context, fn func(TransactionRepository) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	next := *r
	next.users, next.rewards = cloneMap(r.users), cloneMap(r.rewards)
	next.claims, next.tokens = cloneMap(r.claims), cloneMap(r.tokens)
	if err := fn(&next); err != nil {
		r.calls = next.calls

		return err
	}
	*r = next

	return nil
}

func (r *memoryEventRepository) ListRewards(context.Context) ([]DailyReward, error) {
	rewards := make([]DailyReward, 0, len(r.rewards))
	for _, reward := range r.rewards {
		rewards = append(rewards, reward)
	}

	return rewards, nil
}

func (r *memoryEventRepository) SetReward(_ context.Context, reward DailyReward) error {
	r.rewards[reward.Date] = reward

	return nil
}

func (r *memoryEventRepository) DeleteReward(_ context.Context, date string) error {
	if _, ok := r.rewards[date]; !ok {
		return ErrDailyRewardOverrideNotFound
	}
	delete(r.rewards, date)

	return nil
}

func (*memoryEventRepository) DeleteExpiredTokens(context.Context, time.Time) error {
	return nil
}

func (r *memoryEventRepository) GetUser(_ context.Context, id string) (identity.User, error) {
	actor, ok := r.users[id]
	if !ok {
		return identity.User{}, ErrUserNotFound
	}

	return actor, nil
}

func (r *memoryEventRepository) GetRandomEligibleUsers(context.Context, string, int64, int) ([]identity.User, error) {
	if r.fail == "candidates" {
		return nil, errors.New("candidate lookup failed")
	}

	return r.candidates, nil
}

func (r *memoryEventRepository) GetReward(_ context.Context, date string) (DailyReward, error) {
	r.calls = append(r.calls, "reward:"+date)
	reward, ok := r.rewards[date]
	if !ok {
		return DailyReward{}, ErrDailyRewardOverrideNotFound
	}

	return reward, nil
}

func (r *memoryEventRepository) LockUser(ctx context.Context, id string) (identity.User, error) {
	r.calls = append(r.calls, "lock:"+id)

	return r.GetUser(ctx, id)
}

func (r *memoryEventRepository) CreateDailyClaim(_ context.Context, userID, date string, reward value.Money) (bool, error) {
	r.calls = append(r.calls, "claim:"+userID)
	key := userID + ":" + date
	if _, exists := r.claims[key]; exists {
		return false, nil
	}
	r.claims[key] = reward

	return true, nil
}

func (r *memoryEventRepository) SetUserBalance(_ context.Context, id string, balance value.Money) error {
	r.calls = append(r.calls, "balance:"+id)
	if r.fail == "balance:"+id {
		return errors.New("balance update failed")
	}
	actor := r.users[id]
	actor.RemainingCoin = balance.MinorUnits()
	r.users[id] = actor

	return nil
}

func (r *memoryEventRepository) CreateStealToken(_ context.Context, token StealToken) error {
	r.calls = append(r.calls, "create-token")
	if r.fail == "create-token" {
		return errors.New("token insertion failed")
	}
	r.tokens[token.Token] = token

	return nil
}

func (r *memoryEventRepository) LockStealToken(_ context.Context, token string) (StealToken, error) {
	r.calls = append(r.calls, "lock-token")
	found, ok := r.tokens[token]
	if !ok {
		return StealToken{}, ErrStealTokenInvalid
	}

	return found, nil
}

func (r *memoryEventRepository) GetUsersByIDs(_ context.Context, ids []string) ([]identity.User, error) {
	r.calls = append(r.calls, "candidate-snapshot")
	users := make([]identity.User, 0, len(ids))
	for _, id := range ids {
		if actor, ok := r.users[id]; ok {
			users = append(users, actor)
		}
	}

	return users, nil
}

func (r *memoryEventRepository) MarkTokenUsed(_ context.Context, id string) (bool, error) {
	r.calls = append(r.calls, "use-token")
	if r.fail == "use-token" {
		return false, errors.New("token update failed")
	}
	for key, token := range r.tokens {
		if token.ID == id && !token.IsUsed {
			token.IsUsed = true
			r.tokens[key] = token

			return true, nil
		}
	}

	return false, nil
}

func TestDailyRewardUsesBangkokDateOverrideAndSingleClaim(t *testing.T) {
	repo := newMemoryEventRepository()
	repo.users["actor"] = identity.User{
		ID:            "actor",
		RemainingCoin: 100_00,
	}
	repo.rewards["25-09-2026"] = DailyReward{
		Date:   "25-09-2026",
		Reward: 123_45,
	}
	service := NewService(repo, value.MustMoneyFromMinor(300_00), nil)
	service.now = func() time.Time {
		return time.Date(2026, time.September, 24, 17, 0, 0, 0, time.UTC)
	}

	if err := service.RedeemDailyReward(context.Background(), "actor"); err != nil {
		t.Fatal(err)
	}
	if got := repo.users["actor"].RemainingCoin; got != 223_45 {
		t.Fatalf("balance = %d, want 22345", got)
	}
	if err := service.RedeemDailyReward(context.Background(), "actor"); !errors.Is(err, ErrDailyRewardAlreadyClaimed) {
		t.Fatalf("repeat claim error = %v", err)
	}
	if got := repo.users["actor"].RemainingCoin; got != 223_45 {
		t.Fatalf("repeat credited balance = %d", got)
	}
}

func TestDailyRewardDefaultAndRollback(t *testing.T) {
	for _, fail := range []string{"", "balance:actor"} {
		t.Run(fail, func(t *testing.T) {
			repo := newMemoryEventRepository()
			repo.users["actor"] = identity.User{
				ID:            "actor",
				RemainingCoin: 100_00,
			}
			repo.fail = fail
			service := NewService(repo, value.MustMoneyFromMinor(300_00), nil)
			err := service.RedeemDailyReward(context.Background(), "actor")
			if fail == "" {
				if err != nil {
					t.Fatal(err)
				}
				if repo.users["actor"].RemainingCoin != 400_00 || len(repo.claims) != 1 {
					t.Fatalf("default reward state = %+v claims=%v", repo.users, repo.claims)
				}
			} else if err == nil || repo.users["actor"].RemainingCoin != 100_00 || len(repo.claims) != 0 {
				t.Fatalf("failed claim state: error=%v users=%+v claims=%v", err, repo.users, repo.claims)
			}
		})
	}
}

func TestSlotTokenFailureRollsBackDebit(t *testing.T) {
	repo := newMemoryEventRepository()
	repo.users["actor"] = identity.User{
		ID:            "actor",
		RemainingCoin: 100_00,
	}
	repo.fail = "create-token"
	service := NewService(repo, value.Money{}, nil)
	token := &StealToken{
		ID:     "id",
		UserID: "actor",
		Token:  "token",
	}
	if err := service.CommitSlotSpin(context.Background(), "actor", value.MustMoneyFromMinor(50_00), value.Money{}, token); err == nil {
		t.Fatal("token failure was ignored")
	}
	if repo.users["actor"].RemainingCoin != 100_00 || len(repo.tokens) != 0 {
		t.Fatalf("failed spin changed state: %+v", repo)
	}
}

func TestAlienSpinTokenAndFallbackRewards(t *testing.T) {
	for _, candidateLookupFails := range []bool{false, true} {
		t.Run(map[bool]string{
			false: "token",
			true:  "fallback",
		}[candidateLookupFails], func(t *testing.T) {
			repo := newMemoryEventRepository()
			repo.users["actor"] = identity.User{
				ID:            "actor",
				RemainingCoin: 100_00,
			}
			repo.candidates = []identity.User{{
				ID:            "victim",
				Name:          "Victim",
				RoleID:        "USER",
				RemainingCoin: 100_00,
			}}
			if candidateLookupFails {
				repo.fail = "candidates"
			}
			service := NewService(repo, value.Money{}, nil)
			service.drawSlot = func(identity.Profile) string {
				return "👽"
			}

			result, err := service.SpinSlotMachine(context.Background(), identity.Profile{ID: "actor"}, value.MustMoneyFromMinor(50_00))
			if err != nil {
				t.Fatal(err)
			}
			if candidateLookupFails {
				if result.StealToken != nil ||
					result.Reward.MinorUnits() != 200_00 ||
					repo.users["actor"].RemainingCoin != 250_00 {
					t.Fatalf("fallback result=%+v state=%+v", result, repo.users)
				}
			} else if result.StealToken == nil ||
				!result.Reward.IsZero() ||
				len(result.Candidates) != 1 ||
				repo.users["actor"].RemainingCoin != 50_00 ||
				len(repo.tokens) != 1 {
				t.Fatalf("alien result=%+v state=%+v", result, repo.users)
			}
		})
	}
}

func TestStealMinimumCreditLocksAndPreRaidSnapshot(t *testing.T) {
	repo := newMemoryEventRepository()
	repo.users["z-thief"] = identity.User{
		ID:            "z-thief",
		RemainingCoin: 0,
	}
	repo.users["a-victim"] = identity.User{
		ID:            "a-victim",
		Name:          "Victim",
		RemainingCoin: 100_00,
	}
	repo.tokens["token"] = StealToken{
		ID:               "id",
		UserID:           "z-thief",
		Token:            "token",
		AllowedVictimIDs: []string{"deleted", "a-victim"},
		ExpiresAt:        time.Now().Add(time.Minute),
	}
	service := NewService(repo, value.Money{}, nil)
	result, err := service.UseStealToken(context.Background(), "z-thief", "token", 1)
	if err != nil {
		t.Fatal(err)
	}
	if repo.users["a-victim"].RemainingCoin != 80_00 ||
		repo.users["z-thief"].RemainingCoin != 50_00 ||
		!repo.tokens["token"].IsUsed {
		t.Fatalf("raid state=%+v", repo)
	}
	if result.TotalStolen.MinorUnits() != 50_00 ||
		result.AllCandidates[1].BalanceBefore.MinorUnits() != 100_00 ||
		result.AllCandidates[0].Name != "[Deleted User]" {
		t.Fatalf("raid response=%+v", result)
	}
	if want := []string{"lock-token", "lock:a-victim", "lock:z-thief", "candidate-snapshot", "balance:a-victim", "balance:z-thief", "use-token"}; !reflect.DeepEqual(repo.calls, want) {
		t.Fatalf("locking/write order=%v, want %v", repo.calls, want)
	}
	if _, err := service.UseStealToken(context.Background(), "z-thief", "token", 1); !errors.Is(err, ErrStealTokenConflict) {
		t.Fatalf("repeat raid error=%v", err)
	}
}

func TestStealTokenWriteFailureRollsBackBothBalances(t *testing.T) {
	repo := newMemoryEventRepository()
	repo.users["thief"] = identity.User{
		ID:            "thief",
		RemainingCoin: 0,
	}
	repo.users["victim"] = identity.User{
		ID:            "victim",
		RemainingCoin: 100_00,
	}
	repo.tokens["token"] = StealToken{
		ID:               "id",
		UserID:           "thief",
		Token:            "token",
		AllowedVictimIDs: []string{"victim"},
		ExpiresAt:        time.Now().Add(time.Minute),
	}
	repo.fail = "use-token"
	if _, err := NewService(repo, value.Money{}, nil).UseStealToken(context.Background(), "thief", "token", 0); err == nil {
		t.Fatal("token failure was ignored")
	}
	if repo.users["victim"].RemainingCoin != 100_00 ||
		repo.users["thief"].RemainingCoin != 0 ||
		repo.tokens["token"].IsUsed {
		t.Fatalf("failed raid changed state: %+v", repo)
	}
}
