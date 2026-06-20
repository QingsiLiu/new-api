package controller

import (
	"bytes"
	"context"
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

func TestAsyncTaskCancelPreventsSuccessOverwrite(t *testing.T) {
	finishUpstream := make(chan struct{})
	resultURLServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("img-bytes"))
	}))
	defer resultURLServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-finishUpstream
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
	db := setupAsyncTaskTestDB(t)
	require.NoError(t, db.Create(&model.Channel{
		Id:           4001,
		Type:         constant.ChannelTypeOpenAI,
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
		asyncRouter.GET("/tasks/:id", GetAsyncTask)
		asyncRouter.POST("/tasks/:id/cancel", CancelAsyncTask)
		asyncRouter.GET("/tasks/:id/content", GetAsyncTaskContent)
	}
	return engine, "sk-cavas"
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
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("kind", asyncTaskKindImage))
	require.NoError(t, writer.WriteField("action", asyncTaskActionEdit))
	require.NoError(t, writer.WriteField("model", "gemini-2.5-flash-image"))
	require.NoError(t, writer.WriteField("prompt", "edit this"))
	require.NoError(t, writer.WriteField("quality", "1K"))
	require.NoError(t, writer.WriteField("size", "1:1"))
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
			Parameters: map[string]interface{}{"n": 2, "quality": "1K", "size": "1:1"},
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

func TestOpenAIModelListIncludesGPTImage2(t *testing.T) {
	require.Contains(t, openai.ModelList, "gpt-image-2")
}
