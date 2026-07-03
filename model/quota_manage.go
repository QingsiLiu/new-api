package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// UserQuotaChangeRecord stores idempotent quota management requests.
type UserQuotaChangeRecord struct {
	Id         int    `json:"id"`
	RequestId  string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId     int    `json:"user_id" gorm:"index"`
	Mode       string `json:"mode" gorm:"type:varchar(16)"`
	Delta      int    `json:"delta" gorm:"type:bigint"`
	BeforeQuota int   `json:"before_quota" gorm:"type:bigint"`
	AfterQuota  int   `json:"after_quota" gorm:"type:bigint"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
}

func (record *UserQuotaChangeRecord) BeforeCreate(tx *gorm.DB) error {
	record.CreatedAt = common.GetTimestamp()
	return nil
}
