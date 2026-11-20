package testcolor

import (
	"math"
	"testing"
)

func TestContrast(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"16", "231", 21},  // чёрный и белый
		{"196", "60", 1.5}, // красный на прежнем выделении — из ревью
		{"231", "136", 3.3},
		{"244", "244", 1},
	} {
		if got := Contrast(c.a, c.b); math.Abs(got-c.want) > 0.06 {
			t.Errorf("Contrast(%s, %s) = %.2f, want %.2f", c.a, c.b, got, c.want)
		}
	}
	if RGB("17") != [3]int{0, 0, 95} || RGB("232") != [3]int{8, 8, 8} {
		t.Errorf("RGB: %v %v", RGB("17"), RGB("232"))
	}
}
