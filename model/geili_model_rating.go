package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// ModelRating stores one current rating per user and public model slug.
type ModelRating struct {
	Id          int    `json:"id"`
	UserId      int    `json:"-" gorm:"not null;uniqueIndex:idx_model_rating_user"`
	ModelSlug   string `json:"model_slug" gorm:"size:128;not null;uniqueIndex:idx_model_rating_user;index"`
	Rating      int    `json:"rating" gorm:"not null"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;not null"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint;not null"`
}

func (ModelRating) TableName() string {
	return "model_ratings"
}

type ModelRatingAggregate struct {
	Count   int64   `json:"count"`
	Average float64 `json:"average"`
}

func UpsertModelRating(userID int, slug string, rating int) error {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if userID <= 0 || slug == "" || rating < 1 || rating > 5 {
		return errors.New("invalid model rating")
	}
	now := time.Now().Unix()
	entry := ModelRating{
		UserId:      userID,
		ModelSlug:   slug,
		Rating:      rating,
		CreatedTime: now,
		UpdatedTime: now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "model_slug"}},
		DoUpdates: clause.Assignments(map[string]any{
			"rating":       rating,
			"updated_time": now,
		}),
	}).Create(&entry).Error
}

func GetModelRatingAggregate(slug string) (ModelRatingAggregate, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return ModelRatingAggregate{}, errors.New("slug is required")
	}
	var aggregate ModelRatingAggregate
	err := DB.Model(&ModelRating{}).
		Select("COUNT(*) AS count, COALESCE(AVG(rating), 0) AS average").
		Where("model_slug = ?", slug).
		Scan(&aggregate).Error
	return aggregate, err
}
