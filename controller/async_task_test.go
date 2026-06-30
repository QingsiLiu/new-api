package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAsyncTaskHTTPClientDefaultsToFiveMinutes(t *testing.T) {
	if os.Getenv("ASYNC_TASK_HTTP_TIMEOUT_SECONDS") != "" {
		t.Skip("default timeout assertion requires ASYNC_TASK_HTTP_TIMEOUT_SECONDS to be unset")
	}
	require.Equal(t, 300*time.Second, asyncTaskHTTPClient.Timeout)
}

func TestAsyncTaskHTTPClientUsesConfiguredTimeoutFromEnv(t *testing.T) {
	if os.Getenv("ASYNC_TASK_HTTP_TIMEOUT_SUBPROCESS") == "1" {
		require.Equal(t, 420*time.Second, asyncTaskHTTPClient.Timeout)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "^TestAsyncTaskHTTPClientUsesConfiguredTimeoutFromEnv$")
	cmd.Env = append(os.Environ(), "ASYNC_TASK_HTTP_TIMEOUT_SUBPROCESS=1", "ASYNC_TASK_HTTP_TIMEOUT_SECONDS=420")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestAsyncImageRequestsUseConfiguredTimeoutDeadline(t *testing.T) {
	t.Setenv("ASYNC_TASK_HTTP_TIMEOUT_SECONDS", "240")
	baseURL := "https://upstream.example"
	channel := &model.Channel{Key: "sk-upstream", BaseURL: &baseURL}
	requestsSeen := 0
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestsSeen++
		assertAsyncRequestDeadline(t, req, 240*time.Second)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`)),
			Request:    req,
		}, nil
	})})
	defer restoreClient()

	_, err := executeAsyncImageGeneration(context.Background(), channel, asyncTaskRequest{
		Kind:       asyncTaskKindImage,
		Action:     asyncTaskActionGenerate,
		Model:      "gpt-image-2",
		Input:      asyncTaskInput{Prompt: "draw a studio"},
		Parameters: map[string]interface{}{"n": 1},
	})
	require.NoError(t, err)

	editExecution := newAsyncEditExecutionForTimeoutTest(t)
	_, err = executeAsyncImageEdit(context.Background(), channel, editExecution)
	require.NoError(t, err)
	require.Equal(t, 2, requestsSeen)
}

func TestAsyncImageGenerationWrapsSynchronousOpenAIChannel(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"model":"gpt-image-2"`)
		require.Contains(t, string(body), `"prompt":"draw a studio"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
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
	require.Equal(t, "queued", created.Status)
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
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("edit-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/edits", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(2<<20))
		require.Equal(t, "gpt-image-2", r.FormValue("model"))
		require.Equal(t, "edit this", r.FormValue("prompt"))
		require.Len(t, r.MultipartForm.File["image"], 1)
		require.Equal(t, "image/png", r.MultipartForm.File["image"][0].Header.Get("Content-Type"))
		file, err := r.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "reference-image", string(content))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/edit.png"}]}`))
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
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="image"; filename="reference.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
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
	require.Equal(t, "queued", created.Status)
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

func TestAsyncGeminiImageEditUsesGenerateContentWithInlineReferences(t *testing.T) {
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/v1beta/models/gemini-2.5-flash-image:generateContent", req.URL.Path)
		require.Equal(t, "sk-upstream", req.Header.Get("x-goog-api-key"))
		require.Empty(t, req.Header.Get("Authorization"))
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		var payload dto.GeminiChatRequest
		require.NoError(t, common.Unmarshal(body, &payload))
		require.Len(t, payload.Contents, 1)
		require.Equal(t, "user", payload.Contents[0].Role)
		require.Len(t, payload.Contents[0].Parts, 2)
		require.Equal(t, "edit this", payload.Contents[0].Parts[0].Text)
		require.NotNil(t, payload.Contents[0].Parts[1].InlineData)
		require.Equal(t, "image/png", payload.Contents[0].Parts[1].InlineData.MimeType)
		require.Equal(t, "cmVmZXJlbmNlLWltYWdl", payload.Contents[0].Parts[1].InlineData.Data)
		require.Equal(t, []string{"TEXT", "IMAGE"}, payload.GenerationConfig.ResponseModalities)
		require.NotNil(t, payload.GenerationConfig.CandidateCount)
		require.Equal(t, 2, *payload.GenerationConfig.CandidateCount)
		require.NotEmpty(t, payload.GenerationConfig.ImageConfig)
		var imageConfig map[string]string
		require.NoError(t, common.Unmarshal(payload.GenerationConfig.ImageConfig, &imageConfig))
		require.Equal(t, "1:1", imageConfig["aspectRatio"])
		require.Equal(t, "1K", imageConfig["imageSize"])

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"ZWRpdC1ieXRlcw=="}}]},"finishReason":"STOP","index":0}],
				"usageMetadata":{"totalTokenCount":1}
			}`)),
			Request: req,
		}, nil
	})})
	defer restoreClient()

	upstreamURL := "https://upstream.example"
	outputs, err := executeAsyncImageEdit(context.Background(), &model.Channel{
		Type:    constant.ChannelTypeGemini,
		Key:     "sk-upstream",
		BaseURL: &upstreamURL,
	}, newAsyncGeminiEditExecutionForTest(t))

	require.NoError(t, err)
	require.Len(t, outputs, 1)
	require.Equal(t, "image/png", outputs[0].MimeType)
	require.Equal(t, "ZWRpdC1ieXRlcw==", outputs[0].Content)
	require.Equal(t, len("edit-bytes"), outputs[0].Size)
}

func TestAsyncGeminiImageEditMapsCavasSizeAndQualityToImageConfig(t *testing.T) {
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		var payload dto.GeminiChatRequest
		require.NoError(t, common.Unmarshal(body, &payload))
		require.NotEmpty(t, payload.GenerationConfig.ImageConfig)
		var imageConfig map[string]string
		require.NoError(t, common.Unmarshal(payload.GenerationConfig.ImageConfig, &imageConfig))
		require.Equal(t, "2:3", imageConfig["aspectRatio"])
		require.Equal(t, "2K", imageConfig["imageSize"])

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"ZWRpdC1ieXRlcw=="}}]},"finishReason":"STOP","index":0}]
			}`)),
			Request: req,
		}, nil
	})})
	defer restoreClient()

	upstreamURL := "https://upstream.example"
	outputs, err := executeAsyncImageEdit(context.Background(), &model.Channel{
		Type:    constant.ChannelTypeGemini,
		Key:     "sk-upstream",
		BaseURL: &upstreamURL,
	}, newAsyncGeminiEditExecutionWithImageConfigForTest(t, "medium", "1360x2048"))

	require.NoError(t, err)
	require.Len(t, outputs, 1)
}

func TestAsyncGeminiAspectRatioMapsCavasPixelSizes(t *testing.T) {
	tests := map[string]string{
		"auto":      "",
		"2:3":       "2:3",
		"1024x1024": "1:1",
		"1536x1024": "3:2",
		"1024x1536": "2:3",
		"1360x1024": "4:3",
		"1024x1360": "3:4",
		"1792x1024": "16:9",
		"1024x1792": "9:16",
		"2048x2048": "1:1",
		"2048x1360": "3:2",
		"1360x2048": "2:3",
		"2048x1536": "4:3",
		"1536x2048": "3:4",
		"2048x1152": "16:9",
		"1152x2048": "9:16",
		"3840x3840": "1:1",
		"3840x2560": "3:2",
		"2560x3840": "2:3",
		"3840x2880": "4:3",
		"2880x3840": "3:4",
		"3840x2160": "16:9",
		"2160x3840": "9:16",
	}
	for size, want := range tests {
		t.Run(size, func(t *testing.T) {
			require.Equal(t, want, asyncGeminiAspectRatio(size))
		})
	}
}

func TestAsyncGeminiImageSizeMapsCavasQualities(t *testing.T) {
	tests := map[string]string{
		"low":      "1K",
		"1K":       "1K",
		"standard": "1K",
		"auto":     "1K",
		"medium":   "2K",
		"2K":       "2K",
		"hd":       "2K",
		"high":     "4K",
		"4K":       "4K",
	}
	for quality, want := range tests {
		t.Run(quality, func(t *testing.T) {
			require.Equal(t, want, asyncGeminiImageSize(quality))
		})
	}
}

func TestAsyncMultipartFileContentTypeSniffsImageWhenHeaderUnknown(t *testing.T) {
	header := &multipart.FileHeader{
		Header: textproto.MIMEHeader{"Content-Type": []string{"application/octet-stream"}},
	}
	pngBytes := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

	require.Equal(t, "image/png", asyncMultipartFileContentType(header, pngBytes))
}

func TestAsyncImageGenerationForcesURLResponseFormat(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"response_format":"url"`)
		require.NotContains(t, string(body), `"response_format":"b64_json"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
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
		"parameters":{"quality":"high","size":"1024x1024","n":1,"response_format":"b64_json"}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Eventually(t, func() bool {
		select {
		case <-upstreamCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncTaskSchedulerBoundsConcurrentExecutionsAndReportsMetrics(t *testing.T) {
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()

	started := make(chan string, 2)
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		prompt := "first"
		release := releaseFirst
		if strings.Contains(string(body), "second") {
			prompt = "second"
			release = releaseSecond
		}
		started <- prompt
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
	}))
	defer upstream.Close()
	defer close(releaseFirst)
	defer close(releaseSecond)

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	restoreScheduler := setAsyncTaskSchedulerForTest(1, 4)
	defer restoreScheduler()
	first := createAsyncTaskForTest(t, engine, token, "first", "idem-first")
	require.Equal(t, "queued", first.Status)
	require.Equal(t, "first", <-started)

	second := createAsyncTaskForTest(t, engine, token, "second", "idem-second")
	require.Equal(t, "queued", second.Status)
	require.NotEqual(t, first.ID, second.ID)

	select {
	case prompt := <-started:
		t.Fatalf("expected second task to stay queued while first is running, got upstream call for %s", prompt)
	case <-time.After(100 * time.Millisecond):
	}

	metricsRecorder := httptest.NewRecorder()
	metricsRequest := httptest.NewRequest(http.MethodGet, "/v1/async/metrics", nil)
	metricsRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(metricsRecorder, metricsRequest)
	require.Equal(t, http.StatusOK, metricsRecorder.Code, metricsRecorder.Body.String())
	var metrics struct {
		Runtime struct {
			Running    int `json:"running"`
			Queued     int `json:"queued"`
			MaxRunning int `json:"maxRunning"`
			MaxQueued  int `json:"maxQueued"`
		} `json:"runtime"`
	}
	require.NoError(t, common.Unmarshal(metricsRecorder.Body.Bytes(), &metrics))
	require.Equal(t, 1, metrics.Runtime.Running)
	require.Equal(t, 1, metrics.Runtime.Queued)
	require.Equal(t, 1, metrics.Runtime.MaxRunning)
	require.Equal(t, 4, metrics.Runtime.MaxQueued)

	releaseFirst <- struct{}{}
	require.Eventually(t, func() bool {
		select {
		case prompt := <-started:
			return prompt == "second"
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
	releaseSecond <- struct{}{}

	require.Eventually(t, func() bool {
		var count int64
		err := model.DB.Model(&model.Task{}).Where("task_id IN ? AND status = ?", []string{first.ID, second.ID}, model.TaskStatusSuccess).Count(&count).Error
		return err == nil && count == 2
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncTaskIdempotencyKeyReplaysSameTaskWithoutDuplicateExecution(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	restoreScheduler := setAsyncTaskSchedulerForTest(1, 4)
	defer restoreScheduler()
	first := createAsyncTaskForTest(t, engine, token, "same prompt", "retry-key-1")
	require.Equal(t, "queued", first.Status)
	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	second := createAsyncTaskForTest(t, engine, token, "same prompt", "retry-key-1")
	require.Equal(t, first.ID, second.ID)

	select {
	case <-started:
		t.Fatal("idempotent replay executed upstream a second time")
	case <-time.After(100 * time.Millisecond):
	}

	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ? AND platform = ?", 2001, asyncTaskPlatformOpenAI).Count(&taskCount).Error)
	require.EqualValues(t, 1, taskCount)

	close(release)
	released = true
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", first.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusSuccess
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncTaskIdempotencyKeyRejectsDifferentPayload(t *testing.T) {
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	first := createAsyncTaskForTest(t, engine, token, "original prompt", "conflict-key-1")
	require.NotEmpty(t, first.ID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"image",
		"action":"generate",
		"model":"gpt-image-2",
		"idempotency_key":"conflict-key-1",
		"input":{"prompt":"changed prompt"},
		"parameters":{"quality":"high","size":"1024x1024","n":1}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "idempotency_key")

	close(release)
	released = true
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", first.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusSuccess
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncImageEditForcesURLResponseFormat(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("edit-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(2<<20))
		require.Equal(t, "url", r.FormValue("response_format"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/edit.png"}]}`))
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
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
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
	require.Eventually(t, func() bool {
		select {
		case <-upstreamCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncImageGenerationRecordsBillingAndURLWithoutBase64InTaskData(t *testing.T) {
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
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
	require.Eventually(t, func() bool {
		statusRecorder := httptest.NewRecorder()
		statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
		statusRequest.Header.Set("Authorization", "Bearer "+token)
		engine.ServeHTTP(statusRecorder, statusRequest)
		return statusRecorder.Code == http.StatusOK && strings.Contains(statusRecorder.Body.String(), `"url":"`+resultURLServer.URL+`/result.png"`)
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.NotZero(t, task.Quota)
	require.EqualValues(t, model.TaskStatusSuccess, task.Status)
	require.Equal(t, resultURLServer.URL+"/result.png", task.PrivateData.ResultURL)
	require.NotContains(t, string(task.Data), "aW1nLWJ5dGVz")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000-task.Quota, user.Quota)
	require.Equal(t, task.Quota, user.UsedQuota)

	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, 4001).Error)
	require.Equal(t, int64(task.Quota), channel.UsedQuota)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, task.Quota, logs[0].Quota)
	require.Equal(t, "gpt-image-2", logs[0].ModelName)
	require.Equal(t, 4001, logs[0].ChannelId)
	require.Equal(t, 3001, logs[0].TokenId)
	require.Contains(t, logs[0].Other, `"is_task":true`)
}

func TestAsyncImageGenerationUsesMappedUpstreamModel(t *testing.T) {
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"model":"upstream-image-real"`)
		require.NotContains(t, string(body), `"model":"studio-image-alias"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTestWithMapping(t, upstream.URL, "studio-image-alias", `{"studio-image-alias":"upstream-image-real"}`)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"studio-image-alias":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"image",
		"action":"generate",
		"model":"studio-image-alias",
		"input":{"prompt":"draw a studio"},
		"parameters":{"quality":"high","size":"1024x1024","n":1}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusSuccess
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.Equal(t, "studio-image-alias", task.Properties.OriginModelName)
	require.Equal(t, "upstream-image-real", task.Properties.UpstreamModelName)
}

func TestAsyncImageGenerationSpecPricingDependsOnQualitySizeAndCount(t *testing.T) {
	withAsyncTaskSpecPricingEnabled(t, true)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	var baseQuota, highQualityQuota, largeSizeQuota, countQuota int
	t.Run("base", func(t *testing.T) {
		baseQuota = asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
			"quality": "standard",
			"size":    "1024x1024",
			"n":       1,
		})
	})
	t.Run("high-quality", func(t *testing.T) {
		highQualityQuota = asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
			"quality": "high",
			"size":    "1024x1024",
			"n":       1,
		})
	})
	t.Run("large-size", func(t *testing.T) {
		largeSizeQuota = asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
			"quality": "standard",
			"size":    "1792x1024",
			"n":       1,
		})
	})
	t.Run("count", func(t *testing.T) {
		countQuota = asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
			"quality": "standard",
			"size":    "1024x1024",
			"n":       2,
		})
	})

	require.NotEqual(t, baseQuota, highQualityQuota)
	require.NotEqual(t, baseQuota, largeSizeQuota)
	require.NotEqual(t, baseQuota, countQuota)
}

func TestAsyncPricingEstimateMatchesTaskQuotaWithoutSideEffects(t *testing.T) {
	withAsyncTaskSpecPricingEnabled(t, true)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskProductRouterTest(t, upstream.URL, "gpt-image-2", constant.ChannelTypeOpenAI, "")
	payload := `{
		"kind":"image",
		"action":"generate",
		"model":"gpt-image-2",
		"input":{"prompt":"estimate product image"},
		"parameters":{"quality":"high","size":"1792x1024","n":2}
	}`

	estimateRecorder := httptest.NewRecorder()
	estimateRequest := httptest.NewRequest(http.MethodPost, "/v1/pricing/estimate", strings.NewReader(payload))
	estimateRequest.Header.Set("Authorization", "Bearer "+token)
	estimateRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(estimateRecorder, estimateRequest)
	require.Equal(t, http.StatusOK, estimateRecorder.Code, estimateRecorder.Body.String())

	var estimate asyncTaskPricingEstimateResponse
	require.NoError(t, common.Unmarshal(estimateRecorder.Body.Bytes(), &estimate))
	require.Equal(t, "quota", estimate.Unit)
	require.Greater(t, estimate.Quota, 0)
	require.InDelta(t, operation_setting.AsyncImageQualityHighMultiplier, estimate.Breakdown.OtherRatios["quality"], 0.001)
	require.InDelta(t, operation_setting.AsyncImageSize1792x1024Multiplier, estimate.Breakdown.OtherRatios["size"], 0.001)
	require.InDelta(t, 2, estimate.Breakdown.OtherRatios["n"], 0.001)

	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
	require.Zero(t, taskCount)
	var logCount int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logCount).Error)
	require.Zero(t, logCount)
	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, 3001).Error)
	require.Equal(t, 1000000, storedToken.RemainQuota)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/v1/images/tasks", strings.NewReader(payload))
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRecorder, createRequest)
	require.Equal(t, http.StatusOK, createRecorder.Code, createRecorder.Body.String())

	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(createRecorder.Body.Bytes(), &created))
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.Equal(t, estimate.Quota, task.Quota)
}

func TestSafeAsyncTaskErrorRedactsSecretsAndInternalURLs(t *testing.T) {
	message := safeAsyncTaskError(fmt.Errorf(
		`upstream https://api.internal.example/v1 failed Authorization: Bearer sk-live-secret api_key=provider-secret base_url=http://relay-cli-proxy:3000`,
	))

	require.NotContains(t, message, "sk-live-secret")
	require.NotContains(t, message, "provider-secret")
	require.NotContains(t, message, "api.internal.example")
	require.NotContains(t, message, "relay-cli-proxy")
	require.Contains(t, message, "[redacted")
}

func TestAsyncBillingBalanceAndUsageAreReadOnly(t *testing.T) {
	engine, token := setupAsyncTaskProductRouterTest(t, "https://upstream.example", "gpt-image-2", constant.ChannelTypeOpenAI, "")
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    2001,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-image-2",
		TokenName: "cavas",
		Quota:     1234,
		ChannelId: 4001,
		TokenId:   3001,
		Group:     "default",
		CreatedAt: 100,
	}).Error)

	balanceRecorder := httptest.NewRecorder()
	balanceRequest := httptest.NewRequest(http.MethodGet, "/v1/billing/balance", nil)
	balanceRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(balanceRecorder, balanceRequest)
	require.Equal(t, http.StatusOK, balanceRecorder.Code, balanceRecorder.Body.String())

	var balance asyncBillingBalanceResponse
	require.NoError(t, common.Unmarshal(balanceRecorder.Body.Bytes(), &balance))
	require.Equal(t, 1000000, balance.Quota)
	require.Equal(t, "quota", balance.Unit)
	require.Equal(t, 2001, balance.UserID)

	usageRecorder := httptest.NewRecorder()
	usageRequest := httptest.NewRequest(http.MethodGet, "/v1/billing/usage?p=1&page_size=10", nil)
	usageRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(usageRecorder, usageRequest)
	require.Equal(t, http.StatusOK, usageRecorder.Code, usageRecorder.Body.String())

	var usage asyncBillingUsageResponse
	require.NoError(t, common.Unmarshal(usageRecorder.Body.Bytes(), &usage))
	require.Equal(t, 1, usage.Page)
	require.Equal(t, 10, usage.PageSize)
	require.Equal(t, 1, usage.Total)
	require.Len(t, usage.Items, 1)
	require.Equal(t, "gpt-image-2", usage.Items[0].ModelName)
	require.Equal(t, 1234, usage.Items[0].Quota)
	require.Equal(t, model.LogTypeConsume, usage.Items[0].Type)

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, 3001).Error)
	require.Equal(t, 1000000, storedToken.RemainQuota)
}

func TestAsyncVideoGenerationSpecPricingDependsOnResolutionAndDuration(t *testing.T) {
	withAsyncTaskSpecPricingEnabled(t, true)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/jobs/createTask":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-task-1"}}`))
		case "/api/v1/jobs/recordInfo":
			require.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-task-1","state":"success","resultJson":"{\"resultUrls\":[\"https://example.test/video.mp4\"]}"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	var baseQuota, highResolutionQuota, longDurationQuota int
	t.Run("base", func(t *testing.T) {
		baseQuota = asyncTaskQuotaForVideoRequest(t, upstream.URL, map[string]interface{}{
			"ratio":      "16:9",
			"resolution": "480p",
			"duration":   4,
		})
	})
	t.Run("high-resolution", func(t *testing.T) {
		highResolutionQuota = asyncTaskQuotaForVideoRequest(t, upstream.URL, map[string]interface{}{
			"ratio":      "16:9",
			"resolution": "1080p",
			"duration":   4,
		})
	})
	t.Run("long-duration", func(t *testing.T) {
		longDurationQuota = asyncTaskQuotaForVideoRequest(t, upstream.URL, map[string]interface{}{
			"ratio":      "16:9",
			"resolution": "480p",
			"duration":   10,
		})
	})

	require.NotEqual(t, baseQuota, highResolutionQuota)
	require.NotEqual(t, baseQuota, longDurationQuota)
}

func TestAsyncImageSpecPricingIsDisabledByDefault(t *testing.T) {
	withAsyncTaskSpecPricingEnabled(t, false)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	baseQuota := asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
		"quality": "standard",
		"size":    "1024x1024",
		"n":       1,
	})
	highQuota := asyncTaskQuotaForImageRequest(t, upstream.URL, map[string]interface{}{
		"quality": "high",
		"size":    "1792x1024",
		"n":       2,
	})

	require.Equal(t, baseQuota, highQuota)
}

func TestAsyncKieSeedanceVideoTaskPollsAndProxiesContent(t *testing.T) {
	videoContent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/video.mp4", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("mp4-bytes"))
	}))
	defer videoContent.Close()

	createCalled := make(chan struct{}, 1)
	pollCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/jobs/createTask":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload map[string]interface{}
			require.NoError(t, common.Unmarshal(body, &payload))
			require.Equal(t, "bytedance/seedance-2-fast", payload["model"])
			input, ok := payload["input"].(map[string]interface{})
			require.True(t, ok, "expected KIE payload input object: %#v", payload)
			require.Equal(t, "moving product shot", input["prompt"])
			require.Equal(t, "16:9", input["aspect_ratio"])
			require.Equal(t, "720p", input["resolution"])
			require.InDelta(t, 6, input["duration"], 0.001)
			require.Equal(t, true, input["generate_audio"])
			require.Equal(t, false, input["watermark"])
			require.NotContains(t, payload, "content")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-task-1"}}`))
			createCalled <- struct{}{}
		case "/api/v1/jobs/recordInfo":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			require.Equal(t, "kie-task-1", r.URL.Query().Get("taskId"))
			w.Header().Set("Content-Type", "application/json")
			resultJSON := `{"resultUrls":["` + videoContent.URL + `/video.mp4"]}`
			encoded, err := common.Marshal(map[string]interface{}{
				"code": 200,
				"msg":  "success",
				"data": map[string]interface{}{
					"taskId":     "kie-task-1",
					"state":      "success",
					"resultJson": resultJSON,
				},
			})
			require.NoError(t, err)
			_, _ = w.Write(encoded)
			pollCalled <- struct{}{}
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTestWithChannel(t, upstream.URL, "bytedance/seedance-2-fast", constant.ChannelTypeKie)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"bytedance/seedance-2-fast":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"video",
		"action":"generate",
		"model":"bytedance/seedance-2-fast",
		"input":{"prompt":"moving product shot"},
		"parameters":{
			"content":[{"type":"text","text":"moving product shot"}],
			"ratio":"16:9",
			"resolution":"720p",
			"duration":6,
			"generate_audio":true,
			"watermark":false
		}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, "queued", created.Status)
	require.Equal(t, "video", created.Kind)

	require.Eventually(t, func() bool {
		select {
		case <-createCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-pollCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&stored).Error)
	require.Equal(t, "kie-task-1", stored.PrivateData.UpstreamTaskID)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	require.Eventually(t, func() bool {
		statusRecorder = httptest.NewRecorder()
		engine.ServeHTTP(statusRecorder, statusRequest)
		return statusRecorder.Code == http.StatusOK && strings.Contains(statusRecorder.Body.String(), `"status":"succeeded"`)
	}, 2*time.Second, 20*time.Millisecond, statusRecorder.Body.String())
	var status asyncTaskResponse
	require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &status))
	require.Len(t, status.Outputs, 1)
	require.Equal(t, "video/mp4", status.Outputs[0].MimeType)
	require.Equal(t, videoContent.URL+"/video.mp4", status.Outputs[0].URL)

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "video/mp4", contentRecorder.Header().Get("Content-Type"))
	require.Equal(t, "mp4-bytes", contentRecorder.Body.String())
}

func TestAsyncSeedanceProductAliasRoutesToJimengOpenAIVideoChannel(t *testing.T) {
	createCalled := make(chan struct{}, 1)
	pollCalled := make(chan struct{}, 1)
	contentCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload map[string]interface{}
			require.NoError(t, common.Unmarshal(body, &payload))
			require.Equal(t, "video-ds-2.0-fast", payload["model"])
			require.Equal(t, "moving product shot", payload["prompt"])
			require.Equal(t, "5", payload["seconds"])
			require.Equal(t, "16:9", payload["aspect_ratio"])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"zz1-task-1","status":"queued"}`))
			createCalled <- struct{}{}
		case "/v1/videos/zz1-task-1":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"zz1-task-1","status":"completed","progress":100}`))
			pollCalled <- struct{}{}
		case "/v1/videos/zz1-task-1/content":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("zz1-mp4-bytes"))
			contentCalled <- struct{}{}
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTestWithChannelAndMapping(
		t,
		upstream.URL,
		"seedance-2.0-fast",
		constant.ChannelTypeJimengOpenAIVideo,
		`{"seedance-2.0-fast":"video-ds-2.0-fast"}`,
	)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2.0-fast":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"video",
		"action":"generate",
		"model":"seedance-2.0-fast",
		"input":{"prompt":"moving product shot"},
		"parameters":{"content":[{"type":"text","text":"moving product shot"}],"ratio":"16:9","duration":5}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		select {
		case <-createCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-pollCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&stored).Error)
	require.Equal(t, "seedance-2.0-fast", stored.Properties.OriginModelName)
	require.Equal(t, "video-ds-2.0-fast", stored.Properties.UpstreamModelName)
	require.Equal(t, "zz1-task-1", stored.PrivateData.UpstreamTaskID)
	require.EqualValues(t, model.TaskStatusSuccess, stored.Status)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	var status asyncTaskResponse
	require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &status))
	require.Len(t, status.Outputs, 1)
	require.Equal(t, "video/mp4", status.Outputs[0].MimeType)

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "video/mp4", contentRecorder.Header().Get("Content-Type"))
	require.Equal(t, "zz1-mp4-bytes", contentRecorder.Body.String())
	require.Eventually(t, func() bool {
		select {
		case <-contentCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncSeedanceProductAliasRoutesToKieStandardModel(t *testing.T) {
	videoContent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/seedance-2.mp4", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("kie-standard-mp4"))
	}))
	defer videoContent.Close()

	createCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/jobs/createTask":
			require.Equal(t, http.MethodPost, r.Method)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload map[string]interface{}
			require.NoError(t, common.Unmarshal(body, &payload))
			require.Equal(t, "bytedance/seedance-2", payload["model"])
			input, ok := payload["input"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "standard product shot", input["prompt"])
			require.Equal(t, "9:16", input["aspect_ratio"])
			require.InDelta(t, 10, input["duration"], 0.001)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-standard-1"}}`))
			createCalled <- struct{}{}
		case "/api/v1/jobs/recordInfo":
			require.Equal(t, "kie-standard-1", r.URL.Query().Get("taskId"))
			w.Header().Set("Content-Type", "application/json")
			resultJSON := `{"resultUrls":["` + videoContent.URL + `/seedance-2.mp4"]}`
			encoded, err := common.Marshal(map[string]interface{}{
				"code": 200,
				"data": map[string]interface{}{
					"taskId":     "kie-standard-1",
					"state":      "success",
					"resultJson": resultJSON,
				},
			})
			require.NoError(t, err)
			_, _ = w.Write(encoded)
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTestWithChannelAndMapping(
		t,
		upstream.URL,
		"seedance-2.0",
		constant.ChannelTypeKie,
		`{"seedance-2.0":"bytedance/seedance-2"}`,
	)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2.0":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"video",
		"action":"generate",
		"model":"seedance-2.0",
		"input":{"prompt":"standard product shot"},
		"parameters":{"content":[{"type":"text","text":"standard product shot"}],"ratio":"9:16","duration":10}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		select {
		case <-createCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&stored).Error)
	require.Equal(t, "seedance-2.0", stored.Properties.OriginModelName)
	require.Equal(t, "bytedance/seedance-2", stored.Properties.UpstreamModelName)
	require.Equal(t, "kie-standard-1", stored.PrivateData.UpstreamTaskID)
	require.EqualValues(t, model.TaskStatusSuccess, stored.Status)
}

func TestAsyncKieSeedanceVideoTaskFailureRefundsQuota(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/jobs/createTask":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-failed-1"}}`))
		case "/api/v1/jobs/recordInfo":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "kie-failed-1", r.URL.Query().Get("taskId"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"kie-failed-1","state":"failed","failReason":"KIE rejected prompt"}}`))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTestWithChannel(t, upstream.URL, "bytedance/seedance-2-fast", constant.ChannelTypeKie)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"bytedance/seedance-2-fast":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"video",
		"action":"generate",
		"model":"bytedance/seedance-2-fast",
		"input":{"prompt":"blocked prompt"},
		"parameters":{"content":[{"type":"text","text":"blocked prompt"}],"ratio":"16:9","resolution":"480p","duration":4}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusFailure
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.Equal(t, "kie-failed-1", task.PrivateData.UpstreamTaskID)
	require.Contains(t, task.FailReason, "KIE rejected prompt")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)

	var refundLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeRefund).Find(&refundLogs).Error)
	require.Len(t, refundLogs, 1)
	require.Equal(t, task.Quota, refundLogs[0].Quota)
}

func TestAsyncProductImageTaskRouteDefaultsKindToImage(t *testing.T) {
	upstreamCalled := make(chan struct{}, 1)
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("product-image-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/image.png"}]}`))
		upstreamCalled <- struct{}{}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskProductRouterTest(t, upstream.URL, "gpt-image-2", constant.ChannelTypeOpenAI, "")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/images/tasks", strings.NewReader(`{
		"action":"generate",
		"model":"gpt-image-2",
		"input":{"prompt":"product route image"},
		"parameters":{"quality":"high","size":"1024x1024","n":1}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, asyncTaskKindImage, created.Kind)
	require.Eventually(t, func() bool {
		select {
		case <-upstreamCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&stored).Error)
	var data asyncTaskData
	require.NoError(t, stored.GetData(&data))
	require.Equal(t, asyncTaskKindImage, data.Kind)
}

func TestAsyncProductVideoTaskRouteDefaultsKindToVideo(t *testing.T) {
	videoContent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("product-video-bytes"))
	}))
	defer videoContent.Close()

	createCalled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload map[string]interface{}
			require.NoError(t, common.Unmarshal(body, &payload))
			require.Equal(t, "video-ds-2.0-fast", payload["model"])
			require.Equal(t, "product route video", payload["prompt"])
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"product-video-1","status":"queued"}`))
			createCalled <- struct{}{}
		case "/v1/videos/product-video-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"product-video-1","status":"completed","progress":100}`))
		case "/v1/videos/product-video-1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("product-video-bytes"))
		default:
			t.Fatalf("unexpected upstream path: %s", r.URL.String())
		}
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskProductRouterTest(
		t,
		upstream.URL,
		"seedance-2.0-fast",
		constant.ChannelTypeJimengOpenAIVideo,
		`{"seedance-2.0-fast":"video-ds-2.0-fast"}`,
	)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2.0-fast":0.01}`))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/videos/tasks", strings.NewReader(`{
		"action":"generate",
		"model":"seedance-2.0-fast",
		"input":{"prompt":"product route video"},
		"parameters":{"content":[{"type":"text","text":"product route video"}],"ratio":"16:9","duration":5}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Equal(t, asyncTaskKindVideo, created.Kind)
	require.Eventually(t, func() bool {
		select {
		case <-createCalled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&stored).Error)
	var data asyncTaskData
	require.NoError(t, stored.GetData(&data))
	require.Equal(t, asyncTaskKindVideo, data.Kind)
	require.EqualValues(t, model.TaskStatusSuccess, stored.Status)
}

func TestAsyncImageGenerationAcceptsBase64OutputAndServesContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"image",
		"action":"generate",
		"model":"gpt-image-2",
		"input":{"prompt":"draw a studio"},
		"parameters":{"quality":"high","size":"1024x1024","n":1,"output_format":"webp"}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusSuccess
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.Empty(t, task.FailReason)
	require.Contains(t, string(task.Data), "aW1nLWJ5dGVz")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000-task.Quota, user.Quota)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	require.NotContains(t, statusRecorder.Body.String(), "aW1nLWJ5dGVz")
	require.Contains(t, statusRecorder.Body.String(), `"mimeType":"image/webp"`)
	require.Contains(t, statusRecorder.Body.String(), `"size":9`)

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "image/webp", contentRecorder.Header().Get("Content-Type"))
	require.Equal(t, "img-bytes", contentRecorder.Body.String())
}

func TestAsyncImageGenerationTreatsDataURLAsInlineContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"data:image/webp;base64,aW1nLWJ5dGVz"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", strings.NewReader(`{
		"kind":"image",
		"action":"generate",
		"model":"gpt-image-2",
		"input":{"prompt":"draw a studio"},
		"parameters":{"quality":"high","size":"1024x1024","n":1,"output_format":"webp"}
	}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusSuccess
	}, 2*time.Second, 20*time.Millisecond)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	require.NotContains(t, statusRecorder.Body.String(), "data:image")
	require.NotContains(t, statusRecorder.Body.String(), "aW1nLWJ5dGVz")
	require.Contains(t, statusRecorder.Body.String(), `"mimeType":"image/webp"`)
	require.Contains(t, statusRecorder.Body.String(), `"size":9`)

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(contentRecorder, contentRequest)
	require.Equal(t, http.StatusOK, contentRecorder.Code, contentRecorder.Body.String())
	require.Equal(t, "image/webp", contentRecorder.Header().Get("Content-Type"))
	require.Equal(t, "img-bytes", contentRecorder.Body.String())
}

func TestAsyncImageGenerationRejectsOversizedBase64Output(t *testing.T) {
	restoreLimit := setAsyncTaskInlineContentLimitForTest(4)
	defer restoreLimit()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1nLWJ5dGVz"}]}`))
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
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusFailure
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.Contains(t, task.FailReason, "inline base64 image is too large")
	require.NotContains(t, string(task.Data), "aW1nLWJ5dGVz")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)
}

func TestAsyncTaskContentProxyUsesAsyncHTTPClient(t *testing.T) {
	db := setupAsyncTaskTestDB(t)
	t.Setenv("ASYNC_TASK_HTTP_TIMEOUT_SECONDS", "240")
	contentFetched := make(chan struct{}, 1)
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https://example.test/result.png", req.URL.String())
		require.NotNil(t, req.Context())
		assertAsyncRequestDeadline(t, req, 240*time.Second)
		contentFetched <- struct{}{}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/webp"}},
			Body:       io.NopCloser(strings.NewReader("img-bytes")),
			Request:    req,
		}, nil
	})})
	defer restoreClient()

	task := &model.Task{
		TaskID:     "task_content_proxy",
		UserId:     2001,
		Group:      "default",
		ChannelId:  4001,
		Platform:   constant.TaskPlatform("openai-async"),
		Action:     asyncTaskActionGenerate,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: time.Now().Unix(),
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
		FinishTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}
	task.SetData(asyncTaskData{
		Kind:   asyncTaskKindImage,
		Action: asyncTaskActionGenerate,
		Model:  "gpt-image-2",
		Outputs: []asyncTaskStoredOutput{{
			MimeType: "image/png",
			URL:      "https://example.test/result.png",
		}},
	})
	require.NoError(t, db.Create(task).Error)

	engine := gin.New()
	asyncRouter := engine.Group("/v1/async")
	asyncRouter.Use(middleware.TokenAuth())
	asyncRouter.GET("/tasks/:id/content", GetAsyncTaskContent)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+task.TaskID+"/content", nil)
	request.Header.Set("Authorization", "Bearer sk-cavas")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "img-bytes", recorder.Body.String())
	require.Equal(t, "image/webp", recorder.Header().Get("Content-Type"))
	select {
	case <-contentFetched:
	default:
		t.Fatal("content proxy did not use asyncTaskHTTPClient")
	}
}

func TestAsyncTaskFailureRefundsPreConsumedQuotaOnce(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"upstream exploded"}}`, http.StatusBadGateway)
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
	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusFailure
	}, 2*time.Second, 20*time.Millisecond)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", created.ID).First(&task).Error)
	require.NotZero(t, task.Quota)

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)
	require.Equal(t, task.Quota, user.UsedQuota)

	var refundLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeRefund).Find(&refundLogs).Error)
	require.Len(t, refundLogs, 1)
	require.Equal(t, task.Quota, refundLogs[0].Quota)

	completeAsyncTaskFailure(&task, asyncTaskRequest{Kind: asyncTaskKindImage, Action: asyncTaskActionGenerate, Model: "gpt-image-2"}, "retry failure")
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeRefund).Find(&refundLogs).Error)
	require.Len(t, refundLogs, 1)
}

func asyncTaskQuotaForImageRequest(t *testing.T, upstreamURL string, parameters map[string]interface{}) int {
	t.Helper()
	engine, token := setupAsyncTaskRouterTest(t, upstreamURL, "gpt-image-2")
	recorder := httptest.NewRecorder()
	requestBody, err := common.Marshal(map[string]interface{}{
		"kind":       "image",
		"action":     "generate",
		"model":      "gpt-image-2",
		"input":      map[string]interface{}{"prompt": "draw a studio"},
		"parameters": parameters,
	})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", bytes.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	var task model.Task
	require.Eventually(t, func() bool {
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	return task.Quota
}

func withAsyncTaskSpecPricingEnabled(t *testing.T, enabled bool) {
	t.Helper()

	original := operation_setting.AsyncTaskSpecPricingEnabled
	operation_setting.AsyncTaskSpecPricingEnabled = enabled
	t.Cleanup(func() {
		operation_setting.AsyncTaskSpecPricingEnabled = original
	})
}

func asyncTaskQuotaForVideoRequest(t *testing.T, upstreamURL string, parameters map[string]interface{}) int {
	t.Helper()
	engine, token := setupAsyncTaskRouterTestWithChannel(t, upstreamURL, "bytedance/seedance-2-fast", constant.ChannelTypeKie)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"bytedance/seedance-2-fast":0.01}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	})

	recorder := httptest.NewRecorder()
	requestBody, err := common.Marshal(map[string]interface{}{
		"kind":       "video",
		"action":     "generate",
		"model":      "bytedance/seedance-2-fast",
		"input":      map[string]interface{}{"prompt": "moving product shot"},
		"parameters": parameters,
	})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", bytes.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	var task model.Task
	require.Eventually(t, func() bool {
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	return task.Quota
}

func TestAsyncTaskCancelPreventsSuccessOverwrite(t *testing.T) {
	finishUpstream := make(chan struct{})
	upstreamStarted := make(chan struct{})
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamStarted)
		<-finishUpstream
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + resultURLServer.URL + `/result.png"}]}`))
	}))
	defer upstream.Close()

	engine, token := setupAsyncTaskRouterTest(t, upstream.URL, "gpt-image-2")
	restoreScheduler := setAsyncTaskSchedulerForTest(1, 4)
	defer restoreScheduler()
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
	require.Eventually(t, func() bool {
		select {
		case <-upstreamStarted:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/v1/async/tasks/"+created.ID+"/cancel", nil)
	cancelRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(cancelRecorder, cancelRequest)
	require.Equal(t, http.StatusOK, cancelRecorder.Code, cancelRecorder.Body.String())

	close(finishUpstream)

	require.Eventually(t, func() bool {
		var task model.Task
		err := model.DB.Where("task_id = ?", created.ID).First(&task).Error
		return err == nil && task.Status == model.TaskStatusFailure && task.FailReason == asyncTaskStatusCanceled
	}, 2*time.Second, 20*time.Millisecond)

	var user model.User
	require.NoError(t, model.DB.First(&user, 2001).Error)
	require.Equal(t, 1000000, user.Quota)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/async/tasks/"+created.ID, nil)
	statusRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	require.Contains(t, statusRecorder.Body.String(), `"status":"canceled"`)
}

func TestAsyncTaskCancelAbortsInFlightUpstreamRequest(t *testing.T) {
	requestCanceled := make(chan struct{})
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(requestStarted)
		select {
		case <-req.Context().Done():
			close(requestCanceled)
			return nil, req.Context().Err()
		case <-releaseRequest:
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://example.test/result.png"}]}`)),
				Request:    req,
			}, nil
		}
	})})
	defer func() {
		close(releaseRequest)
		restoreClient()
	}()

	engine, token := setupAsyncTaskRouterTest(t, "https://upstream.example", "gpt-image-2")
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

	require.Eventually(t, func() bool {
		select {
		case <-requestStarted:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/v1/async/tasks/"+created.ID+"/cancel", nil)
	cancelRequest.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(cancelRecorder, cancelRequest)
	require.Equal(t, http.StatusOK, cancelRecorder.Code, cancelRecorder.Body.String())
	require.Contains(t, cancelRecorder.Body.String(), `"status":"canceled"`)

	require.Eventually(t, func() bool {
		select {
		case <-requestCanceled:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
}

func TestAsyncTaskExecutorSkipsCanceledTaskBeforeCallingUpstream(t *testing.T) {
	db := setupAsyncTaskTestDB(t)
	upstreamCalled := make(chan struct{}, 1)
	restoreClient := setAsyncTaskHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		upstreamCalled <- struct{}{}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://example.test/result.png"}]}`)),
			Request:    req,
		}, nil
	})})
	defer restoreClient()

	upstreamURL := "https://upstream.example"
	require.NoError(t, db.Create(&model.Channel{
		Id:      4001,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-upstream",
		Status:  common.ChannelStatusEnabled,
		Name:    "CPA OpenAI Compatible",
		BaseURL: &upstreamURL,
		Models:  "gpt-image-2",
		Group:   "default",
	}).Error)
	model.InitChannelCache()

	task := &model.Task{
		TaskID:     "task_canceled_before_execute",
		UserId:     2001,
		Group:      "default",
		ChannelId:  4001,
		Quota:      2500,
		Platform:   constant.TaskPlatform("openai-async"),
		Action:     asyncTaskActionGenerate,
		Status:     model.TaskStatusFailure,
		Progress:   "100%",
		FailReason: asyncTaskStatusCanceled,
		SubmitTime: time.Now().Unix(),
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
	}
	task.SetData(asyncTaskData{Kind: asyncTaskKindImage, Action: asyncTaskActionGenerate, Model: "gpt-image-2"})
	require.NoError(t, db.Create(task).Error)

	executeAsyncTaskInBackground(task.TaskID, 4001, asyncTaskExecution{
		Request: asyncTaskRequest{Kind: asyncTaskKindImage, Action: asyncTaskActionGenerate, Model: "gpt-image-2", Input: asyncTaskInput{Prompt: "draw a studio"}},
		Context: context.Background(),
	})

	select {
	case <-upstreamCalled:
		t.Fatal("executor called upstream for a canceled task")
	default:
	}
}

func TestAsyncTimedOutTaskIsFailedAndRefunded(t *testing.T) {
	db := setupAsyncTaskTestDB(t)
	task := &model.Task{
		TaskID:     "task_timeout",
		UserId:     2001,
		Group:      "default",
		ChannelId:  4001,
		Quota:      2500,
		Platform:   constant.TaskPlatform("openai-async"),
		Action:     asyncTaskActionGenerate,
		Status:     model.TaskStatusInProgress,
		Progress:   "0%",
		SubmitTime: time.Now().Add(-2 * time.Hour).Unix(),
		CreatedAt:  time.Now().Add(-2 * time.Hour).Unix(),
		UpdatedAt:  time.Now().Add(-2 * time.Hour).Unix(),
		Properties: model.Properties{OriginModelName: "gpt-image-2"},
		PrivateData: model.TaskPrivateData{
			BillingSource: service.BillingSourceWallet,
			TokenId:       3001,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.01,
				GroupRatio:      1,
				OriginModelName: "gpt-image-2",
				PerCallBilling:  true,
			},
		},
	}
	task.SetData(asyncTaskData{Kind: asyncTaskKindImage, Action: asyncTaskActionGenerate, Model: "gpt-image-2"})
	require.NoError(t, db.Create(task).Error)

	SweepAsyncTimedOutTasksForTest(time.Now().Add(-time.Hour).Unix(), 100)

	var reloaded model.Task
	require.NoError(t, db.Where("task_id = ?", task.TaskID).First(&reloaded).Error)
	require.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	require.Contains(t, reloaded.FailReason, "timeout")

	var user model.User
	require.NoError(t, db.First(&user, 2001).Error)
	require.Equal(t, 1000000+task.Quota, user.Quota)

	var refundLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeRefund).Find(&refundLogs).Error)
	require.Len(t, refundLogs, 1)
	require.Equal(t, task.Quota, refundLogs[0].Quota)

	require.Equal(t, asyncTaskStatusTimeout, asyncTaskModelToResponse(&reloaded).Status)
}

func setupAsyncTaskRouterTest(t *testing.T, upstreamURL string, modelName string) (*gin.Engine, string) {
	t.Helper()
	return setupAsyncTaskRouterTestWithMapping(t, upstreamURL, modelName, "")
}

func setupAsyncTaskRouterTestWithMapping(t *testing.T, upstreamURL string, modelName string, modelMapping string) (*gin.Engine, string) {
	t.Helper()
	return setupAsyncTaskRouterTestWithChannelAndMapping(t, upstreamURL, modelName, constant.ChannelTypeOpenAI, modelMapping)
}

func setupAsyncTaskRouterTestWithChannel(t *testing.T, upstreamURL string, modelName string, channelType int) (*gin.Engine, string) {
	t.Helper()
	return setupAsyncTaskRouterTestWithChannelAndMapping(t, upstreamURL, modelName, channelType, "")
}

func setupAsyncTaskRouterTestWithChannelAndMapping(t *testing.T, upstreamURL string, modelName string, channelType int, modelMapping string) (*gin.Engine, string) {
	t.Helper()
	db := setupAsyncTaskTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:           4001,
		Type:         channelType,
		Key:          "sk-upstream",
		Status:       common.ChannelStatusEnabled,
		Name:         "CPA OpenAI Compatible",
		BaseURL:      &upstreamURL,
		Models:       modelName,
		Group:        "default",
		ModelMapping: optionalStringPointer(modelMapping),
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
		asyncRouter.GET("/metrics", GetAsyncTaskMetrics)
		asyncRouter.GET("/tasks/:id", GetAsyncTask)
		asyncRouter.POST("/tasks/:id/cancel", CancelAsyncTask)
		asyncRouter.GET("/tasks/:id/content", GetAsyncTaskContent)
	}
	return engine, "sk-cavas"
}

func setupAsyncTaskProductRouterTest(t *testing.T, upstreamURL string, modelName string, channelType int, modelMapping string) (*gin.Engine, string) {
	t.Helper()
	db := setupAsyncTaskTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:           4001,
		Type:         channelType,
		Key:          "sk-upstream",
		Status:       common.ChannelStatusEnabled,
		Name:         "CPA OpenAI Compatible",
		BaseURL:      &upstreamURL,
		Models:       modelName,
		Group:        "default",
		ModelMapping: optionalStringPointer(modelMapping),
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
	productRouter := engine.Group("/v1")
	productRouter.Use(middleware.TokenAuth())
	{
		productRouter.POST("/images/tasks", CreateAsyncImageTask)
		productRouter.POST("/videos/tasks", CreateAsyncVideoTask)
		productRouter.GET("/tasks/:id", GetAsyncTask)
		productRouter.POST("/tasks/:id/cancel", CancelAsyncTask)
		productRouter.GET("/tasks/:id/content", GetAsyncTaskContent)
		productRouter.POST("/pricing/estimate", EstimateAsyncTaskPricing)
		productRouter.GET("/billing/balance", GetAsyncBillingBalance)
		productRouter.GET("/billing/usage", GetAsyncBillingUsage)
	}
	return engine, "sk-cavas"
}

func createAsyncTaskForTest(t *testing.T, engine *gin.Engine, token string, prompt string, idempotencyKey string) asyncTaskResponse {
	t.Helper()
	body, err := common.Marshal(map[string]interface{}{
		"kind":            "image",
		"action":          "generate",
		"model":           "gpt-image-2",
		"idempotency_key": idempotencyKey,
		"input":           map[string]interface{}{"prompt": prompt},
		"parameters":      map[string]interface{}{"quality": "high", "size": "1024x1024", "n": 1},
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/async/tasks", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var created asyncTaskResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &created))
	return created
}

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func assertAsyncRequestDeadline(t *testing.T, req *http.Request, expected time.Duration) {
	t.Helper()
	deadline, ok := req.Context().Deadline()
	require.True(t, ok, "async request should have a context deadline")
	remaining := time.Until(deadline)
	require.GreaterOrEqual(t, remaining, expected-5*time.Second)
	require.LessOrEqual(t, remaining, expected)
}

func newAsyncEditExecutionForTimeoutTest(t *testing.T) asyncTaskExecution {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("kind", asyncTaskKindImage))
	require.NoError(t, writer.WriteField("action", asyncTaskActionEdit))
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("reference-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(2<<20))
	t.Cleanup(func() {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
	})
	return asyncTaskExecution{
		Request: asyncTaskRequest{
			Kind:       asyncTaskKindImage,
			Action:     asyncTaskActionEdit,
			Model:      "gpt-image-2",
			Input:      asyncTaskInput{Prompt: "edit this"},
			Parameters: map[string]interface{}{"n": 1},
		},
		Multipart: request.MultipartForm,
	}
}

func newAsyncGeminiEditExecutionForTest(t *testing.T) asyncTaskExecution {
	return newAsyncGeminiEditExecutionWithImageConfigForTest(t, "1K", "1:1")
}

func newAsyncGeminiEditExecutionWithImageConfigForTest(t *testing.T, quality string, size string) asyncTaskExecution {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("kind", asyncTaskKindImage))
	require.NoError(t, writer.WriteField("action", asyncTaskActionEdit))
	require.NoError(t, writer.WriteField("model", "gemini-2.5-flash-image"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	require.NoError(t, writer.WriteField("quality", quality))
	require.NoError(t, writer.WriteField("size", size))
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="image"; filename="reference.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write([]byte("reference-image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, request.ParseMultipartForm(2<<20))
	t.Cleanup(func() {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
	})
	return asyncTaskExecution{
		Request: asyncTaskRequest{
			Kind:       asyncTaskKindImage,
			Action:     asyncTaskActionEdit,
			Model:      "gemini-2.5-flash-image",
			Input:      asyncTaskInput{Prompt: "edit this"},
			Parameters: map[string]interface{}{"n": 2, "quality": quality, "size": size},
		},
		Multipart: request.MultipartForm,
	}
}

func setAsyncTaskHTTPClientForTest(client *http.Client) func() {
	previous := asyncTaskHTTPClient
	asyncTaskHTTPClient = client
	return func() {
		asyncTaskHTTPClient = previous
	}
}

func setupAsyncTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	resetAsyncTaskSchedulerForTest()
	t.Cleanup(func() {
		waitAsyncTaskSchedulerIdleForTest()
		resetAsyncTaskSchedulerForTest()
	})
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Task{}, &model.Log{}, &model.TopUp{}, &model.UserSubscription{}))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"gpt-image-2":0.01}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	})
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
		Key:            strings.TrimPrefix(token, "sk-"),
		Name:           "cavas",
		Status:         common.TokenStatusEnabled,
		RemainQuota:    1000000,
		UnlimitedQuota: false,
		UsedQuota:      0,
	}).Error)
	return db
}

func waitAsyncTaskSchedulerIdleForTest() {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		metrics := asyncTaskRuntimeMetrics()
		if metrics.Running == 0 && metrics.Queued == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestOpenAIModelListIncludesGPTImage2(t *testing.T) {
	require.Contains(t, openai.ModelList, "gpt-image-2")
}
