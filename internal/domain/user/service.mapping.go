package user

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

var zeroTime time.Time

func profileFromUser(user *identity.User) *identity.Profile {
	return &identity.Profile{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		NickName:      user.NickName,
		RoleID:        user.RoleID,
		GroupID:       user.GroupID,
		RemainingCoin: value.MustMoneyFromMinor(user.RemainingCoin),
		CreatedAt:     user.CreatedAt,
	}
}
