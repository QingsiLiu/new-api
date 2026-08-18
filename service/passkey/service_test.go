package passkey

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestPasskeyProfileSupportsNewLegacyAndStagingHosts(t *testing.T) {
	testCases := []struct {
		host   string
		rpID   string
		origin string
	}{
		{host: "auapi.ai", rpID: "auapi.ai", origin: "https://auapi.ai"},
		{host: "admin.auapi.ai:443", rpID: "auapi.ai", origin: "https://admin.auapi.ai"},
		{host: "next.auapi.ai", rpID: "auapi.ai", origin: "https://next.auapi.ai"},
		{host: "geiliapi.com", rpID: "geiliapi.com", origin: "https://geiliapi.com"},
		{host: "admin.geiliapi.com", rpID: "geiliapi.com", origin: "https://admin.geiliapi.com"},
		{host: "next.geiliapi.com", rpID: "geiliapi.com", origin: "https://next.geiliapi.com"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.host, func(t *testing.T) {
			request := httptest.NewRequest("GET", "https://"+testCase.host+"/api/passkey", nil)
			require.Equal(t, testCase.rpID, RPIDForRequest(request))
			require.Equal(t, testCase.origin, OriginForRequest(request))
		})
	}
}

func TestRequestPasskeyProfileUsesOnlyAllowlistedForwardedHost(t *testing.T) {
	request := httptest.NewRequest("GET", "http://relay-new-api:3000/api/passkey", nil)
	request.Header.Set("X-Forwarded-Host", "auapi.ai")
	require.Equal(t, "auapi.ai", RPIDForRequest(request))
	require.Equal(t, "https://auapi.ai", OriginForRequest(request))

	request.Header.Set("X-Forwarded-Host", "attacker.example")
	require.Empty(t, RPIDForRequest(request))
	require.Empty(t, OriginForRequest(request))

	request.Header.Set("X-Forwarded-Host", "auapi.ai, attacker.example")
	require.Empty(t, RPIDForRequest(request))
	require.Empty(t, OriginForRequest(request))
}

func TestRequestPasskeyProfileDoesNotOverrideRecognizedDirectHost(t *testing.T) {
	request := httptest.NewRequest("GET", "https://geiliapi.com/api/passkey", nil)
	request.Header.Set("X-Forwarded-Host", "auapi.ai")
	require.Equal(t, "geiliapi.com", RPIDForRequest(request))
	require.Equal(t, "https://geiliapi.com", OriginForRequest(request))
}
