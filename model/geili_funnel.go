package model

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FunnelEnvironmentProduction = "production"
	FunnelEnvironmentStaging    = "staging"

	FunnelIdentityAnonymous = "anonymous"
	FunnelIdentityLinked    = "linked"
	FunnelIdentityAmbiguous = "ambiguous"

	FunnelEventSLPView        = "slp_view"
	FunnelEventIdentityLink   = "identity_link"
	FunnelEventAccountActive  = "account_active"
	FunnelEventOpenStudio     = "open_studio"
	FunnelEventPlaygroundFail = "playground_fail"
)

// FunnelEventInput is the storage-facing event contract. It deliberately has
// no catch-all payload field: callers must choose one of the fixed event
// shapes enforced by the service package.
type FunnelEventInput struct {
	Environment  string
	EventID      string
	EventName    string
	EventVersion int
	VisitorHMAC  string
	Locale       string
	ModelSlug    string
	FailureCode  string
	UserID       int
	ReceivedAt   int64
}

type FunnelIngestResult struct {
	Duplicate     bool
	VisitorID     int64
	IdentityState string
}

type FunnelVisitor struct {
	ID             int64   `json:"-" gorm:"primaryKey"`
	Environment    string  `json:"-" gorm:"type:varchar(16);uniqueIndex:idx_funnel_visitor_env_hash,priority:1;index:idx_funnel_visitor_user_state,priority:1"`
	VisitorHMAC    *string `json:"-" gorm:"type:char(64);uniqueIndex:idx_funnel_visitor_env_hash,priority:2"`
	IdentityState  string  `json:"-" gorm:"type:varchar(16);index:idx_funnel_visitor_user_state,priority:3"`
	UserID         *int    `json:"-" gorm:"index:idx_funnel_visitor_user_state,priority:2"`
	FirstSeenAt    int64   `json:"-" gorm:"type:bigint"`
	LastSeenAt     int64   `json:"-" gorm:"type:bigint;index"`
	FirstSLPAt     int64   `json:"-" gorm:"type:bigint;index"`
	FirstSLPLocale string  `json:"-" gorm:"type:varchar(2)"`
	FirstSLPModel  string  `json:"-" gorm:"type:varchar(96)"`
	CreatedAt      int64   `json:"-" gorm:"type:bigint"`
	UpdatedAt      int64   `json:"-" gorm:"type:bigint"`
}

type FunnelEvent struct {
	ID           int64  `json:"-" gorm:"primaryKey"`
	Environment  string `json:"-" gorm:"type:varchar(16);uniqueIndex:idx_funnel_event_env_id,priority:1;index:idx_funnel_event_name_time,priority:1;index:idx_funnel_event_model_time,priority:1"`
	EventID      string `json:"-" gorm:"type:char(36);uniqueIndex:idx_funnel_event_env_id,priority:2"`
	VisitorID    int64  `json:"-" gorm:"index:idx_funnel_event_visitor_time,priority:1"`
	EventName    string `json:"-" gorm:"type:varchar(32);index:idx_funnel_event_name_time,priority:2"`
	EventVersion int    `json:"-" gorm:"type:int"`
	ReceivedAt   int64  `json:"-" gorm:"type:bigint;index:idx_funnel_event_name_time,priority:3;index:idx_funnel_event_visitor_time,priority:2;index:idx_funnel_event_model_time,priority:3"`
	Locale       string `json:"-" gorm:"type:varchar(2)"`
	ModelSlug    string `json:"-" gorm:"type:varchar(96);index:idx_funnel_event_model_time,priority:2"`
	FailureCode  string `json:"-" gorm:"type:varchar(32)"`
}

type FunnelActivityDay struct {
	ID           int64  `json:"-" gorm:"primaryKey"`
	Environment  string `json:"-" gorm:"type:varchar(16);uniqueIndex:idx_funnel_activity_user_day,priority:1"`
	UserID       int    `json:"-" gorm:"uniqueIndex:idx_funnel_activity_user_day,priority:2;index"`
	ActivityDate int64  `json:"-" gorm:"type:bigint;uniqueIndex:idx_funnel_activity_user_day,priority:3;index"`
	FirstSeenAt  int64  `json:"-" gorm:"type:bigint"`
	LastSeenAt   int64  `json:"-" gorm:"type:bigint"`
	CreatedAt    int64  `json:"-" gorm:"type:bigint"`
	UpdatedAt    int64  `json:"-" gorm:"type:bigint"`
}

func (visitor *FunnelVisitor) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if visitor.CreatedAt == 0 {
		visitor.CreatedAt = now
	}
	if visitor.UpdatedAt == 0 {
		visitor.UpdatedAt = now
	}
	return nil
}

func (visitor *FunnelVisitor) BeforeUpdate(_ *gorm.DB) error {
	visitor.UpdatedAt = common.GetTimestamp()
	return nil
}

func (event *FunnelEvent) BeforeCreate(_ *gorm.DB) error {
	return nil
}

func (activity *FunnelActivityDay) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if activity.CreatedAt == 0 {
		activity.CreatedAt = now
	}
	if activity.UpdatedAt == 0 {
		activity.UpdatedAt = now
	}
	return nil
}

func (activity *FunnelActivityDay) BeforeUpdate(_ *gorm.DB) error {
	activity.UpdatedAt = common.GetTimestamp()
	return nil
}

var (
	errFunnelEventConflict = errors.New("funnel event conflict")
	funnelSQLiteIngestMu   sync.Mutex
)

// IngestFunnelEventRecord persists one validated event and all of its derived
// state in a single transaction. The small SQLite mutex keeps two goroutines
// from turning SQLite's deferred writer lock into a spurious "database is
// locked" error; server databases still rely on row locks and unique keys.
func IngestFunnelEventRecord(ctx context.Context, input FunnelEventInput) (FunnelIngestResult, error) {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		funnelSQLiteIngestMu.Lock()
		defer funnelSQLiteIngestMu.Unlock()
	}

	var result FunnelIngestResult
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing FunnelEvent
		err := tx.Where("environment = ? AND event_id = ?", input.Environment, input.EventID).First(&existing).Error
		switch {
		case err == nil:
			result = funnelDuplicateResult(tx, existing)
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		hash := input.VisitorHMAC
		visitorSeed := FunnelVisitor{
			Environment:   input.Environment,
			VisitorHMAC:   &hash,
			IdentityState: FunnelIdentityAnonymous,
			FirstSeenAt:   input.ReceivedAt,
			LastSeenAt:    input.ReceivedAt,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&visitorSeed).Error; err != nil {
			return err
		}

		var visitor FunnelVisitor
		if err := lockForUpdate(tx).Where("environment = ? AND visitor_hmac = ?", input.Environment, input.VisitorHMAC).First(&visitor).Error; err != nil {
			return err
		}
		applyFunnelVisitorSeen(&visitor, input)
		applyFunnelIdentity(&visitor, input)
		if err := tx.Save(&visitor).Error; err != nil {
			return err
		}

		if input.EventName == FunnelEventAccountActive && visitor.IdentityState == FunnelIdentityLinked && visitor.UserID != nil && *visitor.UserID == input.UserID {
			if err := upsertFunnelActivityDay(tx, input.Environment, *visitor.UserID, input.ReceivedAt); err != nil {
				return err
			}
		}

		event := FunnelEvent{
			Environment:  input.Environment,
			EventID:      input.EventID,
			VisitorID:    visitor.ID,
			EventName:    input.EventName,
			EventVersion: input.EventVersion,
			ReceivedAt:   input.ReceivedAt,
			Locale:       input.Locale,
			ModelSlug:    input.ModelSlug,
			FailureCode:  input.FailureCode,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return errFunnelEventConflict
		}

		if err := upsertFunnelCollectionStart(tx, input.Environment, input.ReceivedAt); err != nil {
			return err
		}
		result = FunnelIngestResult{VisitorID: visitor.ID, IdentityState: visitor.IdentityState}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if errors.Is(err, errFunnelEventConflict) {
		var existing FunnelEvent
		if lookupErr := DB.WithContext(ctx).Where("environment = ? AND event_id = ?", input.Environment, input.EventID).First(&existing).Error; lookupErr != nil {
			return FunnelIngestResult{}, lookupErr
		}
		return funnelDuplicateResult(DB.WithContext(ctx), existing), nil
	}
	return FunnelIngestResult{}, err
}

func funnelDuplicateResult(db *gorm.DB, event FunnelEvent) FunnelIngestResult {
	result := FunnelIngestResult{Duplicate: true, VisitorID: event.VisitorID}
	var visitor FunnelVisitor
	if err := db.Select("id", "identity_state").First(&visitor, event.VisitorID).Error; err == nil {
		result.IdentityState = visitor.IdentityState
	}
	return result
}

func applyFunnelVisitorSeen(visitor *FunnelVisitor, input FunnelEventInput) {
	if visitor.FirstSeenAt == 0 || input.ReceivedAt < visitor.FirstSeenAt {
		visitor.FirstSeenAt = input.ReceivedAt
	}
	if input.ReceivedAt > visitor.LastSeenAt {
		visitor.LastSeenAt = input.ReceivedAt
	}
	if input.EventName == FunnelEventSLPView && (visitor.FirstSLPAt == 0 || input.ReceivedAt < visitor.FirstSLPAt) {
		visitor.FirstSLPAt = input.ReceivedAt
		visitor.FirstSLPLocale = input.Locale
		visitor.FirstSLPModel = input.ModelSlug
	}
}

func applyFunnelIdentity(visitor *FunnelVisitor, input FunnelEventInput) {
	if input.UserID <= 0 || !isFunnelTrustedEvent(input.EventName) {
		return
	}
	if visitor.IdentityState == FunnelIdentityAmbiguous {
		visitor.UserID = nil
		return
	}
	if visitor.IdentityState == FunnelIdentityLinked && visitor.UserID != nil && *visitor.UserID != input.UserID {
		visitor.IdentityState = FunnelIdentityAmbiguous
		visitor.UserID = nil
		return
	}
	userID := input.UserID
	visitor.IdentityState = FunnelIdentityLinked
	visitor.UserID = &userID
}

func isFunnelTrustedEvent(eventName string) bool {
	switch eventName {
	case FunnelEventIdentityLink, FunnelEventAccountActive, FunnelEventOpenStudio:
		return true
	default:
		return false
	}
}

func upsertFunnelActivityDay(tx *gorm.DB, environment string, userID int, receivedAt int64) error {
	activityDate := time.Unix(receivedAt, 0).UTC().Truncate(24 * time.Hour).Unix()
	seed := FunnelActivityDay{Environment: environment, UserID: userID, ActivityDate: activityDate, FirstSeenAt: receivedAt, LastSeenAt: receivedAt}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	var activity FunnelActivityDay
	if err := lockForUpdate(tx).Where("environment = ? AND user_id = ? AND activity_date = ?", environment, userID, activityDate).First(&activity).Error; err != nil {
		return err
	}
	if activity.FirstSeenAt == 0 || receivedAt < activity.FirstSeenAt {
		activity.FirstSeenAt = receivedAt
	}
	if receivedAt > activity.LastSeenAt {
		activity.LastSeenAt = receivedAt
	}
	return tx.Save(&activity).Error
}

func upsertFunnelCollectionStart(tx *gorm.DB, environment string, receivedAt int64) error {
	key := "GeiliFunnelCollectionStartedAt." + environment
	seed := Option{Key: key, Value: strconv.FormatInt(receivedAt, 10)}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	var option Option
	if err := lockForUpdate(tx).Where("key = ?", key).First(&option).Error; err != nil {
		return err
	}
	current, err := strconv.ParseInt(option.Value, 10, 64)
	if err != nil || current <= 0 || receivedAt < current {
		option.Value = strconv.FormatInt(receivedAt, 10)
		return tx.Save(&option).Error
	}
	return nil
}
