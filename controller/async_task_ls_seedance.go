package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	asyncLsSeedanceAssetPollInterval = 2 * time.Second
	asyncLsSeedanceVideoPollInterval = 5 * time.Second
	asyncLsSeedanceMaxAssetPolls     = 60
)

type asyncLsSeedanceCreateResponse struct {
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Task    struct {
		ID     string `json:"id"`
		TaskID string `json:"task_id,omitempty"`
		Status string `json:"status,omitempty"`
		Error  any    `json:"error,omitempty"`
	} `json:"task"`
}

type asyncLsSeedanceTaskResponse struct {
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		TaskID    string `json:"task_id,omitempty"`
		Status    string `json:"status,omitempty"`
		Progress  string `json:"progress,omitempty"`
		ResultURL string `json:"result_url,omitempty"`
		Fail      string `json:"fail_reason,omitempty"`
		Error     string `json:"error,omitempty"`
		Message   string `json:"message,omitempty"`
		Data      struct {
			Task struct {
				Status  string   `json:"status,omitempty"`
				Outputs []string `json:"outputs,omitempty"`
				Usage   struct {
					TotalTokens int `json:"total_tokens,omitempty"`
				} `json:"usage,omitempty"`
			} `json:"task,omitempty"`
		} `json:"data,omitempty"`
	} `json:"data"`
}

type asyncLsSeedanceAssetGroupResponse struct {
	ID      string `json:"id"`
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type asyncLsSeedanceAssetResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status,omitempty"`
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func executeAsyncLsSeedanceVideoTask(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) ([]asyncTaskStoredOutput, error) {
	taskID, err := createAsyncLsSeedanceVideoTask(parentCtx, task, channel, request)
	if err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, errors.New("Ls.API Seedance video task returned no task id")
	}
	persistAsyncTaskUpstreamTaskID(task, taskID)
	for {
		record, err := pollAsyncLsSeedanceVideoTask(parentCtx, channel, taskID)
		if err != nil {
			return nil, err
		}
		status := asyncLsSeedanceTaskStatus(record)
		if asyncLsSeedanceTaskSucceeded(status) {
			resultURL := asyncLsSeedanceTaskResultURL(record)
			if resultURL == "" {
				return nil, errors.New("Ls.API Seedance video task succeeded without result URL")
			}
			return []asyncTaskStoredOutput{{
				MimeType: "video/mp4",
				URL:      resultURL,
			}}, nil
		}
		if asyncLsSeedanceTaskFailed(status) {
			return nil, errors.New(firstAsyncNonEmpty(record.Data.Fail, record.Data.Error, record.Data.Message, record.Message, record.Error, "Ls.API Seedance video task failed"))
		}
		select {
		case <-parentCtx.Done():
			return nil, parentCtx.Err()
		case <-time.After(asyncLsSeedanceVideoPollInterval):
		}
	}
}

func createAsyncLsSeedanceVideoTask(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) (string, error) {
	payload, err := asyncLsSeedanceVideoPayload(parentCtx, task, channel, request)
	if err != nil {
		return "", err
	}
	var responsePayload asyncLsSeedanceCreateResponse
	if err := doAsyncLsSeedanceJSON(parentCtx, channel, http.MethodPost, "/seedance/v1/video/generate", payload, &responsePayload); err != nil {
		return "", fmt.Errorf("upstream Ls.API Seedance video task failed: %w", err)
	}
	if !asyncLsSeedanceCodeOK(responsePayload.Code) {
		return "", errors.New(firstAsyncNonEmpty(responsePayload.Error, responsePayload.Message, "Ls.API Seedance video task create failed"))
	}
	return firstAsyncNonEmpty(responsePayload.Task.ID, responsePayload.Task.TaskID), nil
}

func pollAsyncLsSeedanceVideoTask(parentCtx context.Context, channel *model.Channel, taskID string) (asyncLsSeedanceTaskResponse, error) {
	var payload asyncLsSeedanceTaskResponse
	path := "/seedance/v1/video/tasks/" + url.PathEscape(taskID)
	if err := doAsyncLsSeedanceJSON(parentCtx, channel, http.MethodGet, path, nil, &payload); err != nil {
		return asyncLsSeedanceTaskResponse{}, fmt.Errorf("upstream Ls.API Seedance video task poll failed: %w", err)
	}
	if !asyncLsSeedanceCodeOK(payload.Code) {
		return asyncLsSeedanceTaskResponse{}, errors.New(firstAsyncNonEmpty(payload.Error, payload.Message, "Ls.API Seedance video task poll failed"))
	}
	return payload, nil
}

func asyncLsSeedanceVideoPayload(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) (map[string]interface{}, error) {
	content, err := asyncLsSeedanceContent(parentCtx, task, channel, request)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"model":   request.Model,
		"content": content,
	}
	if duration := asyncLsSeedanceDuration(request.Parameters); duration > 0 {
		payload["duration"] = duration
	}
	if resolution := asyncParamString(request.Parameters, "resolution"); resolution != "" {
		payload["resolution"] = resolution
	}
	if ratio := firstAsyncNonEmpty(asyncParamString(request.Parameters, "ratio"), asyncParamString(request.Parameters, "aspect_ratio")); ratio != "" {
		payload["ratio"] = ratio
	}
	for _, field := range []string{"generate_audio", "watermark", "return_last_frame"} {
		if value, ok := request.Parameters[field]; ok {
			payload[field] = value
		}
	}
	return payload, nil
}

func asyncLsSeedanceDuration(parameters map[string]interface{}) int {
	if duration := asyncParamIntValue(parameters, "duration", 0); duration > 0 {
		return duration
	}
	return asyncParamIntValue(parameters, "seconds", 0)
}

func asyncLsSeedanceContent(parentCtx context.Context, task *model.Task, channel *model.Channel, request asyncTaskRequest) ([]interface{}, error) {
	items := make([]interface{}, 0)
	if prompt := asyncVideoPrompt(request); prompt != "" {
		items = append(items, map[string]interface{}{"type": "text", "text": prompt})
	}
	imageURLs := asyncLsSeedanceImageURLs(request.Parameters)
	if len(imageURLs) > 0 {
		groupID := ""
		for idx, rawURL := range imageURLs {
			assetURL, err := asyncLsSeedanceAssetURL(parentCtx, channel, task, rawURL, idx, &groupID)
			if err != nil {
				return nil, err
			}
			items = append(items, map[string]interface{}{
				"type":      "image_url",
				"role":      "reference_image",
				"image_url": map[string]interface{}{"url": assetURL},
			})
		}
	}
	for _, audioURL := range asyncLsSeedanceAudioURLs(request.Parameters) {
		items = append(items, map[string]interface{}{
			"type":      "audio_url",
			"role":      "reference_audio",
			"audio_url": map[string]interface{}{"url": audioURL},
		})
	}
	if len(items) == 0 {
		return nil, errors.New("Ls.API Seedance content is required")
	}
	return items, nil
}

func asyncLsSeedanceAssetURL(parentCtx context.Context, channel *model.Channel, task *model.Task, rawURL string, index int, groupID *string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("empty Ls.API Seedance image reference")
	}
	if strings.HasPrefix(rawURL, "asset://") {
		return rawURL, nil
	}
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") && !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return "", fmt.Errorf("unsupported Ls.API Seedance image reference: %s", rawURL)
	}
	if groupID == nil {
		return "", errors.New("Ls.API Seedance asset group pointer is nil")
	}
	if strings.TrimSpace(*groupID) == "" {
		createdGroupID, err := createAsyncLsSeedanceAssetGroup(parentCtx, channel, task)
		if err != nil {
			return "", err
		}
		*groupID = createdGroupID
	}
	assetID, err := uploadAsyncLsSeedanceAsset(parentCtx, channel, *groupID, rawURL, index)
	if err != nil {
		return "", err
	}
	if err := waitAsyncLsSeedanceAssetReady(parentCtx, channel, assetID); err != nil {
		return "", err
	}
	return "asset://" + assetID, nil
}

func createAsyncLsSeedanceAssetGroup(parentCtx context.Context, channel *model.Channel, task *model.Task) (string, error) {
	name := "geili-async-ref"
	if task != nil && strings.TrimSpace(task.TaskID) != "" {
		name = "geili-" + task.TaskID
	}
	var payload asyncLsSeedanceAssetGroupResponse
	if err := doAsyncLsSeedanceJSON(parentCtx, channel, http.MethodPost, "/seedance/v1/asset-groups", map[string]interface{}{
		"name":        name,
		"description": "Geili async video references",
	}, &payload); err != nil {
		return "", fmt.Errorf("Ls.API Seedance asset group create failed: %w", err)
	}
	if !asyncLsSeedanceCodeOK(payload.Code) || strings.TrimSpace(payload.ID) == "" {
		return "", errors.New(firstAsyncNonEmpty(payload.Error, payload.Message, "Ls.API Seedance asset group create failed"))
	}
	return payload.ID, nil
}

func uploadAsyncLsSeedanceAsset(parentCtx context.Context, channel *model.Channel, groupID string, imageURL string, index int) (string, error) {
	var payload asyncLsSeedanceAssetResponse
	if err := doAsyncLsSeedanceJSON(parentCtx, channel, http.MethodPost, "/seedance/v1/assets", map[string]interface{}{
		"group_id":   groupID,
		"url":        imageURL,
		"asset_type": "Image",
		"name":       fmt.Sprintf("geili-ref-%d", index+1),
	}, &payload); err != nil {
		return "", fmt.Errorf("Ls.API Seedance asset upload failed: %w", err)
	}
	if !asyncLsSeedanceCodeOK(payload.Code) || strings.TrimSpace(payload.ID) == "" {
		return "", errors.New(firstAsyncNonEmpty(payload.Error, payload.Message, "Ls.API Seedance asset upload failed"))
	}
	return payload.ID, nil
}

func waitAsyncLsSeedanceAssetReady(parentCtx context.Context, channel *model.Channel, assetID string) error {
	for attempt := 0; attempt < asyncLsSeedanceMaxAssetPolls; attempt++ {
		var payload asyncLsSeedanceAssetResponse
		if err := doAsyncLsSeedanceJSON(parentCtx, channel, http.MethodPost, "/seedance/v1/assets/get", map[string]interface{}{
			"asset_id": assetID,
		}, &payload); err != nil {
			return fmt.Errorf("Ls.API Seedance asset poll failed: %w", err)
		}
		if !asyncLsSeedanceCodeOK(payload.Code) {
			return errors.New(firstAsyncNonEmpty(payload.Error, payload.Message, "Ls.API Seedance asset poll failed"))
		}
		status := strings.ToLower(strings.TrimSpace(payload.Status))
		switch status {
		case "completed", "complete", "success", "succeeded":
			return nil
		case "failed", "failure", "fail", "error", "canceled", "cancelled":
			return errors.New(firstAsyncNonEmpty(payload.Error, payload.Message, "Ls.API Seedance asset processing failed"))
		}
		select {
		case <-parentCtx.Done():
			return parentCtx.Err()
		case <-time.After(asyncLsSeedanceAssetPollInterval):
		}
	}
	return errors.New("Ls.API Seedance asset processing timed out")
}

func doAsyncLsSeedanceJSON(parentCtx context.Context, channel *model.Channel, method string, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		bytesBody, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bytesBody)
	}
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, asyncChannelURL(channel, path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := asyncTaskHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("status %d: %s", resp.StatusCode, common.LocalLogPreview(string(responseBody)))
	}
	if out == nil {
		return nil
	}
	if err := common.Unmarshal(responseBody, out); err != nil {
		return err
	}
	return nil
}

func asyncLsSeedanceImageURLs(parameters map[string]interface{}) []string {
	urls := make([]string, 0)
	for _, key := range []string{"image", "image_url", "images", "image_urls", "input_urls", "image_input"} {
		urls = append(urls, asyncStringSliceParam(parameters[key])...)
	}
	for _, reference := range asyncOpenAIVideoReferencesParam(parameters["input_references"]) {
		if asyncOpenAIVideoReferenceType(reference) == "image_url" {
			if url := asyncLsSeedanceReferenceURL(reference, "image_url"); url != "" {
				urls = append(urls, url)
			}
		}
	}
	_, contentImageURLs, _, _ := asyncKieSeedanceContentParts(parameters["content"])
	urls = append(urls, contentImageURLs...)
	return dedupeAsyncStrings(urls)
}

func asyncLsSeedanceAudioURLs(parameters map[string]interface{}) []string {
	urls := make([]string, 0)
	for _, key := range []string{"audio", "audio_url", "audio_urls"} {
		urls = append(urls, asyncStringSliceParam(parameters[key])...)
	}
	for _, reference := range asyncOpenAIVideoReferencesParam(parameters["input_references"]) {
		if asyncOpenAIVideoReferenceType(reference) == "audio_url" {
			if url := asyncLsSeedanceReferenceURL(reference, "audio_url"); url != "" {
				urls = append(urls, url)
			}
		}
	}
	_, _, _, contentAudioURLs := asyncKieSeedanceContentParts(parameters["content"])
	urls = append(urls, contentAudioURLs...)
	return dedupeAsyncStrings(urls)
}

func asyncLsSeedanceReferenceURL(reference interface{}, field string) string {
	switch typed := reference.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		if url := asyncKieNestedURL(typed[field]); url != "" {
			return url
		}
		if url := asyncKieNestedURL(typed["url"]); url != "" {
			return url
		}
	}
	return ""
}

func asyncLsSeedanceCodeOK(raw any) bool {
	if raw == nil {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
	return value == "" || value == "success" || value == "0" || value == "200"
}

func asyncLsSeedanceTaskStatus(payload asyncLsSeedanceTaskResponse) string {
	return strings.ToLower(firstAsyncNonEmpty(payload.Data.Status, payload.Data.Data.Task.Status))
}

func asyncLsSeedanceTaskSucceeded(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "success" || status == "succeeded" || status == "completed" || status == "complete" || status == "finished"
}

func asyncLsSeedanceTaskFailed(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "failure" || status == "failed" || status == "fail" || status == "error" || status == "canceled" || status == "cancelled" || status == "timeout" || status == "expired"
}

func asyncLsSeedanceTaskResultURL(payload asyncLsSeedanceTaskResponse) string {
	if payload.Data.ResultURL != "" {
		return strings.TrimSpace(payload.Data.ResultURL)
	}
	if len(payload.Data.Data.Task.Outputs) > 0 {
		return strings.TrimSpace(payload.Data.Data.Task.Outputs[0])
	}
	return ""
}
