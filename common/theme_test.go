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

func TestSetThemeIgnoresRetiredClassicFrontend(t *testing.T) {
	SetTheme("default")
	SetTheme("classic")
	t.Cleanup(func() {
		SetTheme("default")
	})

	if got := GetTheme(); got != "default" {
		t.Fatalf("GetTheme() = %q, want default", got)
	}
	if got := ThemeAwarePath("/console/topup"); got != "/wallet" {
		t.Fatalf("ThemeAwarePath retired classic = %q, want /wallet", got)
	}
}
