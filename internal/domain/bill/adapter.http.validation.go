package bill

import (
	"strings"
)

// ValidateRequest validates the HTTP request fields.
func (r CreateBillRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if r.Total.IsZero() {
		details["total"] = "must be greater than zero"
	}
	if len(r.Lines) == 0 {
		details["lines"] = "must contain at least one selection"
	}
	for _, line := range r.Lines {
		if strings.TrimSpace(line.MatchID) == "" {
			details["lines"] = "each selection must include a match_id"
			break
		}
		if strings.TrimSpace(line.BettingOn) == "" {
			details["lines"] = "each selection must include a betting_on value"
			break
		}
	}
	return details
}

// ValidateRequest validates the HTTP request fields.
func (r VoidBillRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Reason) == "" {
		return map[string]string{"reason": "is required"}
	}
	return nil
}
