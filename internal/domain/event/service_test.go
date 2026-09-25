package event

import (
	"testing"
	"time"
)

func TestDailyRewardDateUsesBangkokCalendar(t *testing.T) {
	s := &eventService{
		now: func() time.Time {
			return time.Date(2026, time.September, 24, 17, 0, 0, 0, time.UTC)
		},
	}

	date := s.now().In(bangkokLocation).Format("02-01-2006")
	if date != "25-09-2026" {
		t.Fatalf("daily reward date = %q, want 25-09-2026", date)
	}
}
