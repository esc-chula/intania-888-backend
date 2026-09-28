package bill

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// HeadResponse is the bill HTTP response.
type HeadResponse struct {
	ID        string          `json:"id"`
	Total     value.Money     `json:"total" swaggertype:"string" example:"100.00"`
	UserID    string          `json:"user_id"`
	Status    string          `json:"status"`
	Payout    *value.Money    `json:"payout" swaggertype:"string"`
	SettledAt *time.Time      `json:"settled_at"`
	VoidedAt  *time.Time      `json:"voided_at"`
	Lines     []*LineResponse `json:"lines"`
} // @name model.BillHeadDto

// LineResponse is the bill HTTP response.
type LineResponse struct {
	BillID    string         `json:"bill_id"`
	MatchID   string         `json:"match_id"`
	Rate      value.Rate     `json:"rate" swaggertype:"number"`
	BettingOn string         `json:"betting_on"`
	Match     match.Response `json:"match"`
} // @name model.BillLineDto

// CreateBillRequest is the bill HTTP request.
type CreateBillRequest struct {
	Total value.Money             `json:"total" swaggertype:"string" example:"100.00" validate:"required"`
	Lines []CreateBillLineRequest `json:"lines" validate:"required,min=1,dive"`
} // @name model.CreateBillRequest

// CreateBillLineRequest is the bill HTTP request.
type CreateBillLineRequest struct {
	MatchID   string `json:"match_id" validate:"required"`
	BettingOn string `json:"betting_on" validate:"required"`
} // @name model.CreateBillLineRequest

// VoidBillRequest is the bill HTTP request.
type VoidBillRequest struct {
	Reason string `json:"reason" validate:"required"`
} // @name model.VoidBillRequest
