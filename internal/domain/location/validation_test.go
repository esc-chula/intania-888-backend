package location

import (
	"strings"
	"testing"
)

func TestLocationValidation(t *testing.T) {
	for _, id := range []string{"A", "INDOOR_STADIUM_1", "venue-1", "bad.id", " S ", "\x7f"} {
		if !validID(id) {
			t.Errorf("validID(%q) = false", id)
		}
	}

	for _, id := range []string{"", "\x80", "สถานที่", strings.Repeat("a", 101)} {
		if validID(id) {
			t.Errorf("validID(%q) = true", id)
		}
	}

	if title, err := normalizeTitle("  สนามโยธา  "); err != nil || title != "สนามโยธา" {
		t.Fatalf("normalizeTitle() = %q, %v", title, err)
	}

	for _, title := range []string{"", " \t ", strings.Repeat("ก", 101)} {
		if _, err := normalizeTitle(title); err == nil {
			t.Errorf("normalizeTitle(%q) accepted an invalid title", title)
		}
	}
}
