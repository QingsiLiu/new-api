package common

import "testing"

func TestThemeDefaultsToDefaultFrontend(t *testing.T) {
	if got := GetTheme(); got != "default" {
		t.Fatalf("GetTheme() = %q, want default", got)
	}
}

func TestThemeAwarePathRewritesConsolePathsForDefaultFrontend(t *testing.T) {
	SetTheme("default")
	t.Cleanup(func() {
		SetTheme("default")
	})

	cases := map[string]string{
		"/console/topup":    "/wallet",
		"/console/log":      "/usage-logs",
		"/console/personal": "/profile",
		"/console/other":    "/console/other",
	}
	for input, want := range cases {
		if got := ThemeAwarePath(input); got != want {
			t.Fatalf("ThemeAwarePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestThemeAwarePathKeepsClassicConsolePaths(t *testing.T) {
	SetTheme("classic")
	t.Cleanup(func() {
		SetTheme("default")
	})

	if got := ThemeAwarePath("/console/topup"); got != "/console/topup" {
		t.Fatalf("ThemeAwarePath classic = %q, want /console/topup", got)
	}
}
