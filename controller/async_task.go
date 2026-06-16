package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	asyncTaskKindImage       = "image"
	asyncTaskActionGenerate  = "generate"
	asyncTaskActionEdit      = "edit"
	asyncTaskStatusQueued    = "queued"
	asyncTaskStatusRunning   = "running"
	asyncTaskStatusSucceeded = "succeeded"
	asyncTaskStatusFailed    = "failed"
	asyncTaskStatusCanceled  = "canceled"
	asyncTaskStatusTimeout   = "timeout"
	asyncTaskPlatformOpenAI  = constant.TaskPlatform("openai-async")

	asyncTaskHTTPTimeout = 90 * time.Second
)

type asyncTaskRequest struct {
	Kind       string                 `json:"kind"`
	Action     string                 `json:"action"`
	Model      string                 `json:"model"`
	Input      asyncTaskInput         `json:"input"`
	Parameters map[string]interface{} `json:"parameters"`
}

type asyncTaskInput struct {
	Prompt string `json:"prompt"`
}

type asyncTaskResponse struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Action      string            `json:"action"`
	Model       string            `json:"model"`
	Status      string            `json:"status"`
	Progress    string            `json:"progress,omitempty"`
	Error       string            `json:"error,omitempty"`
	ChannelID   int               `json:"channelId,omitempty"`
	ChannelName string            `json:"channelName,omitempty"`
	Outputs     []asyncTaskOutput `json:"outputs,omitempty"`
	CreatedAt   int64             `json:"createdAt"`
	UpdatedAt   int64             `json:"updatedAt"`
	CompletedAt int64             `json:"completedAt,omitempty"`
}

type asyncTaskOutput struct {
	Index    int    `json:"index"`
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
	URL      string `json:"url,omitempty"`
}

type asyncTaskData struct {
	Kind    string                  `json:"kind"`
	Action  string                  `json:"action"`
	Model   string                  `json:"model"`
	Outputs []asyncTaskStoredOutput `json:"outputs,omitempty"`
}

type asyncTaskStoredOutput struct {
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
	URL      string `json:"url,omitempty"`
	Size     int    `json:"size,omitempty"`
}

type asyncTaskExecution struct {
	Request      asyncTaskRequest
	Multipart    *multipart.Form
	MultipartErr error
	RelayInfo    *relaycommon.RelayInfo
	Context      context.Context
}

func CreateAsyncTask(c *gin.Context) {
	request, err := readAsyncTaskCreateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	if request.Kind != asyncTaskKindImage {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "only image async tasks are supported in this version"}})
		return
	}
	channel, err := selectAsyncTaskChannel(c, request.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, request.Model); setupErr != nil {
		status := setupErr.StatusCode
		if status == 0 {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": gin.H{"message": setupErr.Error()}})
		return
	}
	relayInfo, priceErr := prepareAsyncTaskBilling(c, request, channel)
	if priceErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": priceErr.Error()}})
		return
	}

	now := time.Now().Unix()
	task := model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     c.GetInt("id"),
		Group:      common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		ChannelId:  channel.Id,
		Platform:   asyncTaskPlatformOpenAI,
		Action:     request.Action,
		Status:     model.TaskStatusInProgress,
		Progress:   "0%",
		SubmitTime: now,
		StartTime:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
		Properties: model.Properties{Input: request.Input.Prompt, OriginModelName: request.Model, UpstreamModelName: request.Model},
	}
	if relayInfo != nil {
		task.Quota = relayInfo.PriceData.Quota
		task.Group = relayInfo.UsingGroup
		task.Properties.OriginModelName = relayInfo.OriginModelName
		task.Properties.UpstreamModelName = firstAsyncNonEmpty(relayInfo.UpstreamModelName, relayInfo.OriginModelName)
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
	}
	task.SetData(asyncTaskData{Kind: request.Kind, Action: request.Action, Model: request.Model})
	if err := model.DB.Create(&task).Error; err != nil {
		// Task is not persisted yet, so background/sweeper paths cannot see it;
		// after persistence, refunds must go through task CAS + RefundTaskQuota.
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to create async task"}})
		return
	}
	service.LogTaskConsumption(c, relayInfo)

	execution := asyncTaskExecution{Request: request, Multipart: cloneAsyncMultipartForm(c.Request.MultipartForm), RelayInfo: relayInfo}
	startAsyncTaskExecution(task.TaskID, channel.Id, execution)
	c.JSON(http.StatusOK, asyncTaskModelToResponse(&task))
}

func GetAsyncTask(c *gin.Context) {
	task, ok := getUserAsyncTask(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, asyncTaskModelToResponse(task))
}

func CancelAsyncTask(c *gin.Context) {
	task, ok := getUserAsyncTask(c)
	if !ok {
		return
	}
	if task.Status != model.TaskStatusSuccess && task.Status != model.TaskStatusFailure {
		if !cancelAsyncTask(task) {
			reloaded, exists, err := model.GetByOnlyTaskId(task.TaskID)
			if err == nil && exists {
				task = reloaded
			}
		}
	}
	c.JSON(http.StatusOK, asyncTaskModelToResponse(task))
}

func GetAsyncTaskContent(c *gin.Context) {
	task, ok := getUserAsyncTask(c)
	if !ok {
		return
	}
	var data asyncTaskData
	_ = task.GetData(&data)
	if task.Status != model.TaskStatusSuccess || len(data.Outputs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "async task content not found"}})
		return
	}
	index, _ := strconv.Atoi(c.DefaultQuery("index", "0"))
	if index < 0 || index >= len(data.Outputs) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "async task content not found"}})
		return
	}
	if strings.TrimSpace(data.Outputs[index].URL) != "" {
		content, mimeType, err := downloadAsyncOutputURL(data.Outputs[index].URL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "failed to fetch async task content"}})
			return
		}
		c.Data(http.StatusOK, firstAsyncNonEmpty(data.Outputs[index].MimeType, mimeType, "application/octet-stream"), content)
		return
	}
	content, err := base64.StdEncoding.DecodeString(data.Outputs[index].Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "async task content is invalid"}})
		return
	}
	c.Data(http.StatusOK, firstAsyncNonEmpty(data.Outputs[index].MimeType, "application/octet-stream"), content)
}

func readAsyncTaskCreateRequest(c *gin.Context) (asyncTaskRequest, error) {
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		return readAsyncMultipartTaskRequest(c)
	}
	var request asyncTaskRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return asyncTaskRequest{}, err
	}
	return normalizeAsyncTaskRequest(request), nil
}

func readAsyncMultipartTaskRequest(c *gin.Context) (asyncTaskRequest, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return asyncTaskRequest{}, err
	}
	request := asyncTaskRequest{
		Kind:   c.PostForm("kind"),
		Action: c.PostForm("action"),
		Model:  c.PostForm("model"),
		Input:  asyncTaskInput{Prompt: c.PostForm("prompt")},
		Parameters: map[string]interface{}{
			"n":               asyncParamInt(c.PostForm("n")),
			"quality":         c.PostForm("quality"),
			"size":            c.PostForm("size"),
			"response_format": c.PostForm("response_format"),
			"output_format":   c.PostForm("output_format"),
		},
	}
	return normalizeAsyncTaskRequest(request), nil
}

func normalizeAsyncTaskRequest(request asyncTaskRequest) asyncTaskRequest {
	request.Kind = strings.ToLower(strings.TrimSpace(request.Kind))
	if request.Kind == "" {
		request.Kind = asyncTaskKindImage
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	if request.Action == "" {
		request.Action = asyncTaskActionGenerate
	}
	request.Model = strings.TrimSpace(request.Model)
	request.Input.Prompt = strings.TrimSpace(request.Input.Prompt)
	if request.Parameters == nil {
		request.Parameters = map[string]interface{}{}
	}
	return request
}

func selectAsyncTaskChannel(c *gin.Context, modelName string) (*model.Channel, error) {
	if strings.TrimSpace(modelName) == "" {
		return nil, errors.New("model is required")
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "" {
		group = "default"
	}
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:        c,
		ModelName:  modelName,
		TokenGroup: group,
		Retry:      common.GetPointer(0),
	})
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("no available channel for model %s", modelName)
	}
	return channel, nil
}

func prepareAsyncTaskBilling(c *gin.Context, request asyncTaskRequest, channel *model.Channel) (*relaycommon.RelayInfo, error) {
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if tokenGroup == "" {
		tokenGroup = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	relayInfo := &relaycommon.RelayInfo{
		UserId:          common.GetContextKeyInt(c, constant.ContextKeyUserId),
		UsingGroup:      common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		UserGroup:       common.GetContextKeyString(c, constant.ContextKeyUserGroup),
		UserQuota:       common.GetContextKeyInt(c, constant.ContextKeyUserQuota),
		UserEmail:       common.GetContextKeyString(c, constant.ContextKeyUserEmail),
		TokenId:         common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		TokenKey:        common.GetContextKeyString(c, constant.ContextKeyTokenKey),
		TokenUnlimited:  common.GetContextKeyBool(c, constant.ContextKeyTokenUnlimited),
		TokenGroup:      tokenGroup,
		OriginModelName: request.Model,
		ForcePreConsume: true,
		RequestURLPath:  c.Request.URL.String(),
		StartTime:       time.Now(),
		RelayFormat:     types.RelayFormatOpenAIImage,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: request.Action},
	}
	if userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		relayInfo.UserSetting = userSetting
	}
	relayInfo.RelayMode = relayconstant.RelayModeImagesGenerations
	if request.Action == asyncTaskActionEdit {
		relayInfo.RelayMode = relayconstant.RelayModeImagesEdits
	}
	relayInfo.InitChannelMeta(c)
	relayInfo.ChannelMeta.ChannelId = channel.Id
	relayInfo.ChannelMeta.UpstreamModelName = request.Model
	if err := helper.ModelMappedHelper(c, relayInfo, nil); err != nil {
		return nil, err
	}
	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		return nil, err
	}
	relayInfo.PriceData = priceData
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			return nil, apiErr
		}
	}
	return relayInfo, nil
}

func startAsyncTaskExecution(taskID string, channelID int, execution asyncTaskExecution) {
	asyncTaskRunnerMu.Lock()
	runner := asyncTaskRunner
	asyncTaskRunnerMu.Unlock()
	runner(taskID, channelID, execution)
}

var (
	asyncTaskRunnerMu sync.Mutex
	asyncTaskRunner   = func(taskID string, channelID int, execution asyncTaskExecution) {
		ctx, cancel := context.WithCancel(context.Background())
		execution.Context = ctx
		registerAsyncTaskCancel(taskID, cancel)
		go executeAsyncTaskInBackground(taskID, channelID, execution)
	}
	asyncTaskHTTPClient = &http.Client{Timeout: asyncTaskHTTPTimeout}

	asyncTaskCancelMu sync.Mutex
	asyncTaskCancels  = map[string]context.CancelFunc{}
)

func setAsyncTaskRunnerForTest(runner func(taskID string, channelID int, execution asyncTaskExecution)) func() {
	asyncTaskRunnerMu.Lock()
	previous := asyncTaskRunner
	asyncTaskRunner = runner
	asyncTaskRunnerMu.Unlock()
	return func() {
		asyncTaskRunnerMu.Lock()
		asyncTaskRunner = previous
		asyncTaskRunnerMu.Unlock()
	}
}

func executeAsyncTaskInBackground(taskID string, channelID int, execution asyncTaskExecution) {
	defer unregisterAsyncTaskCancel(taskID)
	task, exists, err := model.GetByOnlyTaskId(taskID)
	if err != nil || !exists {
		return
	}
	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		completeAsyncTaskFailure(task, execution.Request, "channel not found")
		return
	}
	outputs, err := executeAsyncImageTask(channel, execution)
	if err != nil {
		completeAsyncTaskFailure(task, execution.Request, safeAsyncTaskError(err))
		return
	}
	completeAsyncTaskSuccess(task, execution.Request, outputs)
}

func completeAsyncTaskSuccess(task *model.Task, request asyncTaskRequest, outputs []asyncTaskStoredOutput) {
	if asyncTaskIsTerminal(task.Status) {
		return
	}
	fromStatus := task.Status
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.SetData(asyncTaskData{Kind: request.Kind, Action: request.Action, Model: request.Model, Outputs: outputs})
	task.PrivateData.ResultURL = firstAsyncOutputURL(outputs)
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil || !won {
		return
	}
}

func completeAsyncTaskFailure(task *model.Task, request asyncTaskRequest, reason string) {
	if asyncTaskIsTerminal(task.Status) {
		return
	}
	fromStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = reason
	task.SetData(asyncTaskData{Kind: request.Kind, Action: request.Action, Model: request.Model})
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil || !won {
		return
	}
	service.RefundTaskQuota(context.Background(), task, reason)
}

func cancelAsyncTask(task *model.Task) bool {
	if asyncTaskIsTerminal(task.Status) {
		return false
	}
	fromStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FailReason = asyncTaskStatusCanceled
	task.FinishTime = time.Now().Unix()
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil || !won {
		return false
	}
	cancelAsyncTaskExecution(task.TaskID)
	service.RefundTaskQuota(context.Background(), task, asyncTaskStatusCanceled)
	return true
}

func registerAsyncTaskCancel(taskID string, cancel context.CancelFunc) {
	if taskID == "" || cancel == nil {
		return
	}
	asyncTaskCancelMu.Lock()
	asyncTaskCancels[taskID] = cancel
	asyncTaskCancelMu.Unlock()
}

func cancelAsyncTaskExecution(taskID string) {
	asyncTaskCancelMu.Lock()
	cancel := asyncTaskCancels[taskID]
	asyncTaskCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func unregisterAsyncTaskCancel(taskID string) {
	asyncTaskCancelMu.Lock()
	delete(asyncTaskCancels, taskID)
	asyncTaskCancelMu.Unlock()
}

func asyncTaskIsTerminal(status model.TaskStatus) bool {
	return status == model.TaskStatusSuccess || status == model.TaskStatusFailure
}

func SweepAsyncTimedOutTasksForTest(cutoffUnix int64, limit int) {
	sweepAsyncTimedOutTasks(context.Background(), cutoffUnix, limit)
}

func UpdateAsyncTaskBulk() {
	for {
		time.Sleep(15 * time.Second)
		if constant.TaskTimeoutMinutes <= 0 {
			continue
		}
		cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
		sweepAsyncTimedOutTasks(context.Background(), cutoff, constant.TaskQueryLimit)
	}
}

func sweepAsyncTimedOutTasks(ctx context.Context, cutoffUnix int64, limit int) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 100
	}
	var tasks []*model.Task
	if err := model.DB.Where("platform = ?", asyncTaskPlatformOpenAI).
		Where("progress != ?", "100%").
		Where("status NOT IN ?", []string{model.TaskStatusFailure, model.TaskStatusSuccess}).
		Where("submit_time < ?", cutoffUnix).
		Order("submit_time").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return 0
	}
	count := 0
	reason := "timeout"
	for _, task := range tasks {
		fromStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = time.Now().Unix()
		task.FailReason = reason
		won, err := task.UpdateWithStatus(fromStatus)
		if err != nil || !won {
			continue
		}
		cancelAsyncTaskExecution(task.TaskID)
		count++
		service.RefundTaskQuota(ctx, task, reason)
	}
	return count
}

func executeAsyncImageTask(channel *model.Channel, execution asyncTaskExecution) ([]asyncTaskStoredOutput, error) {
	execution.Request.Model = asyncTaskUpstreamModel(execution)
	ctx := execution.Context
	if ctx == nil {
		ctx = context.Background()
	}
	switch execution.Request.Action {
	case asyncTaskActionEdit:
		return executeAsyncImageEdit(ctx, channel, execution)
	case asyncTaskActionGenerate:
		return executeAsyncImageGeneration(ctx, channel, execution.Request)
	default:
		return nil, fmt.Errorf("unsupported image action %s", execution.Request.Action)
	}
}

func asyncTaskUpstreamModel(execution asyncTaskExecution) string {
	if execution.RelayInfo != nil && strings.TrimSpace(execution.RelayInfo.UpstreamModelName) != "" {
		return execution.RelayInfo.UpstreamModelName
	}
	return execution.Request.Model
}

func executeAsyncImageGeneration(parentCtx context.Context, channel *model.Channel, request asyncTaskRequest) ([]asyncTaskStoredOutput, error) {
	body, err := common.Marshal(asyncImageGenerationPayload(request))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeout)
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/v1/images/generations"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	return doAsyncImageRequest(upstreamReq)
}

func executeAsyncImageEdit(parentCtx context.Context, channel *model.Channel, execution asyncTaskExecution) ([]asyncTaskStoredOutput, error) {
	if execution.MultipartErr != nil {
		return nil, execution.MultipartErr
	}
	request := execution.Request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", request.Model)
	_ = writer.WriteField("prompt", request.Input.Prompt)
	for _, field := range []string{"n", "quality", "size", "response_format", "output_format"} {
		if value := asyncParamString(request.Parameters, field); value != "" {
			_ = writer.WriteField(field, value)
		}
	}
	if execution.Multipart == nil {
		return nil, errors.New("image file is required")
	}
	for _, header := range execution.Multipart.File["image"] {
		if err := copyAsyncMultipartFile(writer, "image", header); err != nil {
			return nil, err
		}
	}
	if header := firstAsyncFileHeader(execution.Multipart.File["mask"]); header != nil {
		if err := copyAsyncMultipartFile(writer, "mask", header); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeout)
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/v1/images/edits"), &body)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Content-Type", writer.FormDataContentType())
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	return doAsyncImageRequest(upstreamReq)
}

func asyncImageGenerationPayload(request asyncTaskRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"model":           request.Model,
		"prompt":          request.Input.Prompt,
		"n":               asyncParamIntValue(request.Parameters, "n", 1),
		"response_format": firstAsyncNonEmpty(asyncParamString(request.Parameters, "response_format"), "url"),
	}
	for _, field := range []string{"quality", "size", "output_format"} {
		if value, ok := request.Parameters[field]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			payload[field] = value
		}
	}
	return payload
}

func doAsyncImageRequest(request *http.Request) ([]asyncTaskStoredOutput, error) {
	response, err := asyncTaskHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream image task failed: %s", common.LocalLogPreview(string(body)))
	}
	var payload struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
		return nil, errors.New(payload.Error.Message)
	}
	outputs := make([]asyncTaskStoredOutput, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.URL) != "" {
			outputs = append(outputs, asyncTaskStoredOutput{MimeType: "image/png", URL: item.URL})
			continue
		}
		if strings.TrimSpace(item.B64JSON) != "" {
			return nil, errors.New("upstream image task returned base64 content; configure upstream response_format=url")
		}
	}
	if len(outputs) == 0 {
		return nil, errors.New("upstream image task returned no image")
	}
	return outputs, nil
}

func getUserAsyncTask(c *gin.Context) (*model.Task, bool) {
	taskID := c.Param("id")
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to get async task"}})
		return nil, false
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "async task not found"}})
		return nil, false
	}
	return task, true
}

func asyncTaskModelToResponse(task *model.Task) asyncTaskResponse {
	var data asyncTaskData
	_ = task.GetData(&data)
	outputs := make([]asyncTaskOutput, 0, len(data.Outputs))
	for index, output := range data.Outputs {
		size := output.Size
		if decoded, err := base64.StdEncoding.DecodeString(output.Content); err == nil {
			size = len(decoded)
		}
		outputs = append(outputs, asyncTaskOutput{Index: index, MimeType: output.MimeType, Size: size, URL: output.URL})
	}
	return asyncTaskResponse{
		ID:          task.TaskID,
		Kind:        data.Kind,
		Action:      data.Action,
		Model:       firstAsyncNonEmpty(data.Model, task.Properties.OriginModelName),
		Status:      asyncTaskStatusFromModelWithReason(task.Status, task.FailReason),
		Progress:    task.Progress,
		Error:       task.FailReason,
		ChannelID:   task.ChannelId,
		ChannelName: asyncTaskChannelName(task.ChannelId),
		Outputs:     outputs,
		CreatedAt:   firstAsyncTimestamp(task.CreatedAt, task.SubmitTime),
		UpdatedAt:   task.UpdatedAt,
		CompletedAt: task.FinishTime,
	}
}

func asyncTaskStatusFromModel(status model.TaskStatus) string {
	return asyncTaskStatusFromModelWithReason(status, "")
}

func asyncTaskStatusFromModelWithReason(status model.TaskStatus, failReason string) string {
	switch status {
	case model.TaskStatusSuccess:
		return asyncTaskStatusSucceeded
	case model.TaskStatusFailure:
		switch failReason {
		case asyncTaskStatusCanceled:
			return asyncTaskStatusCanceled
		case asyncTaskStatusTimeout:
			return asyncTaskStatusTimeout
		}
		return asyncTaskStatusFailed
	case model.TaskStatusQueued, model.TaskStatusSubmitted, model.TaskStatusNotStart:
		return asyncTaskStatusQueued
	default:
		return asyncTaskStatusRunning
	}
}

func asyncTaskChannelName(channelID int) string {
	channel, err := model.CacheGetChannel(channelID)
	if err == nil && channel != nil {
		return channel.Name
	}
	return ""
}

func asyncChannelURL(channel *model.Channel, path string) string {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL + path
}

func copyAsyncMultipartFile(writer *multipart.Writer, field string, header *multipart.FileHeader) error {
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	part, err := writer.CreateFormFile(field, filepath.Base(header.Filename))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func firstAsyncFileHeader(headers []*multipart.FileHeader) *multipart.FileHeader {
	if len(headers) == 0 {
		return nil
	}
	return headers[0]
}

func cloneAsyncMultipartForm(form *multipart.Form) *multipart.Form {
	if form == nil {
		return nil
	}
	cloned := &multipart.Form{Value: map[string][]string{}, File: map[string][]*multipart.FileHeader{}}
	for key, values := range form.Value {
		cloned.Value[key] = append([]string(nil), values...)
	}
	for key, headers := range form.File {
		cloned.File[key] = append([]*multipart.FileHeader(nil), headers...)
	}
	return cloned
}

func asyncParamString(parameters map[string]interface{}, key string) string {
	value, ok := parameters[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstAsyncNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstAsyncTimestamp(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstAsyncOutputURL(outputs []asyncTaskStoredOutput) string {
	for _, output := range outputs {
		if strings.TrimSpace(output.URL) != "" {
			return output.URL
		}
	}
	return ""
}

func asyncParamInt(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1
	}
	if value, err := strconv.Atoi(raw); err == nil && value > 0 {
		return value
	}
	return 1
}

func asyncParamIntValue(parameters map[string]interface{}, key string, fallback int) int {
	value, ok := parameters[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func safeAsyncTaskError(err error) string {
	if err == nil {
		return ""
	}
	return common.LocalLogPreview(err.Error())
}

func downloadAsyncOutputURL(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), asyncTaskHTTPTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	response, err := asyncTaskHTTPClient.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return nil, "", fmt.Errorf("failed to download async output: %d", response.StatusCode)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	return content, firstAsyncNonEmpty(response.Header.Get("Content-Type"), "application/octet-stream"), nil
}
