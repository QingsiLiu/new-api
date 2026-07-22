package common

import (
	"bytes"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestRedisGetDelConsumesValueAtomically(t *testing.T) {
	server := miniredis.RunT(t)
	previous := RDB
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = RDB.Close()
		RDB = previous
	})

	require.NoError(t, RedisSet("sso-ticket:test", "metadata", 0))
	value, err := RedisGetDel("sso-ticket:test")
	require.NoError(t, err)
	require.Equal(t, "metadata", value)
	_, err = RedisGetDel("sso-ticket:test")
	require.ErrorIs(t, err, redis.Nil)
}

func TestRedisDebugLogsDoNotExposeOneTimeTicketOrPayload(t *testing.T) {
	server := miniredis.RunT(t)
	previousRDB := RDB
	previousDebug := DebugEnabled
	previousWriter := gin.DefaultWriter
	var output bytes.Buffer
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	DebugEnabled = true
	gin.DefaultWriter = &output
	t.Cleanup(func() {
		_ = RDB.Close()
		RDB = previousRDB
		DebugEnabled = previousDebug
		gin.DefaultWriter = previousWriter
	})

	const key = "sso_ticket_v2:raw-ticket-must-not-be-logged"
	const value = `{"user_id":123,"nonce":"raw-nonce-must-not-be-logged"}`
	require.NoError(t, RedisSet(key, value, 0))
	_, err := RedisGetDel(key)
	require.NoError(t, err)
	require.NotContains(t, output.String(), "raw-ticket-must-not-be-logged")
	require.NotContains(t, output.String(), "raw-nonce-must-not-be-logged")
	require.Contains(t, output.String(), "key_hash=")
}

func TestRedisGetDelAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	server := miniredis.RunT(t)
	previous := RDB
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = RDB.Close()
		RDB = previous
	})
	require.NoError(t, RedisSet("sso-ticket:concurrent", "metadata", 0))

	var wg sync.WaitGroup
	var mu sync.Mutex
	consumed := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if value, err := RedisGetDel("sso-ticket:concurrent"); err == nil && value == "metadata" {
				mu.Lock()
				consumed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, consumed)
}
