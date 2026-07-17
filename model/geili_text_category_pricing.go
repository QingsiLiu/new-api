package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type TextCategoryPricing struct {
	Category    string  `json:"category" gorm:"size:16;primaryKey"`
	Multiplier  float64 `json:"multiplier"`
	UpdatedTime int64   `json:"updated_time" gorm:"bigint"`
}

func (TextCategoryPricing) TableName() string {
	return "text_category_pricing"
}

func GetTextCategoryMultipliers() (map[string]float64, error) {
	var rows []TextCategoryPricing
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, row := range rows {
		out[row.Category] = row.Multiplier
	}
	return out, nil
}

func UpsertTextCategoryPricing(entry *TextCategoryPricing) error {
	entry.Category = strings.ToLower(strings.TrimSpace(entry.Category))
	if entry.Category == "" {
		return errors.New("category 不能为空")
	}
	var existing TextCategoryPricing
	err := DB.Where("category = ?", entry.Category).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DB.Create(entry).Error
	}
	if err != nil {
		return err
	}
	return DB.Model(&existing).Updates(map[string]any{
		"multiplier":   entry.Multiplier,
		"updated_time": entry.UpdatedTime,
	}).Error
}
