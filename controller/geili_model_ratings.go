package controller

import (
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const publicRatingThreshold int64 = 20

type modelRatingResponse struct {
	Slug       string   `json:"slug"`
	Count      int64    `json:"count"`
	Early      bool     `json:"early"`
	Average    *float64 `json:"average,omitempty"`
	UserRating *int     `json:"user_rating,omitempty"`
}

func buildModelRatingResponse(slug string, userRating *int) (modelRatingResponse, error) {
	aggregate, err := model.GetModelRatingAggregate(slug)
	if err != nil {
		return modelRatingResponse{}, err
	}
	response := modelRatingResponse{
		Slug:       slug,
		Count:      aggregate.Count,
		Early:      aggregate.Count < publicRatingThreshold,
		UserRating: userRating,
	}
	if !response.Early {
		average := math.Round(aggregate.Average*10) / 10
		response.Average = &average
	}
	return response, nil
}

func enabledRatingSlug(c *gin.Context, rawSlug string) (string, bool) {
	slug := strings.TrimSpace(strings.ToLower(rawSlug))
	if _, err := model.GetModelRegistryBySlug(slug); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return "", false
	}
	return slug, true
}

// GetPublicModelRating GET /v1/public/models/:slug/rating
func GetPublicModelRating(c *gin.Context) {
	slug, ok := enabledRatingSlug(c, c.Param("slug"))
	if !ok {
		return
	}
	response, err := buildModelRatingResponse(slug, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, response)
}

type modelRatingRequest struct {
	Slug   string `json:"slug"`
	Rating int    `json:"rating"`
}

// UpsertModelRating POST /api/geili/model-ratings
func UpsertModelRating(c *gin.Context) {
	var request modelRatingRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Rating < 1 || request.Rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "rating must be between 1 and 5"})
		return
	}
	slug, ok := enabledRatingSlug(c, request.Slug)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}
	if err := model.UpsertModelRating(userID, slug, request.Rating); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := buildModelRatingResponse(slug, &request.Rating)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}
