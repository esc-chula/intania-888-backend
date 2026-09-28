package event

import (
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// SetDailyRewardRequest contains an explicitly supplied daily override amount.
type SetDailyRewardRequest struct {
	Amount    value.Money `json:"amount" swaggertype:"string" validate:"required"`
	amountSet bool
} // @name model.SetDailyRewardRequest

// UnmarshalJSON rejects unknown fields and preserves zero versus missing amount.
func (r *SetDailyRewardRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Amount *value.Money `json:"amount"`
	}
	if err := apierror.DecodeKnownObject(data, &wire, "amount"); err != nil {
		return err
	}
	r.amountSet = wire.Amount != nil
	r.Amount = value.Money{}
	if wire.Amount != nil {
		r.Amount = *wire.Amount
	}
	return nil
}

// ValidateRequest requires an explicit nonnegative money value.
func (r SetDailyRewardRequest) ValidateRequest() map[string]string {
	if !r.amountSet {
		return map[string]string{"amount": "is required"}
	}
	if r.Amount.MinorUnits() < 0 {
		return map[string]string{"amount": "must be zero or greater"}
	}
	return nil
}

// DailyRewardScheduleItemResponse is one dated reward amount.
type DailyRewardScheduleItemResponse struct {
	Date   string      `json:"date"`
	Amount value.Money `json:"amount" swaggertype:"string"`
} // @name model.DailyRewardScheduleItem

// DailyRewardScheduleResponse contains the configured default and overrides.
type DailyRewardScheduleResponse struct {
	DefaultAmount value.Money                       `json:"default_amount" swaggertype:"string"`
	Overrides     []DailyRewardScheduleItemResponse `json:"overrides"`
} // @name model.DailyRewardScheduleResponse

// StealTokenResponse is the token issued by an alien slot result.
type StealTokenResponse struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	VictimCount int       `json:"victim_count"`
	Message     string    `json:"message"`
} // @name model.StealTokenDto

// CandidatePreviewResponse identifies a candidate without revealing its balance.
type CandidatePreviewResponse struct {
	Index   int     `json:"index"`
	Name    string  `json:"name"`
	RoleID  string  `json:"role_id"`
	GroupID *string `json:"group_id"`
} // @name model.CandidatePreviewDto

// SpinResponse preserves optional token and candidate fields from the slot API.
type SpinResponse struct {
	Slots      []string                   `json:"slots"`
	Reward     value.Money                `json:"reward" swaggertype:"string"`
	StealToken *StealTokenResponse        `json:"stealToken,omitempty"`
	Candidates []CandidatePreviewResponse `json:"candidates,omitempty"`
}

// UseStealTokenRequest selects one of the stored token candidates.
type UseStealTokenRequest struct {
	Token          string `json:"token" validate:"required"`
	VictimIndex    int    `json:"victim_index" validate:"required,min=0"`
	victimIndexSet bool
} // @name model.UseStealTokenRequestDto

// UnmarshalJSON distinguishes index zero from a missing victim index.
func (r *UseStealTokenRequest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Token       string `json:"token"`
		VictimIndex *int   `json:"victim_index"`
	}
	if err := apierror.DecodeKnownObject(data, &wire, "token", "victim_index"); err != nil {
		return err
	}
	r.Token = wire.Token
	r.VictimIndex = 0
	r.victimIndexSet = wire.VictimIndex != nil
	if wire.VictimIndex != nil {
		r.VictimIndex = *wire.VictimIndex
	}
	return nil
}

// ValidateRequest checks a nonempty token and explicitly supplied nonnegative index.
func (r UseStealTokenRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.Token) == "" {
		details["token"] = "is required"
	}
	if !r.victimIndexSet || r.VictimIndex < 0 {
		details["victim_index"] = "must be zero or greater"
	}
	return details
}

// VictimDetailResponse contains one candidate's pre-raid balance and credited raid amount.
type VictimDetailResponse struct {
	Index         int         `json:"index"`
	UserID        string      `json:"user_id"`
	Name          string      `json:"name"`
	RoleID        string      `json:"role_id"`
	GroupID       *string     `json:"group_id"`
	BalanceBefore value.Money `json:"balance_before" swaggertype:"string"`
	AmountStolen  value.Money `json:"amount_stolen" swaggertype:"string"`
	WasChosen     bool        `json:"was_chosen"`
} // @name model.VictimDetailDto

// UseStealTokenResponse contains the committed raid result.
type UseStealTokenResponse struct {
	TotalStolen      value.Money            `json:"total_stolen" swaggertype:"string"`
	RaiderNewBalance value.Money            `json:"raider_new_balance" swaggertype:"string"`
	AllCandidates    []VictimDetailResponse `json:"all_candidates"`
	Message          string                 `json:"message"`
} // @name model.UseStealTokenResponseDto
