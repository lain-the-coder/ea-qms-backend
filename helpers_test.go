package main

import (
	"testing"
	"time"
)

func TestBusinessDaysFrom(t *testing.T) {
	// Reference week: 2026-08-03 is a Monday.
	mon := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	tue := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	wed := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	thu := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	fri := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	sat := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	sun := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		start time.Time
		n     int
		want  time.Time
	}{
		{"monday plus 1 is tuesday", mon, 1, tue},
		{"monday plus 2 is wednesday", mon, 2, wed},
		{"zero days returns the start unchanged", mon, 0, mon},

		// weekend crossings — the cases most likely to be wrong
		{"friday plus 1 skips the weekend to monday", fri, 1,
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)},
		{"friday plus 2 is tuesday", fri, 2,
			time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)},
		{"thursday plus 2 crosses one weekend to monday", thu, 2,
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)},

		// weekend STARTS — a user saving on a Saturday is normal
		{"saturday plus 1 is monday", sat, 1,
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)},
		{"sunday plus 1 is monday", sun, 1,
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)},

		// the two rules this exists for
		{"monday plus 2 business days (proposed date rule)", mon, 2, wed},
		{"monday plus 10 business days is two weeks later, same weekday", mon, 10,
			time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)},
		{"tuesday plus 10 business days", tue, 10,
			time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)},

		// a longer span crossing several weekends
		{"monday plus 20 business days", mon, 20,
			time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := businessDaysFrom(tt.start, tt.n)
			if !got.Equal(tt.want) {
				t.Errorf("businessDaysFrom(%s, %d) = %s (%s), want %s (%s)",
					tt.start.Format("2006-01-02 Mon"), tt.n,
					got.Format("2006-01-02"), got.Weekday(),
					tt.want.Format("2006-01-02"), tt.want.Weekday())
			}
		})
	}
}

func TestBusinessDaysFromNeverReturnsWeekend(t *testing.T) {
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	for n := 1; n <= 40; n++ {
		got := businessDaysFrom(start, n)
		if got.Weekday() == time.Saturday || got.Weekday() == time.Sunday {
			t.Errorf("n=%d produced %s, a %s", n, got.Format("2006-01-02"), got.Weekday())
		}
	}
}
