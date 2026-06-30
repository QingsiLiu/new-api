package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
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
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	asyncTaskKindImage       = "image"
	asyncTaskKindVideo       = "video"
	asyncTaskActionGenerate  = "generate"
	asyncTaskActionEdit      = "edit"
	asyncTaskStatusQueued    = "queued"
	asyncTaskStatusRunning   = "running"
	asyncTaskStatusSucceeded = "succeeded"
	asyncTaskStatusFailed    = "failed"
	asyncTaskStatusCanceled  = "canceled"
	asyncTaskStatusTimeout   = "timeout"
	asyncTaskPlatformOpenAI  = constant.TaskPlatform("openai-async")

	asyncTaskHTTPTimeoutEnv            = "ASYNC_TASK_HTTP_TIMEOUT_SECONDS"
	asyncTaskMaxRunningEnv             = "ASYNC_TASK_MAX_RUNNING"
	asyncTaskMaxQueuedEnv              = "ASYNC_TASK_MAX_QUEUED"
	asyncTaskDefaultHTTPTimeoutSeconds = 300
	asyncTaskDefaultMaxRunning         = 2
	asyncTaskDefaultMaxQueued          = 100
	asyncTaskDefaultInlineContentLimit = 20 << 20
	asyncTaskKieSeedanceFastModel      = "bytedance/seedance-2-fast"
	asyncTaskKieSeedanceModel          = "bytedance/seedance-2"
)

type asyncTaskRequest struct {
	Kind           string                 `json:"kind"`
	Action         string                 `json:"action"`
	Model          string                 `json:"model"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Input          asyncTaskInput         `json:"input"`
	Parameters     map[string]interface{} `json:"parameters"`
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
	if len(request.IdempotencyKey) > 191 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "idempotency_key is too long"}})
		return
	}
	if !isSupportedAsyncTaskKind(request.Kind) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "unsupported async task kind"}})
		return
	}
	requestHash := asyncTaskRequestHash(request)
	unlockIdempotency := lockAsyncTaskIdempotency(c.GetInt("id"), request.IdempotencyKey)
	if unlockIdempotency != nil {
		defer unlockIdempotency()
		existing, exists, conflict, err := findExistingAsyncTaskByIdempotency(c.GetInt("id"), request.IdempotencyKey, requestHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check async task idempotency"}})
			return
		}
		if conflict {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "idempotency_key has already been used with a different request", "code": "idempotency_conflict"}})
			return
		}
		if exists {
			c.JSON(http.StatusOK, asyncTaskModelToResponse(existing))
			return
		}
	}
	if !asyncTaskSchedulerCanAccept() {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": "async task queue is full", "code": "queued_limit_exceeded"}})
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
		TaskID:         model.GenerateTaskID(),
		UserId:         c.GetInt("id"),
		Group:          common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		ChannelId:      channel.Id,
		Platform:       asyncTaskPlatformOpenAI,
		Action:         request.Action,
		IdempotencyKey: request.IdempotencyKey,
		RequestHash:    requestHash,
		Status:         model.TaskStatusQueued,
		Progress:       "0%",
		SubmitTime:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Properties:     model.Properties{Input: request.Input.Prompt, OriginModelName: request.Model, UpstreamModelName: request.Model},
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
	if err := startAsyncTaskExecution(task.TaskID, channel.Id, execution); err != nil {
		completeAsyncTaskFailure(&task, request, "queued_limit_exceeded")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": "async task queue is full", "code": "queued_limit_exceeded"}})
		return
	}
	c.JSON(http.StatusOK, asyncTaskModelToResponse(&task))
}

func GetAsyncTask(c *gin.Context) {
	task, ok := getUserAsyncTask(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, asyncTaskModelToResponse(task))
}

func GetAsyncTaskMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"runtime": asyncTaskRuntimeMetrics(),
		"tasks":   asyncTaskDatabaseMetrics(c.GetInt("id")),
	})
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
		content, mimeType, err := downloadAsyncTaskOutputURL(task, data.Outputs[index])
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": "failed to fetch async task content"}})
			return
		}
		c.Data(http.StatusOK, firstAsyncNonEmpty(mimeType, data.Outputs[index].MimeType, "application/octet-stream"), content)
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
	var request asyncTaskRequest
	var err error
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		request, err = readAsyncMultipartTaskRequest(c)
	} else {
		if err = common.DecodeJson(c.Request.Body, &request); err != nil {
			return asyncTaskRequest{}, err
		}
	}
	if err != nil {
		return asyncTaskRequest{}, err
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		request.IdempotencyKey = c.GetHeader("Idempotency-Key")
	}
	return normalizeAsyncTaskRequest(request), nil
}

func readAsyncMultipartTaskRequest(c *gin.Context) (asyncTaskRequest, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return asyncTaskRequest{}, err
	}
	request := asyncTaskRequest{
		Kind:           c.PostForm("kind"),
		Action:         c.PostForm("action"),
		Model:          c.PostForm("model"),
		IdempotencyKey: c.PostForm("idempotency_key"),
		Input:          asyncTaskInput{Prompt: c.PostForm("prompt")},
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
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Input.Prompt = strings.TrimSpace(request.Input.Prompt)
	if request.Parameters == nil {
		request.Parameters = map[string]interface{}{}
	}
	return request
}

func isSupportedAsyncTaskKind(kind string) bool {
	return kind == asyncTaskKindImage || kind == asyncTaskKindVideo
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
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: request.Action},
	}
	if userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting); ok {
		relayInfo.UserSetting = userSetting
	}
	switch request.Kind {
	case asyncTaskKindVideo:
		relayInfo.RelayFormat = types.RelayFormatTask
		relayInfo.RelayMode = relayconstant.RelayModeVideoSubmit
	default:
		relayInfo.RelayFormat = types.RelayFormatOpenAIImage
		relayInfo.RelayMode = relayconstant.RelayModeImagesGenerations
	}
	if request.Kind == asyncTaskKindImage && request.Action == asyncTaskActionEdit {
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
	if operation_setting.AsyncTaskSpecPricingEnabled {
		applyAsyncTaskSpecPricing(request, relayInfo)
	}
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, relayInfo.PriceData.Quota, relayInfo); apiErr != nil {
			return nil, apiErr
		}
	}
	return relayInfo, nil
}

func applyAsyncTaskSpecPricing(request asyncTaskRequest, relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil {
		return
	}
	baseQuota := relayInfo.PriceData.Quota
	if baseQuota <= 0 {
		baseQuota = relayInfo.PriceData.QuotaToPreConsume
	}
	if baseQuota <= 0 {
		return
	}

	multiplier := 1.0
	switch request.Kind {
	case asyncTaskKindVideo:
		resolution := asyncParamString(request.Parameters, "resolution")
		if resolution == "" {
			resolution = asyncParamString(request.Parameters, "size")
		}
		seconds := asyncParamIntValue(request.Parameters, "duration", 0)
		if seconds <= 0 {
			seconds = asyncParamIntValue(request.Parameters, "seconds", 0)
		}
		if seconds <= 0 {
			seconds = operation_setting.AsyncVideoBaseSeconds
		}
		multiplier = operation_setting.GetAsyncVideoSpecMultiplier(resolution, seconds)
		relayInfo.PriceData.AddOtherRatio("resolution", operation_setting.GetAsyncVideoResolutionMultiplier(resolution))
		relayInfo.PriceData.AddOtherRatio("seconds", operation_setting.GetAsyncVideoDurationMultiplier(seconds))
	default:
		quality := asyncParamString(request.Parameters, "quality")
		size := asyncParamString(request.Parameters, "size")
		count := asyncParamIntValue(request.Parameters, "n", 1)
		multiplier = operation_setting.GetAsyncImageSpecMultiplier(quality, size, count)
		relayInfo.PriceData.AddOtherRatio("quality", operation_setting.GetAsyncImageQualityMultiplier(quality))
		relayInfo.PriceData.AddOtherRatio("size", operation_setting.GetAsyncImageSizeMultiplier(size))
		relayInfo.PriceData.AddOtherRatio("n", operation_setting.GetAsyncImageCountMultiplier(count))
	}

	if multiplier <= 0 {
		return
	}

	relayInfo.PriceData.Quota = int(math.Round(float64(baseQuota) * multiplier))
	relayInfo.PriceData.QuotaToPreConsume = relayInfo.PriceData.Quota
}

func startAsyncTaskExecution(taskID string, channelID int, execution asyncTaskExecution) error {
	asyncTaskRunnerMu.Lock()
	runner := asyncTaskRunner
	asyncTaskRunnerMu.Unlock()
	return runner(taskID, channelID, execution)
}

var (
	asyncTaskRunnerMu sync.Mutex
	asyncTaskRunner   = func(taskID string, channelID int, execution asyncTaskExecution) error {
		return currentAsyncTaskScheduler().Schedule(asyncTaskQueuedExecution{TaskID: taskID, ChannelID: channelID, Execution: execution})
	}
	asyncTaskHTTPClient         = newAsyncTaskHTTPClient()
	asyncTaskInlineContentLimit = asyncTaskDefaultInlineContentLimit

	asyncTaskCancelMu sync.Mutex
	asyncTaskCancels  = map[string]context.CancelFunc{}

	asyncTaskSchedulerMu       sync.Mutex
	asyncTaskSchedulerInstance *asyncTaskExecutionScheduler

	asyncTaskIdempotencyMu    sync.Mutex
	asyncTaskIdempotencyLocks = map[string]*sync.Mutex{}
)

func newAsyncTaskHTTPClient() *http.Client {
	return &http.Client{Timeout: asyncTaskHTTPTimeoutDuration()}
}

func asyncTaskHTTPTimeoutDuration() time.Duration {
	seconds := common.GetEnvOrDefault(asyncTaskHTTPTimeoutEnv, asyncTaskDefaultHTTPTimeoutSeconds)
	if seconds <= 0 {
		seconds = asyncTaskDefaultHTTPTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func setAsyncTaskRunnerForTest(runner func(taskID string, channelID int, execution asyncTaskExecution) error) func() {
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

func setLegacyAsyncTaskRunnerForTest(runner func(taskID string, channelID int, execution asyncTaskExecution)) func() {
	return setAsyncTaskRunnerForTest(func(taskID string, channelID int, execution asyncTaskExecution) error {
		runner(taskID, channelID, execution)
		return nil
	})
}

func setAsyncTaskSchedulerForTest(maxRunning int, maxQueued int) func() {
	replacement := newAsyncTaskExecutionScheduler(maxRunning, maxQueued)
	asyncTaskSchedulerMu.Lock()
	previous := asyncTaskSchedulerInstance
	asyncTaskSchedulerInstance = replacement
	asyncTaskSchedulerMu.Unlock()
	return func() {
		asyncTaskSchedulerMu.Lock()
		if asyncTaskSchedulerInstance == replacement {
			asyncTaskSchedulerInstance = previous
		}
		asyncTaskSchedulerMu.Unlock()
		replacement.Stop()
	}
}

func resetAsyncTaskSchedulerForTest() {
	asyncTaskSchedulerMu.Lock()
	previous := asyncTaskSchedulerInstance
	asyncTaskSchedulerInstance = newAsyncTaskExecutionScheduler(asyncTaskDefaultMaxRunning, asyncTaskDefaultMaxQueued)
	asyncTaskSchedulerMu.Unlock()
	if previous != nil {
		previous.Stop()
	}
}

type asyncTaskQueuedExecution struct {
	TaskID    string
	ChannelID int
	Execution asyncTaskExecution
}

type asyncTaskExecutionScheduler struct {
	maxRunning int
	maxQueued  int
	queue      chan asyncTaskQueuedExecution
	stop       chan struct{}
	stopOnce   sync.Once
	mu         sync.Mutex
	running    int
}

type asyncTaskRuntimeMetricsData struct {
	Running    int `json:"running"`
	Queued     int `json:"queued"`
	MaxRunning int `json:"maxRunning"`
	MaxQueued  int `json:"maxQueued"`
}

type asyncTaskDatabaseMetricsData struct {
	Queued    int64 `json:"queued"`
	Running   int64 `json:"running"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
	Timeout   int64 `json:"timeout"`
	Total     int64 `json:"total"`
}

func currentAsyncTaskScheduler() *asyncTaskExecutionScheduler {
	asyncTaskSchedulerMu.Lock()
	defer asyncTaskSchedulerMu.Unlock()
	if asyncTaskSchedulerInstance == nil {
		asyncTaskSchedulerInstance = newAsyncTaskExecutionScheduler(
			common.GetEnvOrDefault(asyncTaskMaxRunningEnv, asyncTaskDefaultMaxRunning),
			common.GetEnvOrDefault(asyncTaskMaxQueuedEnv, asyncTaskDefaultMaxQueued),
		)
	}
	return asyncTaskSchedulerInstance
}

func newAsyncTaskExecutionScheduler(maxRunning int, maxQueued int) *asyncTaskExecutionScheduler {
	if maxRunning <= 0 {
		maxRunning = asyncTaskDefaultMaxRunning
	}
	if maxQueued <= 0 {
		maxQueued = asyncTaskDefaultMaxQueued
	}
	scheduler := &asyncTaskExecutionScheduler{
		maxRunning: maxRunning,
		maxQueued:  maxQueued,
		queue:      make(chan asyncTaskQueuedExecution, maxQueued),
		stop:       make(chan struct{}),
	}
	for i := 0; i < maxRunning; i++ {
		go scheduler.worker()
	}
	return scheduler
}

func (scheduler *asyncTaskExecutionScheduler) Schedule(job asyncTaskQueuedExecution) error {
	select {
	case <-scheduler.stop:
		return errors.New("async task scheduler stopped")
	default:
	}
	select {
	case scheduler.queue <- job:
		return nil
	default:
		return errors.New("queued_limit_exceeded")
	}
}

func (scheduler *asyncTaskExecutionScheduler) CanAccept() bool {
	return len(scheduler.queue) < cap(scheduler.queue)
}

func (scheduler *asyncTaskExecutionScheduler) Metrics() asyncTaskRuntimeMetricsData {
	scheduler.mu.Lock()
	running := scheduler.running
	scheduler.mu.Unlock()
	return asyncTaskRuntimeMetricsData{
		Running:    running,
		Queued:     len(scheduler.queue),
		MaxRunning: scheduler.maxRunning,
		MaxQueued:  scheduler.maxQueued,
	}
}

func (scheduler *asyncTaskExecutionScheduler) Stop() {
	scheduler.stopOnce.Do(func() {
		close(scheduler.stop)
	})
}

func (scheduler *asyncTaskExecutionScheduler) worker() {
	for {
		select {
		case <-scheduler.stop:
			return
		case job := <-scheduler.queue:
			scheduler.mu.Lock()
			scheduler.running++
			scheduler.mu.Unlock()
			scheduler.run(job)
			scheduler.mu.Lock()
			scheduler.running--
			scheduler.mu.Unlock()
		}
	}
}

func (scheduler *asyncTaskExecutionScheduler) run(job asyncTaskQueuedExecution) {
	ctx, cancel := context.WithCancel(context.Background())
	job.Execution.Context = ctx
	registerAsyncTaskCancel(job.TaskID, cancel)
	defer cancel()
	executeAsyncTaskInBackground(job.TaskID, job.ChannelID, job.Execution)
}

func asyncTaskSchedulerCanAccept() bool {
	return currentAsyncTaskScheduler().CanAccept()
}

func asyncTaskRuntimeMetrics() asyncTaskRuntimeMetricsData {
	return currentAsyncTaskScheduler().Metrics()
}

func asyncTaskDatabaseMetrics(userID int) asyncTaskDatabaseMetricsData {
	return asyncTaskDatabaseMetricsData{
		Queued:    countAsyncTasksByStatus(userID, []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusSubmitted, model.TaskStatusNotStart}, ""),
		Running:   countAsyncTasksByStatus(userID, []model.TaskStatus{model.TaskStatusInProgress}, ""),
		Succeeded: countAsyncTasksByStatus(userID, []model.TaskStatus{model.TaskStatusSuccess}, ""),
		Failed:    countAsyncTasksByStatus(userID, []model.TaskStatus{model.TaskStatusFailure}, ""),
		Timeout:   countAsyncTasksByStatus(userID, []model.TaskStatus{model.TaskStatusFailure}, asyncTaskStatusTimeout),
		Total:     countAsyncTasksByStatus(userID, nil, ""),
	}
}

func countAsyncTasksByStatus(userID int, statuses []model.TaskStatus, failReason string) int64 {
	if model.DB == nil {
		return 0
	}
	query := model.DB.Model(&model.Task{}).Where("platform = ?", asyncTaskPlatformOpenAI)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	if failReason != "" {
		query = query.Where("fail_reason = ?", failReason)
	}
	var count int64
	_ = query.Count(&count).Error
	return count
}

func lockAsyncTaskIdempotency(userID int, idempotencyKey string) func() {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil
	}
	key := fmt.Sprintf("%d:%s", userID, idempotencyKey)
	asyncTaskIdempotencyMu.Lock()
	lock := asyncTaskIdempotencyLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		asyncTaskIdempotencyLocks[key] = lock
	}
	asyncTaskIdempotencyMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func findExistingAsyncTaskByIdempotency(userID int, idempotencyKey string, requestHash string) (*model.Task, bool, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, false, false, nil
	}
	var tasks []model.Task
	err := model.DB.Where("user_id = ? AND platform = ? AND idempotency_key = ?", userID, asyncTaskPlatformOpenAI, idempotencyKey).
		Order("created_at DESC").
		Limit(1).
		Find(&tasks).Error
	if err != nil {
		return nil, false, false, err
	}
	if len(tasks) == 0 {
		return nil, false, false, nil
	}
	task := tasks[0]
	if strings.TrimSpace(task.RequestHash) != requestHash {
		return &task, true, true, nil
	}
	return &task, true, false, nil
}

func asyncTaskRequestHash(request asyncTaskRequest) string {
	fingerprint := struct {
		Kind       string                 `json:"kind"`
		Action     string                 `json:"action"`
		Model      string                 `json:"model"`
		Input      asyncTaskInput         `json:"input"`
		Parameters map[string]interface{} `json:"parameters"`
	}{
		Kind:       request.Kind,
		Action:     request.Action,
		Model:      request.Model,
		Input:      request.Input,
		Parameters: request.Parameters,
	}
	payload, err := common.Marshal(fingerprint)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(common.Sha256Raw(payload))
}

func setAsyncTaskInlineContentLimitForTest(limit int) func() {
	previous := asyncTaskInlineContentLimit
	asyncTaskInlineContentLimit = limit
	return func() {
		asyncTaskInlineContentLimit = previous
	}
}

func executeAsyncTaskInBackground(taskID string, channelID int, execution asyncTaskExecution) {
	defer unregisterAsyncTaskCancel(taskID)
	task, exists, err := model.GetByOnlyTaskId(taskID)
	if err != nil || !exists {
		return
	}
	if asyncTaskIsTerminal(task.Status) {
		return
	}
	task, ok := markAsyncTaskRunning(task)
	if !ok {
		return
	}
	channel, err := model.CacheGetChannel(channelID)
	if err != nil || channel == nil {
		completeAsyncTaskFailure(task, execution.Request, "channel not found")
		return
	}
	var outputs []asyncTaskStoredOutput
	switch execution.Request.Kind {
	case asyncTaskKindImage:
		outputs, err = executeAsyncImageTask(channel, execution)
	case asyncTaskKindVideo:
		outputs, err = executeAsyncVideoTask(task, channel, execution)
	default:
		err = fmt.Errorf("unsupported async task kind %s", execution.Request.Kind)
	}
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

func markAsyncTaskRunning(task *model.Task) (*model.Task, bool) {
	if task.Status == model.TaskStatusInProgress {
		return task, true
	}
	if task.Status != model.TaskStatusQueued && task.Status != model.TaskStatusSubmitted && task.Status != model.TaskStatusNotStart {
		return task, true
	}
	fromStatus := task.Status
	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.UpdatedAt = task.StartTime
	task.Progress = "0%"
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil || !won {
		reloaded, exists, reloadErr := model.GetByOnlyTaskId(task.TaskID)
		if reloadErr != nil || !exists || asyncTaskIsTerminal(reloaded.Status) {
			return nil, false
		}
		return reloaded, reloaded.Status == model.TaskStatusInProgress
	}
	return task, true
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

func executeAsyncVideoTask(task *model.Task, channel *model.Channel, execution asyncTaskExecution) ([]asyncTaskStoredOutput, error) {
	execution.Request.Model = asyncTaskUpstreamModel(execution)
	ctx := execution.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if execution.Request.Action != asyncTaskActionGenerate {
		return nil, fmt.Errorf("unsupported video action %s", execution.Request.Action)
	}
	switch channel.Type {
	case constant.ChannelTypeKie:
		if !isAsyncKieSeedanceModel(execution.Request.Model) {
			return nil, fmt.Errorf("unsupported KIE video model %s", execution.Request.Model)
		}
		return executeAsyncKieSeedanceVideoTask(ctx, task, channel, execution.Request)
	case constant.ChannelTypeJimengOpenAIVideo:
		return executeAsyncOpenAIVideoTask(ctx, task, channel, execution.Request)
	default:
		return nil, fmt.Errorf("unsupported video async channel type %s", constant.GetChannelTypeName(channel.Type))
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
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/v1/images/generations"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	return doAsyncImageRequest(upstreamReq, asyncImageOutputMimeType(request.Parameters))
}

func executeAsyncImageEdit(parentCtx context.Context, channel *model.Channel, execution asyncTaskExecution) ([]asyncTaskStoredOutput, error) {
	if execution.MultipartErr != nil {
		return nil, execution.MultipartErr
	}
	request := execution.Request
	if shouldExecuteAsyncGeminiImageEdit(channel, request.Model) {
		return executeAsyncGeminiImageEdit(parentCtx, channel, execution)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", request.Model)
	_ = writer.WriteField("prompt", request.Input.Prompt)
	_ = writer.WriteField("response_format", "url")
	for _, field := range []string{"n", "quality", "size", "output_format"} {
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
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/v1/images/edits"), &body)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Content-Type", writer.FormDataContentType())
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	return doAsyncImageRequest(upstreamReq, asyncImageOutputMimeType(request.Parameters))
}

func shouldExecuteAsyncGeminiImageEdit(channel *model.Channel, modelName string) bool {
	return channel != nil &&
		channel.Type == constant.ChannelTypeGemini &&
		model_setting.IsGeminiModelSupportImagine(modelName) &&
		!strings.HasPrefix(modelName, "imagen")
}

func executeAsyncGeminiImageEdit(parentCtx context.Context, channel *model.Channel, execution asyncTaskExecution) ([]asyncTaskStoredOutput, error) {
	if execution.Multipart == nil {
		return nil, errors.New("image file is required")
	}
	request := execution.Request
	payload, err := asyncGeminiImageEditPayload(request, execution.Multipart)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncGeminiGenerateContentURL(channel, request.Model), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("x-goog-api-key", channel.Key)
	return doAsyncGeminiImageRequest(upstreamReq)
}

func asyncGeminiImageEditPayload(request asyncTaskRequest, form *multipart.Form) (dto.GeminiChatRequest, error) {
	parts := []dto.GeminiPart{{Text: request.Input.Prompt}}
	imageFiles := asyncMultipartImageFiles(form)
	if len(imageFiles) == 0 {
		return dto.GeminiChatRequest{}, errors.New("image file is required")
	}
	for _, header := range imageFiles {
		part, err := asyncGeminiMultipartImagePart(header)
		if err != nil {
			return dto.GeminiChatRequest{}, err
		}
		parts = append(parts, part)
	}
	imageConfig, err := asyncGeminiImageConfig(request.Parameters)
	if err != nil {
		return dto.GeminiChatRequest{}, err
	}
	return dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role:  "user",
				Parts: parts,
			},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			CandidateCount:     common.GetPointer(asyncParamIntValue(request.Parameters, "n", 1)),
			ImageConfig:        imageConfig,
		},
	}, nil
}

func asyncMultipartImageFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil || form.File == nil {
		return nil
	}
	files := append([]*multipart.FileHeader(nil), form.File["image"]...)
	if len(files) > 0 {
		return files
	}
	files = append(files, form.File["image[]"]...)
	for fieldName, headers := range form.File {
		if strings.HasPrefix(fieldName, "image[") {
			files = append(files, headers...)
		}
	}
	return files
}

func asyncGeminiMultipartImagePart(header *multipart.FileHeader) (dto.GeminiPart, error) {
	file, err := header.Open()
	if err != nil {
		return dto.GeminiPart{}, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return dto.GeminiPart{}, err
	}
	contentType := asyncMultipartFileContentType(header, content)
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return dto.GeminiPart{}, fmt.Errorf("mime type is not supported by Gemini image edit: %s", contentType)
	}
	return dto.GeminiPart{
		InlineData: &dto.GeminiInlineData{
			MimeType: contentType,
			Data:     base64.StdEncoding.EncodeToString(content),
		},
	}, nil
}

func asyncGeminiImageConfig(parameters map[string]interface{}) (jsonBytes []byte, err error) {
	config := map[string]string{}
	if aspectRatio := asyncGeminiAspectRatio(asyncParamString(parameters, "size")); aspectRatio != "" {
		config["aspectRatio"] = aspectRatio
	}
	if imageSize := asyncGeminiImageSize(asyncParamString(parameters, "quality")); imageSize != "" {
		config["imageSize"] = imageSize
	}
	if len(config) == 0 {
		return nil, nil
	}
	return common.Marshal(config)
}

func asyncGeminiAspectRatio(size string) string {
	size = strings.TrimSpace(size)
	if size == "" || strings.EqualFold(size, "auto") {
		return ""
	}
	if strings.Contains(size, ":") {
		return size
	}
	switch size {
	case "256x256", "512x512", "1024x1024":
		return "1:1"
	case "1536x1024", "2048x1360", "3840x2560":
		return "3:2"
	case "1024x1536", "1360x2048", "2560x3840":
		return "2:3"
	case "1360x1024", "2048x1536", "3840x2880":
		return "4:3"
	case "1024x1360", "1536x2048", "2880x3840":
		return "3:4"
	case "1792x1024", "2048x1152", "3840x2160":
		return "16:9"
	case "1024x1792", "1152x2048", "2160x3840":
		return "9:16"
	case "2048x2048", "3840x3840":
		return "1:1"
	}
	return ""
}

func asyncGeminiImageSize(quality string) string {
	switch strings.TrimSpace(quality) {
	case "4K", "high":
		return "4K"
	case "2K", "hd", "medium":
		return "2K"
	case "1K", "standard", "low", "auto":
		return "1K"
	default:
		return ""
	}
}

func asyncGeminiGenerateContentURL(channel *model.Channel, modelName string) string {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") || strings.HasSuffix(strings.ToLower(baseURL), "/v1beta") {
		baseURL = baseURL[:strings.LastIndex(baseURL, "/")]
	}
	version := model_setting.GetGeminiVersionSetting(modelName)
	return fmt.Sprintf("%s/%s/models/%s:generateContent", baseURL, version, modelName)
}

func asyncImageGenerationPayload(request asyncTaskRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"model":           request.Model,
		"prompt":          request.Input.Prompt,
		"n":               asyncParamIntValue(request.Parameters, "n", 1),
		"response_format": "url",
	}
	for _, field := range []string{"quality", "size", "output_format"} {
		if value, ok := request.Parameters[field]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			payload[field] = value
		}
	}
	return payload
}

func asyncImageOutputMimeType(parameters map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(asyncParamString(parameters, "output_format"))) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func doAsyncImageRequest(request *http.Request, defaultMimeType string) ([]asyncTaskStoredOutput, error) {
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
		if url := strings.TrimSpace(item.URL); url != "" {
			if output, ok, err := asyncStoredOutputFromDataURL(url, defaultMimeType); ok || err != nil {
				if err != nil {
					return nil, err
				}
				outputs = append(outputs, output)
				continue
			}
			outputs = append(outputs, asyncTaskStoredOutput{MimeType: defaultMimeType, URL: url})
			continue
		}
		if encoded := strings.TrimSpace(item.B64JSON); encoded != "" {
			content, err := decodeAsyncInlineBase64(encoded)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, asyncTaskStoredOutput{MimeType: defaultMimeType, Content: encoded, Size: len(content)})
		}
	}
	if len(outputs) == 0 {
		return nil, errors.New("upstream image task returned no image")
	}
	return outputs, nil
}

type asyncKieCreateTaskResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Error string `json:"error"`
	Data  struct {
		TaskID string `json:"taskId"`
		ID     string `json:"id"`
	} `json:"data"`
}

type asyncKieRecordInfoResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Error string `json:"error"`
	Data  struct {
		TaskID     string   `json:"taskId"`
		ID         string   `json:"id"`
		State      string   `json:"state"`
		Status     string   `json:"status"`
		FailReason string   `json:"failReason"`
		Error      string   `json:"error"`
		ResultJSON string   `json:"resultJson"`
		ResultURLs []string `json:"resultUrls"`
		VideoURL   string   `json:"videoUrl"`
		OutputURL  string   `json:"outputUrl"`
		Response   struct {
			ResultURLs []string `json:"resultUrls"`
			VideoURL   string   `json:"videoUrl"`
			OutputURL  string   `json:"outputUrl"`
		} `json:"response"`
	} `json:"data"`
}

type asyncOpenAIVideoTaskResponse struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Error      any    `json:"error"`
	Message    string `json:"message"`
	FailReason string `json:"fail_reason"`
}

func isAsyncKieSeedanceModel(modelName string) bool {
	switch strings.TrimSpace(modelName) {
	case asyncTaskKieSeedanceFastModel, asyncTaskKieSeedanceModel:
		return true
	default:
		return false
	}
}

func executeAsyncOpenAIVideoTask(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) ([]asyncTaskStoredOutput, error) {
	taskID, err := createAsyncOpenAIVideoTask(parentCtx, channel, request)
	if err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, errors.New("OpenAI video task returned no task id")
	}
	persistAsyncTaskUpstreamTaskID(task, taskID)
	for {
		record, err := pollAsyncOpenAIVideoTask(parentCtx, channel, taskID)
		if err != nil {
			return nil, err
		}
		status := strings.ToLower(strings.TrimSpace(record.Status))
		if asyncOpenAIVideoTaskSucceeded(status) {
			return []asyncTaskStoredOutput{{
				MimeType: "video/mp4",
				URL:      asyncChannelURL(channel, "/v1/videos/"+url.PathEscape(taskID)+"/content"),
			}}, nil
		}
		if asyncOpenAIVideoTaskFailed(status) {
			return nil, errors.New(firstAsyncNonEmpty(asyncOpenAIVideoErrorMessage(record.Error), record.FailReason, record.Message, "OpenAI video task failed"))
		}
		select {
		case <-parentCtx.Done():
			return nil, parentCtx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func createAsyncOpenAIVideoTask(parentCtx context.Context, channel *model.Channel, request asyncTaskRequest) (string, error) {
	body, err := common.Marshal(asyncOpenAIVideoPayload(request))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/v1/videos"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	response, err := asyncTaskHTTPClient.Do(upstreamReq)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("upstream OpenAI video task failed: %s", common.LocalLogPreview(string(responseBody)))
	}
	var payload asyncOpenAIVideoTaskResponse
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}
	if msg := firstAsyncNonEmpty(asyncOpenAIVideoErrorMessage(payload.Error), payload.FailReason, payload.Message); msg != "" && asyncOpenAIVideoTaskFailed(payload.Status) {
		return "", errors.New(msg)
	}
	return firstAsyncNonEmpty(payload.ID, payload.TaskID), nil
}

func pollAsyncOpenAIVideoTask(parentCtx context.Context, channel *model.Channel, taskID string) (asyncOpenAIVideoTaskResponse, error) {
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, asyncChannelURL(channel, "/v1/videos/"+url.PathEscape(taskID)), nil)
	if err != nil {
		return asyncOpenAIVideoTaskResponse{}, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	response, err := asyncTaskHTTPClient.Do(upstreamReq)
	if err != nil {
		return asyncOpenAIVideoTaskResponse{}, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return asyncOpenAIVideoTaskResponse{}, fmt.Errorf("upstream OpenAI video task poll failed: %s", common.LocalLogPreview(string(responseBody)))
	}
	var payload asyncOpenAIVideoTaskResponse
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return asyncOpenAIVideoTaskResponse{}, err
	}
	return payload, nil
}

func asyncOpenAIVideoPayload(request asyncTaskRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"model":        request.Model,
		"prompt":       asyncVideoPrompt(request),
		"seconds":      asyncOpenAIVideoSeconds(request.Parameters),
		"aspect_ratio": asyncOpenAIVideoAspectRatio(request.Parameters),
	}
	if reference := asyncOpenAIVideoInputReference(request.Parameters); reference != "" {
		payload["input_reference"] = reference
	}
	return payload
}

func asyncVideoPrompt(request asyncTaskRequest) string {
	if prompt := strings.TrimSpace(request.Input.Prompt); prompt != "" {
		return prompt
	}
	prompt, _, _, _ := asyncKieSeedanceContentParts(request.Parameters["content"])
	return prompt
}

func asyncOpenAIVideoSeconds(parameters map[string]interface{}) string {
	if value := asyncParamString(parameters, "seconds"); value != "" {
		return value
	}
	if duration := asyncParamIntValue(parameters, "duration", 0); duration > 0 {
		return strconv.Itoa(duration)
	}
	return "5"
}

func asyncOpenAIVideoAspectRatio(parameters map[string]interface{}) string {
	if value := asyncParamString(parameters, "aspect_ratio"); value != "" {
		return value
	}
	if value := asyncParamString(parameters, "ratio"); value != "" {
		return value
	}
	size := strings.ToLower(strings.TrimSpace(asyncParamString(parameters, "size")))
	switch size {
	case "1024x1024":
		return "1:1"
	case "720x1280", "1024x1792":
		return "9:16"
	case "1280x720", "1792x1024":
		return "16:9"
	default:
		return "16:9"
	}
}

func asyncOpenAIVideoInputReference(parameters map[string]interface{}) string {
	for _, key := range []string{"input_reference", "image", "image_url"} {
		if value := asyncParamString(parameters, key); value != "" {
			return value
		}
	}
	_, imageURLs, _, _ := asyncKieSeedanceContentParts(parameters["content"])
	if len(imageURLs) > 0 {
		return imageURLs[0]
	}
	return ""
}

func asyncOpenAIVideoTaskSucceeded(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "succeeded" || status == "success" || status == "completed" || status == "complete" || status == "finished"
}

func asyncOpenAIVideoTaskFailed(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "failed" || status == "fail" || status == "error" || status == "canceled" || status == "cancelled" || status == "timeout" || status == "expired"
}

func asyncOpenAIVideoErrorMessage(raw any) string {
	switch typed := raw.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		return firstAsyncNonEmpty(strings.TrimSpace(fmt.Sprint(typed["message"])), strings.TrimSpace(fmt.Sprint(typed["error"])), strings.TrimSpace(fmt.Sprint(typed["code"])))
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func executeAsyncKieSeedanceVideoTask(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) ([]asyncTaskStoredOutput, error) {
	taskID, err := createAsyncKieVideoTask(parentCtx, channel, request)
	if err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, errors.New("KIE video task returned no task id")
	}
	persistAsyncTaskUpstreamTaskID(task, taskID)
	for {
		record, err := pollAsyncKieVideoTask(parentCtx, channel, taskID)
		if err != nil {
			return nil, err
		}
		status := asyncKieTaskStatus(record)
		if asyncKieTaskSucceeded(status) {
			outputs := asyncKieVideoOutputs(record)
			if len(outputs) == 0 {
				return nil, errors.New("KIE video task succeeded without video URL")
			}
			return outputs, nil
		}
		if asyncKieTaskFailed(status) {
			return nil, errors.New(firstAsyncNonEmpty(record.Data.FailReason, record.Data.Error, record.Msg, "KIE video task failed"))
		}
		select {
		case <-parentCtx.Done():
			return nil, parentCtx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func createAsyncKieVideoTask(parentCtx context.Context, channel *model.Channel, request asyncTaskRequest) (string, error) {
	body, err := common.Marshal(asyncKieSeedancePayload(request))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, asyncChannelURL(channel, "/api/v1/jobs/createTask"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	response, err := asyncTaskHTTPClient.Do(upstreamReq)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("upstream KIE video task failed: %s", common.LocalLogPreview(string(responseBody)))
	}
	var payload asyncKieCreateTaskResponse
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return "", err
	}
	if payload.Code != 0 && payload.Code != 200 {
		return "", errors.New(firstAsyncNonEmpty(payload.Error, payload.Msg, "KIE video task create failed"))
	}
	return firstAsyncNonEmpty(payload.Data.TaskID, payload.Data.ID), nil
}

func pollAsyncKieVideoTask(parentCtx context.Context, channel *model.Channel, taskID string) (asyncKieRecordInfoResponse, error) {
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	query := url.Values{"taskId": []string{taskID}}
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, asyncChannelURL(channel, "/api/v1/jobs/recordInfo")+"?"+query.Encode(), nil)
	if err != nil {
		return asyncKieRecordInfoResponse{}, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.Key)
	response, err := asyncTaskHTTPClient.Do(upstreamReq)
	if err != nil {
		return asyncKieRecordInfoResponse{}, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return asyncKieRecordInfoResponse{}, fmt.Errorf("upstream KIE video task poll failed: %s", common.LocalLogPreview(string(responseBody)))
	}
	var payload asyncKieRecordInfoResponse
	if err := common.Unmarshal(responseBody, &payload); err != nil {
		return asyncKieRecordInfoResponse{}, err
	}
	if payload.Code != 0 && payload.Code != 200 {
		return asyncKieRecordInfoResponse{}, errors.New(firstAsyncNonEmpty(payload.Error, payload.Msg, "KIE video task poll failed"))
	}
	return payload, nil
}

func asyncKieSeedancePayload(request asyncTaskRequest) map[string]interface{} {
	input := map[string]interface{}{"prompt": request.Input.Prompt}
	contentPrompt, imageURLs, videoURLs, audioURLs := asyncKieSeedanceContentParts(request.Parameters["content"])
	if strings.TrimSpace(request.Input.Prompt) == "" && contentPrompt != "" {
		input["prompt"] = contentPrompt
	}
	if len(imageURLs) > 0 {
		input["reference_image_urls"] = imageURLs
	}
	if len(videoURLs) > 0 {
		input["reference_video_urls"] = videoURLs
	}
	if len(audioURLs) > 0 {
		input["reference_audio_urls"] = audioURLs
	}
	if value := asyncParamString(request.Parameters, "ratio"); value != "" {
		input["aspect_ratio"] = value
	}
	for _, field := range []string{"resolution", "duration", "generate_audio", "watermark"} {
		if value, ok := request.Parameters[field]; ok {
			input[field] = value
		}
	}
	return map[string]interface{}{
		"model": request.Model,
		"input": input,
	}
}

func asyncKieSeedanceContentParts(raw interface{}) (string, []string, []string, []string) {
	items, ok := raw.([]interface{})
	if !ok {
		return "", nil, nil, nil
	}
	var prompt string
	var imageURLs []string
	var videoURLs []string
	var audioURLs []string
	for _, item := range items {
		part, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(part["type"]))) {
		case "text":
			if text := strings.TrimSpace(fmt.Sprint(part["text"])); text != "" && prompt == "" {
				prompt = text
			}
		case "image_url":
			if url := asyncKieNestedURL(part["image_url"]); url != "" {
				imageURLs = append(imageURLs, url)
			}
		case "video_url":
			if url := asyncKieNestedURL(part["video_url"]); url != "" {
				videoURLs = append(videoURLs, url)
			}
		case "audio_url":
			if url := asyncKieNestedURL(part["audio_url"]); url != "" {
				audioURLs = append(audioURLs, url)
			}
		}
	}
	return prompt, imageURLs, videoURLs, audioURLs
}

func asyncKieNestedURL(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		return strings.TrimSpace(fmt.Sprint(typed["url"]))
	default:
		return ""
	}
}

func asyncKieTaskStatus(payload asyncKieRecordInfoResponse) string {
	return strings.ToLower(firstAsyncNonEmpty(payload.Data.State, payload.Data.Status))
}

func asyncKieTaskSucceeded(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "success" || status == "succeeded" || status == "completed" || status == "finish" || status == "finished"
}

func asyncKieTaskFailed(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "fail" || status == "failed" || status == "error" || status == "canceled" || status == "cancelled" || status == "timeout" || status == "expired"
}

func asyncKieVideoOutputs(payload asyncKieRecordInfoResponse) []asyncTaskStoredOutput {
	urls := append([]string{}, payload.Data.ResultURLs...)
	urls = append(urls, payload.Data.Response.ResultURLs...)
	urls = append(urls, payload.Data.VideoURL, payload.Data.OutputURL, payload.Data.Response.VideoURL, payload.Data.Response.OutputURL)
	urls = append(urls, asyncKieResultJSONURLs(payload.Data.ResultJSON)...)
	outputs := make([]asyncTaskStoredOutput, 0, len(urls))
	seen := map[string]bool{}
	for _, rawURL := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		outputs = append(outputs, asyncTaskStoredOutput{MimeType: "video/mp4", URL: rawURL})
	}
	return outputs
}

func asyncKieResultJSONURLs(resultJSON string) []string {
	resultJSON = strings.TrimSpace(resultJSON)
	if resultJSON == "" {
		return nil
	}
	var payload struct {
		ResultURLs []string `json:"resultUrls"`
		URLs       []string `json:"urls"`
		VideoURL   string   `json:"videoUrl"`
		OutputURL  string   `json:"outputUrl"`
	}
	if err := common.Unmarshal([]byte(resultJSON), &payload); err != nil {
		return nil
	}
	urls := append([]string{}, payload.ResultURLs...)
	urls = append(urls, payload.URLs...)
	urls = append(urls, payload.VideoURL, payload.OutputURL)
	return urls
}

func persistAsyncTaskUpstreamTaskID(task *model.Task, upstreamTaskID string) {
	if task == nil || strings.TrimSpace(upstreamTaskID) == "" {
		return
	}
	task.PrivateData.UpstreamTaskID = strings.TrimSpace(upstreamTaskID)
	task.UpdatedAt = time.Now().Unix()
	_ = model.DB.Model(task).Select("private_data", "updated_at").Updates(task).Error
}

func doAsyncGeminiImageRequest(request *http.Request) ([]asyncTaskStoredOutput, error) {
	response, err := asyncTaskHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("upstream image task failed: %s", common.LocalLogPreview(string(body)))
	}
	var payload dto.GeminiChatResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	outputs := make([]asyncTaskStoredOutput, 0, len(payload.Candidates))
	for _, candidate := range payload.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || !strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "image/") {
				continue
			}
			content, err := decodeAsyncInlineBase64(part.InlineData.Data)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, asyncTaskStoredOutput{
				MimeType: firstAsyncNonEmpty(part.InlineData.MimeType, "image/png"),
				Content:  strings.TrimSpace(part.InlineData.Data),
				Size:     len(content),
			})
		}
	}
	if len(outputs) == 0 {
		return nil, errors.New("upstream Gemini image task returned no image")
	}
	return outputs, nil
}

func asyncStoredOutputFromDataURL(value string, fallbackMimeType string) (asyncTaskStoredOutput, bool, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:") {
		return asyncTaskStoredOutput{}, false, nil
	}
	prefix, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return asyncTaskStoredOutput{}, true, errors.New("upstream image task returned invalid data URL image content")
	}
	mimeType := fallbackMimeType
	if contentType, _, ok := strings.Cut(strings.TrimPrefix(prefix, "data:"), ";"); ok && strings.TrimSpace(contentType) != "" {
		mimeType = strings.TrimSpace(contentType)
	}
	encoded = strings.TrimSpace(encoded)
	content, err := decodeAsyncInlineBase64(encoded)
	if err != nil {
		return asyncTaskStoredOutput{}, true, err
	}
	return asyncTaskStoredOutput{MimeType: firstAsyncNonEmpty(mimeType, fallbackMimeType, "application/octet-stream"), Content: encoded, Size: len(content)}, true, nil
}

func decodeAsyncInlineBase64(encoded string) ([]byte, error) {
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("upstream image task returned invalid base64 image content")
	}
	if err := validateAsyncInlineContentSize(len(content)); err != nil {
		return nil, err
	}
	return content, nil
}

func validateAsyncInlineContentSize(size int) error {
	if asyncTaskInlineContentLimit > 0 && size > asyncTaskInlineContentLimit {
		return errors.New("upstream inline base64 image is too large; configure upstream response_format=url or object storage")
	}
	return nil
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
		if size == 0 && strings.TrimSpace(output.Content) != "" {
			decoded, err := base64.StdEncoding.DecodeString(output.Content)
			if err == nil {
				size = len(decoded)
			}
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
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeAsyncMultipartQuote(field), escapeAsyncMultipartQuote(filepath.Base(header.Filename))))
	partHeader.Set("Content-Type", asyncMultipartFileContentType(header, content))
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}
	_, err = part.Write(content)
	return err
}

func asyncMultipartFileContentType(header *multipart.FileHeader, content []byte) string {
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	if len(content) > 0 {
		detected := http.DetectContentType(content)
		if detected != "" && detected != "application/octet-stream" {
			return detected
		}
	}
	return "application/octet-stream"
}

func escapeAsyncMultipartQuote(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(value)
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

func downloadAsyncTaskOutputURL(task *model.Task, output asyncTaskStoredOutput) ([]byte, string, error) {
	rawURL := strings.TrimSpace(output.URL)
	headers := map[string]string{}
	if channel := asyncTaskAuthenticatedContentChannel(task, rawURL); channel != nil {
		headers["Authorization"] = "Bearer " + channel.Key
	}
	return downloadAsyncOutputURL(rawURL, headers)
}

func asyncTaskAuthenticatedContentChannel(task *model.Task, rawURL string) *model.Channel {
	if task == nil || strings.TrimSpace(rawURL) == "" {
		return nil
	}
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil || channel == nil {
		return nil
	}
	switch channel.Type {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora, constant.ChannelTypeJimengOpenAIVideo:
	default:
		return nil
	}
	if !asyncTaskURLMatchesOpenAIVideoContent(channel, rawURL) {
		return nil
	}
	return channel
}

func asyncTaskURLMatchesOpenAIVideoContent(channel *model.Channel, rawURL string) bool {
	outputURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || outputURL.Scheme == "" || outputURL.Host == "" {
		return false
	}
	baseURL, err := url.Parse(asyncChannelURL(channel, ""))
	if err != nil || baseURL.Host == "" {
		return false
	}
	if !strings.EqualFold(outputURL.Scheme, baseURL.Scheme) || !strings.EqualFold(outputURL.Host, baseURL.Host) {
		return false
	}
	basePath := strings.TrimRight(baseURL.EscapedPath(), "/")
	contentPrefix := basePath + "/v1/videos/"
	outputPath := outputURL.EscapedPath()
	return strings.HasPrefix(outputPath, contentPrefix) && strings.HasSuffix(outputPath, "/content")
}

func downloadAsyncOutputURL(url string, headers map[string]string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), asyncTaskHTTPTimeoutDuration())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			request.Header.Set(key, value)
		}
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
