package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
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
