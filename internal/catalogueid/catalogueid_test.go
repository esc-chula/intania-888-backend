package catalogueid

import (
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	for _, id := range []string{"A", "INDOOR_STADIUM_1", "venue-1", "new_ID-1", strings.Repeat("a", 100)} {
		if !Valid(id) {
			t.Errorf("Valid(%q) = false", id)
		}
	}

	for _, id := range []string{"", "bad.id", " S ", "a/b", "a?b", "a%b", "a\x00b", "\t", "\x7f", "\x80", "สถานที่", strings.Repeat("a", 101)} {
		if Valid(id) {
			t.Errorf("Valid(%q) = true", id)
		}
	}
}
