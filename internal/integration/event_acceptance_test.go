//go:build integration

package integration_test

import (
	"sync"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/event"
	"github.com/esc-chula/intania-888-backend/internal/model"
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
