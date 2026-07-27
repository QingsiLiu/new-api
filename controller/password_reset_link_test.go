package controller

import (
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

func TestPasswordResetLinkUsesConfiguredCustomerPage(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "false")
	t.Setenv("GEILI_PASSWORD_RESET_URL", "https://geiliapi.com/user/reset")

	got := passwordResetLink("user@example.com", "a+b")
	want := "https://geiliapi.com/user/reset?email=user%40example.com&token=a%2Bb"
	if got != want {
		t.Fatalf("passwordResetLink() = %q, want %q", got, want)
	}
}

func TestPasswordResetLinkFallsBackToTrimmedServerAddress(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "false")
	t.Setenv("GEILI_PASSWORD_RESET_URL", "")
	previous := system_setting.ServerAddress
	system_setting.ServerAddress = "https://all.geiliapi.com/"
	t.Cleanup(func() { system_setting.ServerAddress = previous })

	got := passwordResetLink("user@example.com", "token")
	want := "https://all.geiliapi.com/user/reset?email=user%40example.com&token=token"
	if got != want {
		t.Fatalf("passwordResetLink() = %q, want %q", got, want)
	}
}

func TestPasswordResetLinkRejectsInvalidConfiguredURL(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "false")
	previous := system_setting.ServerAddress
	system_setting.ServerAddress = "https://all.geiliapi.com/"
	t.Cleanup(func() { system_setting.ServerAddress = previous })

	for _, configured := range []string{"javascript:alert(1)", "https:///missing-host", "/relative/reset"} {
		t.Run(configured, func(t *testing.T) {
			t.Setenv("GEILI_PASSWORD_RESET_URL", configured)
			got := passwordResetLink("user@example.com", "token")
			want := "https://all.geiliapi.com/user/reset?email=user%40example.com&token=token"
			if got != want {
				t.Fatalf("passwordResetLink() = %q, want safe fallback %q", got, want)
			}
		})
	}
}

func TestPasswordResetLinkPreservesExistingQueryOnce(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "false")
	t.Setenv("GEILI_PASSWORD_RESET_URL", "https://geiliapi.com/user/reset?locale=en&email=stale&token=stale")

	got := passwordResetLink("user@example.com", "a+b")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse passwordResetLink(): %v", err)
	}
	query := parsed.Query()
	if query.Get("locale") != "en" {
		t.Fatalf("locale query = %q, want en", query.Get("locale"))
	}
	if values := query["email"]; len(values) != 1 || values[0] != "user@example.com" {
		t.Fatalf("email query = %#v, want one normalized value", values)
	}
	if values := query["token"]; len(values) != 1 || values[0] != "a+b" {
		t.Fatalf("token query = %#v, want one escaped value", values)
	}
}

func TestPasswordResetLinkUsesFragmentTicketWithoutEmailWhenV2Enabled(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "true")
	t.Setenv("GEILI_PASSWORD_RESET_URL", "https://geiliapi.com/user/reset?locale=en&email=stale&token=stale#old")

	got := passwordResetLink("user@example.com", "opaque_ticket_123")
	want := "https://geiliapi.com/user/reset?locale=en#ticket=opaque_ticket_123"
	if got != want {
		t.Fatalf("passwordResetLink() = %q, want %q", got, want)
	}
	if strings.Contains(got, "user@example.com") || strings.Contains(got, "email=") || strings.Contains(got, "token=") {
		t.Fatalf("v2 reset link contains legacy credentials: %q", got)
	}
}
