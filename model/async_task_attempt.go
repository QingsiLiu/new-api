package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AsyncTaskAttempt is an internal, provider-safe audit record for one execution
// of an async image or video task. It deliberately excludes request payloads,
// credentials, upstream URLs, and raw provider response bodies.
type AsyncTaskAttempt struct {
	ID int64 `json:"id" gorm:"primaryKey"`

	TaskID    string `json:"task_id" gorm:"type:varchar(191);not null;uniqueIndex:idx_async_task_attempt_number;index:idx_async_task_attempt_task"`
	UserID    int    `json:"user_id" gorm:"index"`
	AttemptNo int    `json:"attempt_no" gorm:"not null;uniqueIndex:idx_async_task_attempt_number"`

	ChannelID int    `json:"channel_id" gorm:"not null;index:idx_async_task_attempt_channel_started,priority:1"`
	Group     string `json:"group" gorm:"type:varchar(64)"`
	Model     string `json:"model" gorm:"type:varchar(255);index:idx_async_task_attempt_model_started,priority:1"`
	Kind      string `json:"kind" gorm:"type:varchar(16)"`
	Action    string `json:"action" gorm:"type:varchar(32)"`
	SpecKey   string `json:"spec_key,omitempty" gorm:"type:varchar(64)"`

	Status          string `json:"status" gorm:"type:varchar(16);index"`
	Stage           string `json:"stage,omitempty" gorm:"type:varchar(16)"`
	FailureClass    string `json:"failure_class,omitempty" gorm:"type:varchar(32);index"`
	Retryable       bool   `json:"retryable"`
	AcceptanceState string `json:"acceptance_state" gorm:"type:varchar(16)"`
	HTTPStatus      int    `json:"http_status,omitempty"`
	ProviderCode    string `json:"provider_code,omitempty" gorm:"type:varchar(128)"`
	UpstreamTaskID  string `json:"upstream_task_id,omitempty" gorm:"type:varchar(191)"`

	StartedAt       int64 `json:"started_at" gorm:"bigint;index:idx_async_task_attempt_started;index:idx_async_task_attempt_channel_started,priority:2;index:idx_async_task_attempt_model_started,priority:2"`
	SubmittedAt     int64 `json:"submitted_at,omitempty" gorm:"bigint"`
	CompletedAt     int64 `json:"completed_at,omitempty" gorm:"bigint"`
	SubmitLatencyMS int64 `json:"submit_latency_ms,omitempty" gorm:"bigint"`
	DurationMS      int64 `json:"duration_ms,omitempty" gorm:"bigint"`
	PollCount       int   `json:"poll_count,omitempty"`
	PollErrorCount  int   `json:"poll_error_count,omitempty"`
	CreatedAt       int64 `json:"created_at" gorm:"bigint;index:idx_async_task_attempt_created"`
	UpdatedAt       int64 `json:"updated_at" gorm:"bigint"`
}

const (
	AsyncTaskAttemptStatusRunning   = "running"
	AsyncTaskAttemptStatusSucceeded = "succeeded"
	AsyncTaskAttemptStatusFailed    = "failed"
	AsyncTaskAttemptStatusSkipped   = "skipped"

	AsyncAttemptStageSelect   = "select"
	AsyncAttemptStageUpload   = "upload"
	AsyncAttemptStageSubmit   = "submit"
	AsyncAttemptStagePoll     = "poll"
	AsyncAttemptStageDownload = "download"
	AsyncAttemptStageArchive  = "archive"

	AsyncAttemptAcceptanceNotAccepted = "not_accepted"
	AsyncAttemptAcceptanceAccepted    = "accepted"
	AsyncAttemptAcceptanceUnknown     = "unknown"
)

func (a *AsyncTaskAttempt) BeforeCreate(_ *gorm.DB) error {
	if a.CreatedAt == 0 {
		a.CreatedAt = time.Now().Unix()
	}
	if a.UpdatedAt == 0 {
		a.UpdatedAt = a.CreatedAt
	}
	if a.StartedAt == 0 {
		a.StartedAt = a.CreatedAt
	}
	return nil
}

func CreateAsyncTaskAttempt(attempt *AsyncTaskAttempt) error {
	if attempt == nil {
		return nil
	}
	if attempt.CreatedAt == 0 {
		attempt.CreatedAt = common.GetTimestamp()
	}
	if attempt.UpdatedAt == 0 {
		attempt.UpdatedAt = attempt.CreatedAt
	}
	if attempt.StartedAt == 0 {
		attempt.StartedAt = attempt.CreatedAt
	}
	return DB.Create(attempt).Error
}

func UpdateAsyncTaskAttempt(attempt *AsyncTaskAttempt) error {
	if attempt == nil || attempt.ID == 0 {
		return nil
	}
	attempt.UpdatedAt = common.GetTimestamp()
	return DB.Model(attempt).Select("*").Updates(attempt).Error
}

func GetAsyncTaskAttempts(taskID string) ([]AsyncTaskAttempt, error) {
	attempts := make([]AsyncTaskAttempt, 0)
	if taskID == "" {
		return attempts, nil
	}
	err := DB.Where("task_id = ?", taskID).Order("attempt_no").Find(&attempts).Error
	return attempts, err
}

type AsyncTaskAttemptQuery struct {
	StartedAfter  int64
	StartedBefore int64
	ChannelID     int
	Model         string
	Kind          string
	Action        string
}

func QueryAsyncTaskAttempts(query AsyncTaskAttemptQuery) ([]AsyncTaskAttempt, error) {
	attempts := make([]AsyncTaskAttempt, 0)
	db := DB.Model(&AsyncTaskAttempt{})
	if query.StartedAfter > 0 {
		db = db.Where("started_at >= ?", query.StartedAfter)
	}
	if query.StartedBefore > 0 {
		db = db.Where("started_at <= ?", query.StartedBefore)
	}
	if query.ChannelID > 0 {
		db = db.Where("channel_id = ?", query.ChannelID)
	}
	if query.Model != "" {
		db = db.Where("model = ?", query.Model)
	}
	if query.Kind != "" {
		db = db.Where("kind = ?", query.Kind)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	err := db.Order("started_at ASC, id ASC").Find(&attempts).Error
	return attempts, err
}

// DeleteExpiredAsyncTaskAttempts removes only internal audit records; tasks and
// billing evidence remain untouched. Callers should use a cutoff older than the
// configured retention period.
func DeleteExpiredAsyncTaskAttempts(cutoffUnix int64) (int64, error) {
	if cutoffUnix <= 0 {
		return 0, nil
	}
	result := DB.Where("created_at < ?", cutoffUnix).Delete(&AsyncTaskAttempt{})
	return result.RowsAffected, result.Error
}
