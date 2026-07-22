package oauth

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func captureOAuthLog(t *testing.T, write func()) string {
	t.Helper()

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = &output
	gin.DefaultErrorWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
		common.LogWriterMu.Unlock()
	})

	write()
	return output.String()
}

func TestOAuthSecurityLogRedactsCredentialsResponsesAndIdentity(t *testing.T) {
	ctx := context.WithValue(context.Background(), common.RequestIdKey, "oauth-request-visible")
	output := captureOAuthLog(t, func() {
		logOAuthSecurityEvent(ctx, oauthLogWarn, "oidc", "token_exchange_failed", oauthSecurityFields{
			AuthorizationCode: "authorization-code-secret",
			ResponseBody:      []byte(`{"access_token":"access-token-secret"}`),
			Subject:           "subject-secret",
			Username:          "private-user",
			Email:             "private@example.com",
			Endpoint:          "https://identity.example.com/token?client_secret=query-secret",
			RedirectURI:       "https://geiliapi.com/oauth/oidc?code=redirect-secret",
			StatusCode:        401,
			Scope:             "openid email profile",
			Err:               errors.New("Bearer error-token-secret"),
		})
	})

	for _, secret := range []string{
		"authorization-code-secret",
		"access-token-secret",
		"subject-secret",
		"private-user",
		"private@example.com",
		"query-secret",
		"redirect-secret",
		"error-token-secret",
	} {
		require.NotContains(t, output, secret)
	}

	for _, expected := range []string{
		"oauth-request-visible",
		`"component":"authentication"`,
		`"protocol":"oauth"`,
		`"provider":"oidc"`,
		`"event":"token_exchange_failed"`,
		`"authorization_code_present":true`,
		`"authorization_code_sha256":"`,
		`"response_bytes":`,
		`"response_sha256":"`,
		`"subject_sha256":"`,
		`"username_sha256":"`,
		`"email_sha256":"`,
		`"endpoint":"https://identity.example.com/token"`,
		`"redirect_uri":"https://geiliapi.com/oauth/oidc"`,
		`"scope_count":3`,
		`"status_code":401`,
		`"error_sha256":"`,
	} {
		require.Contains(t, output, expected)
	}
}

func TestOAuthProvidersCannotBypassSecurityLogger(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "security_log.go" {
			continue
		}
		path := filepath.Join(".", name)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, body, 0)
		require.NoError(t, err)
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "logger" && strings.HasPrefix(selector.Sel.Name, "Log") {
				t.Errorf("%s directly calls logger.%s; use logOAuthSecurityEvent", name, selector.Sel.Name)
			}
			return true
		})
	}
}
