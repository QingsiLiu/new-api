package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	asyncOutputArchiveEnabledEnv   = "GEILI_ASYNC_OUTPUT_ARCHIVE_ENABLED"
	asyncOutputArchiveEndpointEnv  = "GEILI_SPACES_ENDPOINT"
	asyncOutputArchiveRegionEnv    = "GEILI_SPACES_REGION"
	asyncOutputArchiveBucketEnv    = "GEILI_SPACES_BUCKET"
	asyncOutputArchiveAccessKeyEnv = "GEILI_SPACES_ACCESS_KEY"
	asyncOutputArchiveSecretKeyEnv = "GEILI_SPACES_SECRET_KEY"
	asyncOutputArchivePublicEnv    = "GEILI_SPACES_PUBLIC_BASE_URL"
	asyncOutputArchivePrefixEnv    = "GEILI_SPACES_PREFIX"

	asyncOutputArchiveCacheControl = "public, max-age=259200"
)

type asyncOutputArchiveConfig struct {
	Enabled       bool
	Endpoint      string
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	PublicBaseURL string
	Prefix        string
}

type asyncOutputArchiveObject struct {
	Bucket       string
	Key          string
	ContentType  string
	CacheControl string
	Body         io.Reader
}

type asyncOutputArchivePayload struct {
	Body        io.ReadSeekCloser
	Size        int
	ContentType string
}

var asyncOutputArchiveUploadForTest func(context.Context, asyncOutputArchiveObject) (string, error)

func archiveAsyncTaskImageOutputs(ctx context.Context, task *model.Task, request asyncTaskRequest, outputs []asyncTaskStoredOutput) ([]asyncTaskStoredOutput, error) {
	if request.Kind != asyncTaskKindImage {
		return outputs, nil
	}
	cfg, err := loadAsyncOutputArchiveConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return outputs, nil
	}
	archived := make([]asyncTaskStoredOutput, 0, len(outputs))
	for index, output := range outputs {
		next, err := archiveAsyncTaskImageOutput(ctx, cfg, task, request, output, index)
		if err != nil {
			return nil, err
		}
		archived = append(archived, next)
	}
	return archived, nil
}

func archiveAsyncTaskImageOutput(ctx context.Context, cfg asyncOutputArchiveConfig, task *model.Task, request asyncTaskRequest, output asyncTaskStoredOutput, index int) (asyncTaskStoredOutput, error) {
	payload, err := asyncTaskImageOutputPayload(ctx, task, output)
	if err != nil {
		return asyncTaskStoredOutput{}, err
	}
	defer payload.Body.Close()

	key := asyncOutputArchiveKey(cfg, request.Model, task.TaskID, index, payload.ContentType)
	if _, err := payload.Body.Seek(0, io.SeekStart); err != nil {
		return asyncTaskStoredOutput{}, errors.New("failed to archive async image output")
	}
	publicURL, err := uploadAsyncOutputArchiveObject(ctx, cfg, asyncOutputArchiveObject{
		Bucket:       cfg.Bucket,
		Key:          key,
		ContentType:  payload.ContentType,
		CacheControl: asyncOutputArchiveCacheControl,
		Body:         payload.Body,
	})
	if err != nil {
		return asyncTaskStoredOutput{}, errors.New("failed to archive async image output")
	}
	return asyncTaskStoredOutput{
		MimeType: payload.ContentType,
		URL:      publicURL,
		Size:     payload.Size,
	}, nil
}

func asyncTaskImageOutputPayload(ctx context.Context, task *model.Task, output asyncTaskStoredOutput) (asyncOutputArchivePayload, error) {
	if rawURL := strings.TrimSpace(output.URL); rawURL != "" {
		return downloadAsyncTaskImageOutputToTempFile(ctx, task, output)
	}
	if encoded := strings.TrimSpace(output.Content); encoded != "" {
		return decodeAsyncTaskImageOutputToTempFile(encoded, output.MimeType)
	}
	return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
}

func downloadAsyncTaskImageOutputToTempFile(parentCtx context.Context, task *model.Task, output asyncTaskStoredOutput) (asyncOutputArchivePayload, error) {
	rawURL := strings.TrimSpace(output.URL)
	ctx, cancel := context.WithTimeout(parentCtx, asyncTaskHTTPTimeoutDuration())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	if channel := asyncTaskAuthenticatedContentChannel(task, rawURL); channel != nil {
		request.Header.Set("Authorization", "Bearer "+channel.Key)
	}
	response, err := asyncTaskHTTPClient.Do(request)
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	file, err := os.CreateTemp("", "new-api-async-image-*")
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()
	size64, err := io.Copy(file, response.Body)
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	contentType, err := detectAsyncImageContentType(file, firstAsyncNonEmpty(response.Header.Get("Content-Type"), output.MimeType))
	if err != nil {
		return asyncOutputArchivePayload{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	cleanupOnError = false
	return asyncOutputArchivePayload{
		Body:        &removingReadSeekCloser{ReadSeekCloser: file, path: file.Name()},
		Size:        int(size64),
		ContentType: contentType,
	}, nil
}

func decodeAsyncTaskImageOutputToTempFile(encoded string, fallbackMimeType string) (asyncOutputArchivePayload, error) {
	file, err := os.CreateTemp("", "new-api-async-image-*")
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()
	size64, err := io.Copy(file, base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded)))
	if err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	contentType, err := detectAsyncImageContentType(file, fallbackMimeType)
	if err != nil {
		return asyncOutputArchivePayload{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return asyncOutputArchivePayload{}, errors.New("failed to archive async image output")
	}
	cleanupOnError = false
	return asyncOutputArchivePayload{
		Body:        &removingReadSeekCloser{ReadSeekCloser: file, path: file.Name()},
		Size:        int(size64),
		ContentType: contentType,
	}, nil
}

func detectAsyncImageContentType(reader io.ReadSeeker, preferred string) (string, error) {
	buffer := make([]byte, 512)
	n, _ := reader.Read(buffer)
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("failed to archive async image output")
	}
	sniffed := http.DetectContentType(buffer[:n])
	normalized := normalizeAsyncImageContentType(preferred)
	sniffed = normalizeAsyncImageContentType(sniffed)
	if !strings.HasPrefix(sniffed, "image/") {
		if sniffed != "application/octet-stream" || !strings.HasPrefix(normalized, "image/") {
			return "", errors.New("failed to archive async image output")
		}
	}
	if normalized == "" || normalized == "application/octet-stream" {
		normalized = sniffed
	}
	if !strings.HasPrefix(normalized, "image/") {
		return "", errors.New("failed to archive async image output")
	}
	return normalized, nil
}

func normalizeAsyncImageContentType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err == nil {
		return strings.ToLower(mediaType)
	}
	return strings.ToLower(value)
}

func loadAsyncOutputArchiveConfig() (asyncOutputArchiveConfig, error) {
	cfg := asyncOutputArchiveConfig{
		Enabled:       common.GetEnvOrDefaultBool(asyncOutputArchiveEnabledEnv, false),
		Endpoint:      strings.TrimRight(strings.TrimSpace(os.Getenv(asyncOutputArchiveEndpointEnv)), "/"),
		Region:        strings.TrimSpace(os.Getenv(asyncOutputArchiveRegionEnv)),
		Bucket:        strings.TrimSpace(os.Getenv(asyncOutputArchiveBucketEnv)),
		AccessKey:     strings.TrimSpace(os.Getenv(asyncOutputArchiveAccessKeyEnv)),
		SecretKey:     strings.TrimSpace(os.Getenv(asyncOutputArchiveSecretKeyEnv)),
		PublicBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv(asyncOutputArchivePublicEnv)), "/"),
		Prefix:        strings.Trim(strings.TrimSpace(common.GetEnvOrDefaultString(asyncOutputArchivePrefixEnv, "image")), "/"),
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	missing := []string{}
	if cfg.Endpoint == "" {
		missing = append(missing, asyncOutputArchiveEndpointEnv)
	}
	if cfg.Region == "" {
		missing = append(missing, asyncOutputArchiveRegionEnv)
	}
	if cfg.Bucket == "" {
		missing = append(missing, asyncOutputArchiveBucketEnv)
	}
	if cfg.AccessKey == "" {
		missing = append(missing, asyncOutputArchiveAccessKeyEnv)
	}
	if cfg.SecretKey == "" {
		missing = append(missing, asyncOutputArchiveSecretKeyEnv)
	}
	if cfg.PublicBaseURL == "" {
		missing = append(missing, asyncOutputArchivePublicEnv)
	}
	if len(missing) > 0 {
		return asyncOutputArchiveConfig{}, fmt.Errorf("async image output archive is missing configuration: %s", strings.Join(missing, ", "))
	}
	cfg.Endpoint = normalizeAsyncOutputArchiveEndpoint(cfg.Endpoint, cfg.Bucket)
	return cfg, nil
}

func uploadAsyncOutputArchiveObject(ctx context.Context, cfg asyncOutputArchiveConfig, object asyncOutputArchiveObject) (string, error) {
	if asyncOutputArchiveUploadForTest != nil {
		return asyncOutputArchiveUploadForTest(ctx, object)
	}
	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	})
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(object.Bucket),
		Key:          aws.String(object.Key),
		Body:         object.Body,
		ContentType:  aws.String(object.ContentType),
		CacheControl: aws.String(object.CacheControl),
		ACL:          s3types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return "", err
	}
	return cfg.PublicBaseURL + "/" + object.Key, nil
}

func asyncOutputArchiveKey(cfg asyncOutputArchiveConfig, modelName string, taskID string, index int, contentType string) string {
	now := time.Now().UTC()
	parts := []string{
		cfg.Prefix,
		sanitizeAsyncOutputArchivePathPart(modelName),
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		fmt.Sprintf("%s-%d%s", sanitizeAsyncOutputArchivePathPart(taskID), index, asyncImageExtension(contentType)),
	}
	return path.Join(parts...)
}

func sanitizeAsyncOutputArchivePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		allowed := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.'
		if allowed {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "unknown"
	}
	return result
}

func asyncImageExtension(contentType string) string {
	switch normalizeAsyncImageContentType(contentType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	default:
		extensions, err := mime.ExtensionsByType(contentType)
		if err == nil && len(extensions) > 0 {
			return extensions[0]
		}
		return ".bin"
	}
}

func normalizeAsyncOutputArchiveEndpoint(endpoint string, bucket string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || bucket == "" {
		return endpoint
	}
	prefix := bucket + "."
	if strings.HasPrefix(parsed.Host, prefix) {
		parsed.Host = strings.TrimPrefix(parsed.Host, prefix)
	}
	return strings.TrimRight(parsed.String(), "/")
}

type removingReadSeekCloser struct {
	io.ReadSeekCloser
	path string
}

func (r *removingReadSeekCloser) Close() error {
	err := r.ReadSeekCloser.Close()
	removeErr := os.Remove(r.path)
	if err != nil {
		return err
	}
	return removeErr
}
