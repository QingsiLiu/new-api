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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
