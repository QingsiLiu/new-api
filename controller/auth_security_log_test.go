package controller

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

func TestAuthSecurityLogRedactsCredentialsAndIdentity(t *testing.T) {
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

	ctx := context.WithValue(context.Background(), common.RequestIdKey, "auth-request-visible")
	logAuthSecurityEvent(ctx, authLogError, "password_reset_send_failed", authSecurityFields{
		Provider:   "password",
		Method:     "password_reset",
		Username:   "private-user",
		Email:      "private@example.com",
		Credential: "reset-token-secret",
		Err:        errors.New("smtp token secret"),
	})
	outputString := output.String()
	for _, secret := range []string{"private-user", "private@example.com", "reset-token-secret", "smtp token secret"} {
		require.NotContains(t, outputString, secret)
	}
	for _, expected := range []string{
		"auth-request-visible",
		`"component":"authentication"`,
		`"event":"password_reset_send_failed"`,
		`"provider":"password"`,
		`"method":"password_reset"`,
		`"username_sha256":"`,
		`"email_sha256":"`,
		`"credential_present":true`,
		`"credential_sha256":"`,
		`"error_sha256":"`,
	} {
		require.Contains(t, outputString, expected)
	}
}

func TestAuthLogsDoNotEmbedRawCredentials(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "security_log.go") {
			continue
		}
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
			if !ok || (identifier.Name != "logger" && identifier.Name != "common") {
				return true
			}
			if identifier.Name == "logger" && !strings.HasPrefix(selector.Sel.Name, "Log") {
				return true
			}
			if identifier.Name == "common" && selector.Sel.Name != "SysLog" && selector.Sel.Name != "SysError" {
				return true
			}
			start := fileSet.Position(call.Pos()).Offset
			end := fileSet.Position(call.End()).Offset
			logCall := string(body[start:end])
			for _, fragment := range []string{
				"username", "email", "access_token", "refresh_token", "Authorization", "cookie", "req.Key", "legacy_id=",
			} {
				if strings.Contains(logCall, fragment) {
					t.Errorf("%s embeds sensitive auth fragment %q; use logAuthSecurityEvent", path, fragment)
				}
			}
			return true
		})
	}
}
