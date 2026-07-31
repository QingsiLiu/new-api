package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateTaskPromptSafetyRejectsVideoAndSunoPrompts(t *testing.T) {
	previousEnabled := setting.CheckSensitiveEnabled
	previousPromptEnabled := setting.CheckSensitiveOnPromptEnabled
	previousWords := append([]string(nil), setting.SensitiveWords...)
	setting.CheckSensitiveEnabled = true
	setting.CheckSensitiveOnPromptEnabled = true
	setting.SensitiveWords = []string{"blocked task prompt"}
	t.Cleanup(func() {
		setting.CheckSensitiveEnabled = previousEnabled
		setting.CheckSensitiveOnPromptEnabled = previousPromptEnabled
		setting.SensitiveWords = previousWords
	})

	for name, request := range map[string]any{
		"video": relaycommon.TaskSubmitReq{Prompt: "make a blocked task prompt"},
		"suno":  &dto.SunoSubmitReq{GptDescriptionPrompt: "a blocked task prompt song"},
	} {
		t.Run(name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Set("task_request", request)

			taskErr := validateTaskPromptSafety(context)

			require.NotNil(t, taskErr)
			require.Equal(t, "sensitive_words_detected", taskErr.Code)
			require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		})
	}
}
