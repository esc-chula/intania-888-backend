package integration_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/bill"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestMoneyDTOContractRejectsNumbersAndEmitsStrings(t *testing.T) {
	var request bill.CreateBillRequest

	if err := json.Unmarshal([]byte(`{"total":100,"lines":[{"match_id":"m","betting_on":"A"}]}`), &request); err == nil {
		t.Fatal("numeric bill total must be rejected")
	}

	if err := json.Unmarshal([]byte(`{"total":"100.00","lines":[{"match_id":"m","betting_on":"A"}]}`), &request); err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(httpidentity.ProfileResponse{RemainingCoin: value.MustMoneyFromMinor(888_88)})

	if err != nil {
		t.Fatal(err)
	}

	if string(b) == "" || !strings.Contains(string(b), `"remaining_coin":"888.88"`) {
		t.Fatalf("money response was not canonical: %s", b)
	}
}
