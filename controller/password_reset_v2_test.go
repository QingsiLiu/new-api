package controller

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestPasswordResetTicketStoresOnlyHashAndMinimalPayloadInRedis(t *testing.T) {
	server := setupPasswordResetTicketRedis(t)
	ticket, err := issuePasswordResetTicket(" Person@Example.Invalid ")
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(ticket)
	require.NoError(t, err)
	require.Len(t, raw, passwordResetTicketBytes)

	keys := server.Keys()
	require.Equal(t, []string{passwordResetV2KeyPrefix + passwordResetTicketHash(ticket)}, keys)
	require.NotContains(t, keys[0], ticket)
	stored, err := common.RedisGet(keys[0])
	require.NoError(t, err)
	require.NotContains(t, stored, ticket)
	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(stored), &payload))
	require.ElementsMatch(t, []string{"email", "expires_at", "purpose"}, mapKeys(payload))
	require.Equal(t, "person@example.invalid", payload["email"])
	require.Equal(t, passwordResetV2Purpose, payload["purpose"])
	require.Greater(t, server.TTL(keys[0]), time.Duration(0))
	require.LessOrEqual(t, server.TTL(keys[0]), passwordResetTicketTTL())
}

func TestPasswordResetTicketAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	for _, backend := range []string{"redis", "memory"} {
		t.Run(backend, func(t *testing.T) {
			if backend == "redis" {
				setupPasswordResetTicketRedis(t)
			} else {
				setupPasswordResetTicketMemory(t)
			}
			ticket, err := issuePasswordResetTicket("concurrent@example.invalid")
			require.NoError(t, err)

			var wg sync.WaitGroup
			var mu sync.Mutex
			consumed := 0
			for i := 0; i < 24; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if email, err := consumePasswordResetTicket(ticket); err == nil && email == "concurrent@example.invalid" {
						mu.Lock()
						consumed++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			require.Equal(t, 1, consumed)
			_, err = consumePasswordResetTicket(ticket)
			require.ErrorIs(t, err, errPasswordResetTicketInvalid)
		})
	}
}

func TestPasswordResetMemoryTicketExpiresAndIsConsumed(t *testing.T) {
	setupPasswordResetTicketMemory(t)
	ticket := testPasswordResetTicket("expired-memory-ticket")
	hash := passwordResetTicketHash(ticket)
	passwordResetMemoryTickets.Lock()
	passwordResetMemoryTickets.items[hash] = passwordResetTicketPayload{
		Email: "expired@example.invalid", ExpiresAt: time.Now().Add(-time.Second).Unix(), Purpose: passwordResetV2Purpose,
	}
	passwordResetMemoryTickets.Unlock()

	_, err := consumePasswordResetTicket(ticket)
	require.ErrorIs(t, err, errPasswordResetTicketInvalid)
	_, err = consumePasswordResetTicket(ticket)
	require.ErrorIs(t, err, errPasswordResetTicketInvalid)
}

func TestResetPasswordV2BurnsTicketBeforeDatabaseFailure(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "true")
	setupPasswordResetTicketMemory(t)
	db := setupUserControllerTestDB(t)
	user := model.User{Username: "reset-v2-db-failure", Email: "db-failure@example.invalid", Password: "OriginalPassword123"}
	require.NoError(t, user.Insert(0))
	ticket, err := issuePasswordResetTicket(user.Email)
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&model.User{}))

	first := resetPasswordV2Request(ticket)
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), "invalid")
	require.Equal(t, "private, no-store", first.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", first.Header().Get("Referrer-Policy"))

	second := resetPasswordV2Request(ticket)
	require.Equal(t, first.Body.String(), second.Body.String())
}

func TestResetPasswordV2SucceedsOnceAndUpdatesStoredPassword(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "true")
	setupPasswordResetTicketMemory(t)
	setupUserControllerTestDB(t)
	user := model.User{Username: "reset-v2-success", Email: "success@example.invalid", Password: "OriginalPassword123"}
	require.NoError(t, user.Insert(0))
	ticket, err := issuePasswordResetTicket(user.Email)
	require.NoError(t, err)

	first := resetPasswordV2Request(ticket)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var response struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(first.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 12)
	require.NotContains(t, first.Body.String(), ticket)
	require.NotContains(t, first.Body.String(), user.Email)
	updated, err := model.GetUniqueUserByEmail(user.Email)
	require.NoError(t, err)
	require.True(t, common.ValidatePasswordAndHash(response.Data, updated.Password))

	second := resetPasswordV2Request(ticket)
	require.Equal(t, http.StatusOK, second.Code)
	require.Contains(t, second.Body.String(), "invalid")
	require.NotContains(t, second.Body.String(), response.Data)
}

func TestPasswordResetV2IsDisabledByDefault(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "false")
	recorder := resetPasswordV2Request(testPasswordResetTicket("disabled"))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "ticket")
}

func TestPasswordResetEmailPostUsesGenericResponseAndFragmentTicket(t *testing.T) {
	t.Setenv(passwordResetV2EnabledEnv, "true")
	t.Setenv(geiliPasswordResetURLEnv, "https://geiliapi.com/user/reset")
	setupPasswordResetTicketMemory(t)
	setupUserControllerTestDB(t)
	user := model.User{Username: "reset-v2-email", Email: "person@example.invalid", Password: "OriginalPassword123"}
	require.NoError(t, user.Insert(0))

	previousSender := sendPasswordResetMail
	var recipient string
	var content string
	sendPasswordResetMail = func(_ string, to string, body string) error {
		recipient = to
		content = body
		return nil
	}
	t.Cleanup(func() { sendPasswordResetMail = previousSender })

	existing := passwordResetEmailPostRequest(`{"email":" PERSON@example.invalid "}`)
	require.Equal(t, http.StatusOK, existing.Code, existing.Body.String())
	require.JSONEq(t, `{"success":true,"message":""}`, existing.Body.String())
	require.Equal(t, "person@example.invalid", recipient)
	require.Contains(t, content, "https://geiliapi.com/user/reset#ticket=")
	require.NotContains(t, content, "?email=")
	require.NotContains(t, content, "person@example.invalid")

	unknown := passwordResetEmailPostRequest(`{"email":"unknown@example.invalid"}`)
	require.Equal(t, existing.Body.String(), unknown.Body.String())
	require.Equal(t, "private, no-store", unknown.Header().Get("Cache-Control"))

	invalid := passwordResetEmailPostRequest(`{"email":"not-an-email"}`)
	require.Equal(t, http.StatusOK, invalid.Code)
	require.Contains(t, invalid.Body.String(), "invalid")
}

func resetPasswordV2Request(ticket string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := fmt.Sprintf(`{"ticket":%q}`, ticket)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/reset/v2", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ResetPasswordV2(ctx)
	return recorder
}

func passwordResetEmailPostRequest(body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/reset_password", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	SendPasswordResetEmailPost(ctx)
	return recorder
}

func setupPasswordResetTicketRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	previousRDB := common.RDB
	previousEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousEnabled
	})
	return server
}

func setupPasswordResetTicketMemory(t *testing.T) {
	t.Helper()
	previousEnabled := common.RedisEnabled
	common.RedisEnabled = false
	passwordResetMemoryTickets.Lock()
	passwordResetMemoryTickets.items = make(map[string]passwordResetTicketPayload)
	passwordResetMemoryTickets.Unlock()
	t.Cleanup(func() {
		common.RedisEnabled = previousEnabled
		passwordResetMemoryTickets.Lock()
		passwordResetMemoryTickets.items = make(map[string]passwordResetTicketPayload)
		passwordResetMemoryTickets.Unlock()
	})
}

func testPasswordResetTicket(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	raw := digest[:passwordResetTicketBytes]
	return base64.RawURLEncoding.EncodeToString(raw)
}
