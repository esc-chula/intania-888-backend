package bill

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestBillHTTPMappingPreservesNullableFieldsAndNestedMatch(t *testing.T) {
	a, b := "A", "B"
	result := &Result{
		ID:        "bill",
		UserID:    "user",
		Total:     value.MustMoneyFromMinor(10000),
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Lines: []Line{{
			BillID:    "bill",
			MatchID:   "match",
			BettingOn: "A",
			Rate:      value.MustRateFromMicro(2000000),
			Match:     match.Snapshot{ID: "match", TeamAID: &a, TeamBID: &b},
		}},
	}
	encoded, err := json.Marshal(billResultDTO(result))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != 8 || object["total"] != "100.00" {
		t.Fatalf("bill wire fields = %s", encoded)
	}
	for _, key := range []string{"payout", "settled_at", "voided_at"} {
		field, present := object[key]
		if !present || field != nil {
			t.Fatalf("nullable %s = %v, present %v", key, field, present)
		}
	}
	lines := object["lines"].([]any)
	line := lines[0].(map[string]any)
	nested := line["match"].(map[string]any)
	if line["rate"] != float64(2) || nested["team_a_rate"] != float64(0) || nested["team_b_rate"] != float64(0) {
		t.Fatalf("rate wire fields = %s", encoded)
	}
	if nested["team_a"] != "A" || nested["team_b"] != "B" || nested["winner"] != "" || nested["team_a_score"] != nil {
		t.Fatalf("match wire fields = %s", encoded)
	}
}

func TestBillHTTPMappingKeepsEmptyLinesAsArray(t *testing.T) {
	encoded, err := json.Marshal(billResultDTO(&Result{}))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if string(object["lines"]) != "[]" {
		t.Fatalf("empty lines = %s; want []", object["lines"])
	}
}
