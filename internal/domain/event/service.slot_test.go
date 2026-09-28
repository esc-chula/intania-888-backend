package event

import (
	"context"
	"math"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestSlotOverflowRetainsExistingZeroReward(t *testing.T) {
	repo := newMemoryEventRepository()
	repo.users["actor"] = identity.User{ID: "actor", RemainingCoin: math.MaxInt64}
	service := NewService(repo, value.Money{}, nil)
	service.drawSlot = func(identity.Profile) string { return "💰" }
	result, err := service.SpinSlotMachine(context.Background(), identity.Profile{ID: "actor"}, value.MustMoneyFromMinor(math.MaxInt64))
	if err != nil || !result.Reward.IsZero() || repo.users["actor"].RemainingCoin != 0 {
		t.Fatalf("legacy overflow outcome changed: result=%+v, balance=%d, error=%v", result, repo.users["actor"].RemainingCoin, err)
	}
}
