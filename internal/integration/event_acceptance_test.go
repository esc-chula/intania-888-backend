//go:build integration

package integration_test

import (
	"sync"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/event"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
)

func TestSlotSpinConcurrentCommitPreservesBalance(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "slot-user", 10000)

	repository := event.NewEventRepository(postgres.DB, cache.RedisClient{})
	spend := model.MustMoneyFromMinor(5000)
	reward := model.MustMoneyFromMinor(7500)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- repository.CommitSlotSpin("slot-user", spend, reward, nil)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("slot spin commit failed: %v", err)
		}
	}

	var user model.User
	if err := postgres.DB.First(&user, "id = ?", "slot-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 15000 {
		t.Fatalf("user balance = %d, want 15000", user.RemainingCoin)
	}
}

func TestSlotSpinRejectsConcurrentOverspend(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "slot-balance-user", 5000)

	repository := event.NewEventRepository(postgres.DB, cache.RedisClient{})
	spend := model.MustMoneyFromMinor(5000)
	zeroReward := model.MustMoneyFromMinor(0)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- repository.CommitSlotSpin("slot-balance-user", spend, zeroReward, nil)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}

	if successes != 1 {
		t.Fatalf("concurrent slot successes = %d, want 1", successes)
	}

	var user model.User
	if err := postgres.DB.First(&user, "id = ?", "slot-balance-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 0 {
		t.Fatalf("user balance = %d, want 0", user.RemainingCoin)
	}
}

func seedStealToken(t *testing.T, postgres *testutil.Postgres, id, ownerID, token, victimID string) {
	t.Helper()

	_, err := postgres.SQL.Exec(
		`INSERT INTO steal_tokens(id, user_id, token, is_used, allowed_victim_ids, expires_at)
		 VALUES($1, $2, $3, false, $4, $5)`,
		id,
		ownerID,
		token,
		victimID,
		time.Now().Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStealTokenConcurrentUseTransfersOnce(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "thief-user", 0)
	seedStakeMineUser(t, postgres, "victim-user", 10000)
	seedStealToken(t, postgres, "steal-token", "thief-user", "steal-value", "victim-user")

	repository := event.NewEventRepository(postgres.DB, cache.RedisClient{})
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repository.ConsumeStealToken("thief-user", "steal-value", 0)
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}

	if successes != 1 {
		t.Fatalf("concurrent steal successes = %d, want 1", successes)
	}

	var thief, victim model.User
	if err := postgres.DB.First(&thief, "id = ?", "thief-user").Error; err != nil {
		t.Fatal(err)
	}
	if err := postgres.DB.First(&victim, "id = ?", "victim-user").Error; err != nil {
		t.Fatal(err)
	}

	if thief.RemainingCoin != 5000 {
		t.Fatalf("thief balance = %d, want 5000", thief.RemainingCoin)
	}
	if victim.RemainingCoin != 8000 {
		t.Fatalf("victim balance = %d, want 8000", victim.RemainingCoin)
	}

	var token model.StealToken
	if err := postgres.DB.First(&token, "id = ?", "steal-token").Error; err != nil {
		t.Fatal(err)
	}
	if !token.IsUsed {
		t.Fatal("steal token was not marked used")
	}
}

func TestStealTokenEnforcesHundredCoinVictimFloor(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "floor-thief", 0)
	seedStakeMineUser(t, postgres, "floor-victim", 9999)
	seedStealToken(t, postgres, "floor-token", "floor-thief", "floor-value", "floor-victim")

	repository := event.NewEventRepository(postgres.DB, cache.RedisClient{})
	if _, err := repository.ConsumeStealToken("floor-thief", "floor-value", 0); err == nil {
		t.Fatal("steal succeeded for victim below 100.00 coins")
	}

	if err := postgres.DB.Model(&model.User{}).
		Where("id = ?", "floor-victim").
		Update("remaining_coin", 10000).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := repository.ConsumeStealToken("floor-thief", "floor-value", 0); err != nil {
		t.Fatalf("steal at exactly 100.00 coins failed: %v", err)
	}
}
