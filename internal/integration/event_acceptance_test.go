//go:build integration

package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/event"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestSlotSpinConcurrentCommitPreservesBalance(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "slot-user", 100_00)

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	spend := value.MustMoneyFromMinor(50_00)
	reward := value.MustMoneyFromMinor(75_00)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- service.CommitSlotSpin(context.Background(), "slot-user", spend, reward, nil)
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

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "slot-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 150_00 {
		t.Fatalf("user balance = %d, want 150_00", user.RemainingCoin)
	}
}

func TestSlotSpinRejectsConcurrentOverspend(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "slot-balance-user", 50_00)

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	spend := value.MustMoneyFromMinor(50_00)
	zeroReward := value.MustMoneyFromMinor(0)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- service.CommitSlotSpin(context.Background(), "slot-balance-user", spend, zeroReward, nil)
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

	var user persistence.User
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
	seedStakeMineUser(t, postgres, "victim-user", 100_00)
	seedStealToken(t, postgres, "steal-token", "thief-user", "steal-value", "victim-user")

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.UseStealToken(context.Background(), "thief-user", "steal-value", 0)
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

	var thief, victim persistence.User
	if err := postgres.DB.First(&thief, "id = ?", "thief-user").Error; err != nil {
		t.Fatal(err)
	}
	if err := postgres.DB.First(&victim, "id = ?", "victim-user").Error; err != nil {
		t.Fatal(err)
	}

	if thief.RemainingCoin != 50_00 {
		t.Fatalf("thief balance = %d, want 50_00", thief.RemainingCoin)
	}
	if victim.RemainingCoin != 80_00 {
		t.Fatalf("victim balance = %d, want 80_00", victim.RemainingCoin)
	}

	var token persistence.StealToken
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
	seedStakeMineUser(t, postgres, "floor-victim", 99_99)
	seedStealToken(t, postgres, "floor-token", "floor-thief", "floor-value", "floor-victim")

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	if _, err := service.UseStealToken(context.Background(), "floor-thief", "floor-value", 0); err == nil {
		t.Fatal("steal succeeded for victim below 100.00 coins")
	}

	if err := postgres.DB.Model(&persistence.User{}).
		Where("id = ?", "floor-victim").
		Update("remaining_coin", 100_00).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := service.UseStealToken(context.Background(), "floor-thief", "floor-value", 0); err != nil {
		t.Fatalf("steal at exactly 100.00 coins failed: %v", err)
	}
}

func TestDailyRewardConcurrentRedeemClaimsOnce(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "daily-user", 100_00)
	date := time.Now().In(time.FixedZone("Asia/Bangkok", 7*60*60)).Format("02-01-2006")

	if err := postgres.DB.Create(&persistence.DailyReward{Date: date, Reward: 123_45}).Error; err != nil {
		t.Fatal(err)
	}

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := service.RedeemDailyReward(context.Background(), "daily-user")
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
		t.Fatalf("concurrent daily reward successes = %d, want 1", successes)
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "daily-user").Error; err != nil {
		t.Fatal(err)
	}
	if user.RemainingCoin != 223_45 {
		t.Fatalf("user balance = %d, want 223_45", user.RemainingCoin)
	}

	var claimCount int64
	if err := postgres.DB.Model(&persistence.DailyRewardClaim{}).
		Where("user_id = ? AND reward_date = ?", "daily-user", date).
		Count(&claimCount).Error; err != nil {
		t.Fatal(err)
	}
	if claimCount != 1 {
		t.Fatalf("daily reward claims = %d, want 1", claimCount)
	}
}

func TestDailyRewardUsesDefaultWhenUnconfigured(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "default-daily-user", 100_00)

	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	if err := service.RedeemDailyReward(context.Background(), "default-daily-user"); err != nil {
		t.Fatal(err)
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "default-daily-user").Error; err != nil {
		t.Fatal(err)
	}
	if user.RemainingCoin != 400_00 {
		t.Fatalf("user balance = %d, want 400_00", user.RemainingCoin)
	}
}

func TestSlotTokenInsertFailureRollsBackBalance(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "slot-rollback", 100_00)
	cleanup, err := postgres.InstallFailureTrigger("steal_tokens", "INSERT")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	})
	service := event.NewService(event.NewGORMRepository(postgres.DB), value.Money{}, nil)
	token := &event.StealToken{ID: "rollback-token", UserID: "slot-rollback", Token: "rollback-value", ExpiresAt: time.Now().Add(time.Minute)}
	if err := service.CommitSlotSpin(context.Background(), "slot-rollback", value.MustMoneyFromMinor(50_00), value.Money{}, token); err == nil {
		t.Fatal("token insertion failure was ignored")
	}
	var actor persistence.User
	if err := postgres.DB.First(&actor, "id = ?", "slot-rollback").Error; err != nil {
		t.Fatal(err)
	}
	if actor.RemainingCoin != 100_00 {
		t.Fatalf("rolled-back balance=%d, want 10000", actor.RemainingCoin)
	}
	var count int64
	if err := postgres.DB.Model(&persistence.StealToken{}).Where("id = ?", "rollback-token").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed spin inserted %d tokens", count)
	}
}

func TestRaidTokenUpdateFailureRollsBackBothBalances(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "rollback-thief", 0)
	seedStakeMineUser(t, postgres, "rollback-victim", 100_00)
	seedStealToken(t, postgres, "rollback-token", "rollback-thief", "rollback-value", "rollback-victim")
	cleanup, err := postgres.InstallFailureTrigger("steal_tokens", "UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	})
	service := event.NewService(event.NewGORMRepository(postgres.DB), value.Money{}, nil)
	if _, err := service.UseStealToken(context.Background(), "rollback-thief", "rollback-value", 0); err == nil {
		t.Fatal("token update failure was ignored")
	}
	for id, balance := range map[string]int64{"rollback-thief": 0, "rollback-victim": 100_00} {
		var actor persistence.User
		if err := postgres.DB.First(&actor, "id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		if actor.RemainingCoin != balance {
			t.Fatalf("%s rollback balance=%d, want %d", id, actor.RemainingCoin, balance)
		}
	}
	var token persistence.StealToken
	if err := postgres.DB.First(&token, "id = ?", "rollback-token").Error; err != nil {
		t.Fatal(err)
	}
	if token.IsUsed {
		t.Fatal("failed raid consumed its token")
	}
}

func TestDailyRewardBalanceFailureRollsBackClaim(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "claim-rollback", 100_00)
	cleanup, err := postgres.InstallFailureTrigger("users", "UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	})
	service := event.NewService(event.NewGORMRepository(postgres.DB), value.MustMoneyFromMinor(300_00), nil)
	if err := service.RedeemDailyReward(context.Background(), "claim-rollback"); err == nil {
		t.Fatal("balance update failure was ignored")
	}
	var actor persistence.User
	if err := postgres.DB.First(&actor, "id = ?", "claim-rollback").Error; err != nil {
		t.Fatal(err)
	}
	if actor.RemainingCoin != 100_00 {
		t.Fatalf("rollback balance=%d, want 10000", actor.RemainingCoin)
	}
	var count int64
	if err := postgres.DB.Model(&persistence.DailyRewardClaim{}).Where("user_id = ?", "claim-rollback").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed redemption left %d claims", count)
	}
}
