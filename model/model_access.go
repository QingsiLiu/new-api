package model

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

func splitCSVValues(raw string) []string {
	parts := strings.Split(strings.Trim(raw, ","), ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func joinCSVValues(values []string) string {
	return strings.Join(values, ",")
}

func setCSVValue(values []string, value string, enabled bool) ([]string, bool) {
	exists := false
	next := make([]string, 0, len(values)+1)
	for _, current := range values {
		if current == value {
			exists = true
			if !enabled {
				continue
			}
		}
		next = append(next, current)
	}
	if enabled && !exists {
		next = append(next, value)
	}
	return next, exists != enabled
}

func replaceCSVValue(values []string, oldValue string, newValue string) ([]string, bool) {
	changed := false
	seen := make(map[string]struct{}, len(values))
	next := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case oldValue:
			changed = true
			if _, ok := seen[newValue]; ok {
				continue
			}
			seen[newValue] = struct{}{}
			next = append(next, newValue)
		case newValue:
			if _, ok := seen[newValue]; ok {
				changed = true
				continue
			}
			seen[newValue] = struct{}{}
			next = append(next, newValue)
		default:
			if _, ok := seen[value]; ok {
				changed = true
				continue
			}
			seen[value] = struct{}{}
			next = append(next, value)
		}
	}
	return next, changed
}

func renameModelChannelAccessTx(tx *gorm.DB, oldModelName string, newModelName string) (int, error) {
	oldModelName = strings.TrimSpace(oldModelName)
	newModelName = strings.TrimSpace(newModelName)
	if oldModelName == "" || newModelName == "" || oldModelName == newModelName {
		return 0, nil
	}

	var channels []Channel
	if err := tx.Find(&channels).Error; err != nil {
		return 0, err
	}

	updated := 0
	for i := range channels {
		channel := channels[i]
		models, changed := replaceCSVValue(splitCSVValues(channel.Models), oldModelName, newModelName)
		if !changed {
			continue
		}
		channel.Models = joinCSVValues(models)
		if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", channel.Models).Error; err != nil {
			return updated, err
		}
		if err := channel.UpdateAbilities(tx); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func UpdateModelChannelAccess(modelName string, channelIDs []int) (int, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return 0, fmt.Errorf("model name is required")
	}

	selected := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			return 0, fmt.Errorf("invalid channel id: %d", channelID)
		}
		selected[channelID] = struct{}{}
	}

	var channels []Channel
	if err := DB.Find(&channels).Error; err != nil {
		return 0, err
	}
	existing := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		existing[channel.Id] = struct{}{}
	}
	for channelID := range selected {
		if _, ok := existing[channelID]; !ok {
			return 0, fmt.Errorf("channel not found: %s", strconv.Itoa(channelID))
		}
	}

	tx := DB.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updated := 0
	for i := range channels {
		channel := channels[i]
		_, shouldEnable := selected[channel.Id]
		models, changed := setCSVValue(splitCSVValues(channel.Models), modelName, shouldEnable)
		if !changed {
			continue
		}
		channel.Models = joinCSVValues(models)
		if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", channel.Models).Error; err != nil {
			tx.Rollback()
			return updated, err
		}
		if err := channel.UpdateAbilities(tx); err != nil {
			tx.Rollback()
			return updated, err
		}
		updated++
	}

	if err := tx.Commit().Error; err != nil {
		return updated, err
	}
	return updated, nil
}
