// Пакет testcolor — контраст цветов палитры xterm-256 для тестов (WCAG 2.x).
package testcolor

import (
	"math"
	"strconv"
)

// base16 — первые 16 цветов в значениях xterm по умолчанию.
var base16 = [16][3]int{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0}, {0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

// RGB — цвет xterm-256 с номером n (строкой, как в lipgloss.AdaptiveColor).
func RGB(n string) [3]int {
	i, err := strconv.Atoi(n)
	if err != nil || i < 0 || i > 255 {
		panic("testcolor: не номер xterm-256: " + n)
	}
	switch {
	case i < 16:
		return base16[i]
	case i < 232:
		lv := [6]int{0, 95, 135, 175, 215, 255}
		i -= 16
		return [3]int{lv[i/36], lv[i/6%6], lv[i%6]}
	}
	g := 8 + 10*(i-232)
	return [3]int{g, g, g}
}

// luminance — относительная яркость по WCAG.
func luminance(c [3]int) float64 {
	lin := func(v int) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}

// Contrast — коэффициент контраста WCAG двух цветов xterm-256: от 1 до 21.
func Contrast(a, b string) float64 {
	la, lb := luminance(RGB(a)), luminance(RGB(b))
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
