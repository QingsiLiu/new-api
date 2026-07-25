package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/oauth"
	"github.com/stretchr/testify/require"
)

func TestTrustedSignupCreditOAuthProviders(t *testing.T) {
	require.True(t, isTrustedSignupCreditOAuthProvider(&oauth.GitHubProvider{}))
	require.True(t, isTrustedSignupCreditOAuthProvider(&oauth.DiscordProvider{}))
	require.True(t, isTrustedSignupCreditOAuthProvider(&oauth.LinuxDOProvider{}))

	t.Setenv("GEILI_CREDITS_TRUST_OIDC", "false")
	require.False(t, isTrustedSignupCreditOAuthProvider(&oauth.OIDCProvider{}))
	t.Setenv("GEILI_CREDITS_TRUST_OIDC", "true")
	require.True(t, isTrustedSignupCreditOAuthProvider(&oauth.OIDCProvider{}))
	require.False(t, isTrustedSignupCreditOAuthProvider(nil))
}
