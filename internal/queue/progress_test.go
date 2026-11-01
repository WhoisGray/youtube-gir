package queue

import "testing"

func TestAdvancePercent(t *testing.T) {
	cases := []struct {
		shown, raw, want float64
	}{
		{0, 40, 40},
		{40, 80, 80},
		{80, 100, 99},
		{99, 3, 99},
		{40, 39, 40},
		{10, -5, 10},
		{0, 0, 0},
		{0, -1, 0},
	}
	for _, tc := range cases {
		if got := advancePercent(tc.shown, tc.raw); got != tc.want {
			t.Fatalf("advancePercent(%v, %v) = %v, want %v", tc.shown, tc.raw, got, tc.want)
		}
	}
}
