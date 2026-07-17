package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupModelRatingTestDB(t *testing.T) {
	t.Helper()
	setupModelRegistryTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ModelRating{}))
	seedModelRegistryFixtures(t)
}

func postModelRating(t *testing.T, userID int, slug string, rating int) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/geili/model-ratings",
		strings.NewReader(fmt.Sprintf(`{"slug":%q,"rating":%d}`, slug, rating)),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	UpsertModelRating(ctx)
	return rec
}

func getModelRating(t *testing.T, slug string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/public/models/"+slug+"/rating", nil)
	ctx.Params = gin.Params{{Key: "slug", Value: slug}}
	GetPublicModelRating(ctx)
	return rec
}

func ratingResponseData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}

func TestUpsertModelRatingKeepsOneVotePerUser(t *testing.T) {
	setupModelRatingTestDB(t)

	require.Equal(t, http.StatusOK, postModelRating(t, 7, "seedance-2-0", 5).Code)
	updated := postModelRating(t, 7, "seedance-2-0", 2)
	require.Equal(t, http.StatusOK, updated.Code)

	var count int64
	require.NoError(t, model.DB.Model(&model.ModelRating{}).Count(&count).Error)
	require.EqualValues(t, 1, count, "同一用户重复评分必须更新而不是新增")

	data := ratingResponseData(t, updated)
	require.Equal(t, float64(1), data["count"])
	require.Equal(t, true, data["early"])
	require.Equal(t, float64(2), data["user_rating"])
	require.NotContains(t, data, "average", "少于 20 票时端点也不得公开均分")
}

func TestPublicModelRatingHidesAverageUntilTwentyVotes(t *testing.T) {
	setupModelRatingTestDB(t)
	for userID := 1; userID <= 19; userID++ {
		require.NoError(t, model.UpsertModelRating(userID, "seedance-2-0", 4))
	}

	early := ratingResponseData(t, getModelRating(t, "seedance-2-0"))
	require.Equal(t, float64(19), early["count"])
	require.Equal(t, true, early["early"])
	require.NotContains(t, early, "average")

	require.NoError(t, model.UpsertModelRating(20, "seedance-2-0", 4))
	mature := ratingResponseData(t, getModelRating(t, "seedance-2-0"))
	require.Equal(t, float64(20), mature["count"])
	require.Equal(t, false, mature["early"])
	require.Equal(t, 4.0, mature["average"])
}

func TestModelRatingRejectsInvalidOrUnknownModel(t *testing.T) {
	setupModelRatingTestDB(t)

	require.Equal(t, http.StatusBadRequest, postModelRating(t, 7, "seedance-2-0", 0).Code)
	require.Equal(t, http.StatusBadRequest, postModelRating(t, 7, "seedance-2-0", 6).Code)
	require.Equal(t, http.StatusNotFound, postModelRating(t, 7, "unknown-model", 5).Code)
	require.Equal(t, http.StatusNotFound, getModelRating(t, "unknown-model").Code)
}
