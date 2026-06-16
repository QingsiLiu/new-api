package controller

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAsyncImageGenerationWrapsSynchronousOpenAIChannel(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"model":"gpt-image-2"`)
		require.Contains(t, string(body), `"prompt":"draw a studio"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
		upstreamCalled <- struct{}{}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"image",
		"action":"generate",
		"model":"gpt-image-2",
		"input":{"prompt":"draw a studio"},
		"parameters":{"quality":"high","size":"1024x1024","n":1}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, "running", created.Status)
	require.NotEmpty(t, created.ID)
	require.Empty(t, created.Outputs)
	require.Eventually(t, func() bool {
		select {
		case <-upstreamCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	fetchRecorder := httptest.NewRecorder()
	fetchRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	fetchRequest.Header.Set("Authorization", "Bearer "+token)
	require.Eventually(t, func() bool {
		fetchRecorder = httptest.NewRecorder()
		engine.ServeHTTP(fetchRecorder, fetchRequest)
		return fetchRecorder.Code == http.StatusOK && strings.Contains(fetchRecorder.Body.String(), `"status":"succeeded"`)
	}, 2*time.Second, 20*time.Millisecond, fetchRecorder.Body.String())

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "img-bytes", contentRecorder.Body.String())
}

func TestAsyncImageEditForwardsReferenceFiles(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/edits", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(2<<20))
		require.Equal(t, "gpt-image-2", r.FormValue("model"))
		require.Equal(t, "edit this", r.FormValue("prompt"))
		require.Len(t, r.MultipartForm.File["image"], 1)
		file, err := r.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "reference-image", string(content))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ZWRpdC1ieXRlcw=="}]}`))
		upstreamCalled <- struct{}{}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("kind", "image"))
	require.NoError(t, writer.WriteField("action", "edit"))
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("reference-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, "running", created.Status)
	require.Eventually(t, func() bool {
		select {
		case <-upstreamCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		statusRecorder := httptest.NewRecorder()
		statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
		statusRequest.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(statusRecorder, statusRequest)
		return statusRecorder.Code == http.StatusOK && strings.Contains(statusRecorder.Body.String(), `"status":"succeeded"`)
	}, 2*time.Second, 20*time.Millisecond)

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "edit-bytes", contentRecorder.Body.String())
}

func setupAsyncTaskRouterTest(t *testing.T, upstreamURL string, modelName string) (*gin.Engine, string) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Task{}))
	token := "sk-cavas"
	require.NoError(t, db.Create(&model.User{
		Id:       2001,
		Username: "cavas-service",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Quota:    1000000,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             3001,
		UserId:         2001,
		Key:            "cavas",
		Name:           "cavas",
		Status:         common.TokenStatusEnabled,
		RemainQuota:    1000000,
		UnlimitedQuota: true,
		UsedQuota:      0,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:      4001,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-upstream",
		Status:  common.ChannelStatusEnabled,
		Name:    "CPA OpenAI Compatible",
		BaseURL: &upstreamURL,
		Models:  modelName,
		Group:   "default",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: 4001,
		Enabled:   true,
		Weight:    1,
	}).Error)
	model.InitChannelCache()

	engine := gin.New()
	asyncRouter := engine.Group("/v1/async")
	asyncRouter.Use(middleware.TokenAuth())
	{
		asyncRouter.POST("/tasks", CreateAsyncTask)
		asyncRouter.GET("/tasks/:id", GetAsyncTask)
		asyncRouter.POST("/tasks/:id/cancel", CancelAsyncTask)
		asyncRouter.GET("/tasks/:id/content", GetAsyncTaskContent)
	}
	return engine, token
}

func TestOpenAIModelListIncludesGPTImage2(t *testing.T) {
	require.Contains(t, openai.ModelList, "gpt-image-2")
}
