package policy

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
)

func TestPolicySnapshotPreservesExistingCacheJSON(t *testing.T) {
	raw := newFakeCache()
	ctx := context.Background()
	expiry := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	input := []*AccessPolicy{{
		ID:            "id",
		Kind:          KindBlacklist,
		PrincipalType: PrincipalGoogleSubject,
		Principal:     "subject",
		Reason:        "reason",
		Enabled:       true,
		ExpiresAt:     &expiry,
		CreatedAt:     time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
	}}
	adapter := NewRedisSnapshotCache(raw)
	if err := adapter.Store(ctx, input); err != nil {
		t.Fatal(err)
	}
	encoded := raw.values[security.ToPolicySnapshotCacheKey()]
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &records); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "kind", "principal_type", "principal", "reason", "enabled", "expires_at", "created_at", "updated_at"} {
		if _, ok := records[0][key]; !ok {
			t.Fatalf("missing legacy cache field %q: %s", key, encoded)
		}
	}
	if len(records[0]) != 9 {
		t.Fatalf("unexpected cache fields: %s", encoded)
	}
	got, err := adapter.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, input) {
		t.Fatalf("snapshot roundtrip=%+v, want %+v", got, input)
	}
}

func TestBootstrapFilePreservesWireFormat(t *testing.T) {
	var input BootstrapFile
	decoder := json.NewDecoder(strings.NewReader(`{"entries":[{"kind":"allowlist","principal_type":"email","principal":"partner@example.com","reason":"approved","expires_at":null}]}`))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		t.Fatal(err)
	}
	if len(input.Entries) != 1 {
		t.Fatalf("entries=%v", input.Entries)
	}
	normalized, err := ValidateCreateInput(input.Entries[0].Input())
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Kind != KindAllowlist || normalized.PrincipalType != PrincipalEmail || normalized.ExpiresAt != nil {
		t.Fatalf("bootstrap input=%+v", normalized)
	}
}
