package models

import "testing"

func TestThemeRegistry(t *testing.T) {
	for _, id := range []string{ThemeLight, ThemeDark, ThemeGlassLight, ThemeGlassDark} {
		if !IsValidTheme(id) {
			t.Errorf("valid theme rejected: %s", id)
		}
	}
	for _, id := range []string{"", "unknown", "glass", "glass-orange"} {
		if IsValidTheme(id) {
			t.Errorf("unknown theme accepted: %s", id)
		}
	}
	if DefaultTheme != ThemeLight {
		t.Fatal("default theme changed")
	}
}
