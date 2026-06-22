package openai

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestConvertImageEditRequestMultipart verifies that ConvertImageRequest
// re-serializes multipart image edit requests with all fields (including
// stream) and the file intact, both when the form was already parsed and when
// it must be re-parsed from the reusable body.
func TestConvertImageEditRequestMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newMultipartContext := func(t *testing.T, prompt string) *gin.Context {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "gpt-image-1"))
		require.NoError(t, writer.WriteField("prompt", prompt))
		require.NoError(t, writer.WriteField("stream", "true"))
		require.NoError(t, writer.WriteField("partial_images", "3"))
		part, err := writer.CreateFormFile("image", "input.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("fake image"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return c
	}

	convertAndReplay := func(t *testing.T, c *gin.Context, prompt string) {
		info := &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeImagesEdits,
		}
		request := dto.ImageRequest{
			Model:  "gpt-image-1",
			Prompt: prompt,
			Stream: common.GetPointer(true),
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
		require.NoError(t, err)
		convertedBody, ok := converted.(*bytes.Buffer)
		require.True(t, ok)

		replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
		replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

		require.Equal(t, "gpt-image-1", replayedRequest.PostForm.Get("model"))
		require.Equal(t, prompt, replayedRequest.PostForm.Get("prompt"))
		require.Equal(t, "true", replayedRequest.PostForm.Get("stream"))
		require.Equal(t, "3", replayedRequest.PostForm.Get("partial_images"))
		require.Len(t, replayedRequest.MultipartForm.File["image"], 1)

		file, err := replayedRequest.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("fake image"), fileBytes)
	}

	t.Run("with pre-parsed form", func(t *testing.T) {
		prompt := "edit this image"
		c := newMultipartContext(t, prompt)
		require.NoError(t, c.Request.ParseMultipartForm(32<<20))

		convertAndReplay(t, c, prompt)
	})

	t.Run("re-parses reusable body when form is missing", func(t *testing.T) {
		prompt := "edit without pre-parsed form"
		c := newMultipartContext(t, prompt)

		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		c.Request.Body = io.NopCloser(storage)
		c.Request.MultipartForm = nil
		c.Request.PostForm = nil

		convertAndReplay(t, c, prompt)
	})
}

func TestConvertImageEditRequestJSONDataURLToMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	imageBytes := []byte("json image")
	payload := []byte(`{
		"model":"gpt-image-2",
		"prompt":"make this sharper",
		"image":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(imageBytes) + `",
		"size":"1024x1024",
		"quality":"high",
		"n":1
	}`)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	request := dto.ImageRequest{}
	require.NoError(t, common.Unmarshal(payload, &request))

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)

	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)

	replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
	replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

	require.Equal(t, "gpt-image-2", replayedRequest.PostForm.Get("model"))
	require.Equal(t, "make this sharper", replayedRequest.PostForm.Get("prompt"))
	require.Equal(t, "1024x1024", replayedRequest.PostForm.Get("size"))
	require.Equal(t, "high", replayedRequest.PostForm.Get("quality"))
	require.Equal(t, "1", replayedRequest.PostForm.Get("n"))
	require.Len(t, replayedRequest.MultipartForm.File["image"], 1)

	file, err := replayedRequest.MultipartForm.File["image"][0].Open()
	require.NoError(t, err)
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, imageBytes, fileBytes)
}

func TestConvertImageEditRequestJSONImageURLObjectToMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload := []byte(`{
		"model":"gpt-image-2",
		"prompt":"use this reference",
		"image_url":{"url":"https://example.com/input.png"},
		"size":"1024x1024"
	}`)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	request := dto.ImageRequest{}
	require.NoError(t, common.Unmarshal(payload, &request))

	converted, err := (&Adaptor{}).ConvertImageRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	}, request)
	require.NoError(t, err)

	convertedBody, ok := converted.(*bytes.Buffer)
	require.True(t, ok)

	replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
	replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

	require.Equal(t, "gpt-image-2", replayedRequest.PostForm.Get("model"))
	require.Equal(t, "use this reference", replayedRequest.PostForm.Get("prompt"))
	require.Equal(t, "1024x1024", replayedRequest.PostForm.Get("size"))
	require.Equal(t, "https://example.com/input.png", replayedRequest.PostForm.Get("image_url"))
	require.Empty(t, replayedRequest.MultipartForm.File)
}
