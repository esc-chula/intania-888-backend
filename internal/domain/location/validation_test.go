package location

import (
	"strings"
	"testing"
)

func TestLocationValidation(t *testing.T) {
	if title, err := normalizeTitle("  สนามโยธา  "); err != nil || title != "สนามโยธา" {
		t.Fatalf("normalizeTitle() = %q, %v", title, err)
	}

	for _, title := range []string{"", " \t ", strings.Repeat("ก", 101)} {
		if _, err := normalizeTitle(title); err == nil {
			t.Errorf("normalizeTitle(%q) accepted an invalid title", title)
		}
	}
}
