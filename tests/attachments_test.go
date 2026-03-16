package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/apis"
	"github.com/maximhq/maxim-go/logging"
)

var (
	baseUrl       string
	apiKey        string
	logRepoId     string
	testImagePath = "/Users/rohanbarde/Downloads/test_image.jpg" // Add the path to the test image here
	testImageURL  = "https://www.hollywoodreporter.com/wp-content/uploads/2014/06/optimuskneesstill.jpg?w=2000&h=1126&crop=1"
)

func init() {
	baseUrl = getEnvOrDefault("MAXIM_BASE_URL", "https://app.getmaxim.ai")
	apiKey = os.Getenv("MAXIM_API_KEY")
	logRepoId = os.Getenv("MAXIM_LOG_REPO_ID")
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// captureServer captures requests for verification
type captureServer struct {
	mu            sync.Mutex
	uploadURLReqs []*http.Request
	pushLogsReqs  []struct {
		req  *http.Request
		body string
	}
	uploadReqs []struct {
		req  *http.Request
		body []byte
	}
}

func (s *captureServer) handleUploadURL(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.uploadURLReqs = append(s.uploadURLReqs, r)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": map[string]string{
			"url": "https://signed.example.com/upload",
		},
	})
}

func (s *captureServer) handlePushLogs(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.pushLogsReqs = append(s.pushLogsReqs, struct {
		req  *http.Request
		body string
	}{r, string(body)})
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{})
}

func (s *captureServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.uploadReqs = append(s.uploadReqs, struct {
		req  *http.Request
		body []byte
	}{r, body})
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func TestTraceAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	trace.End()

	// Should not panic; attachment is queued for flush
	logger.Flush()
}

func TestSpanAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	span := trace.AddSpan(&logging.SpanConfig{Id: uuid.New().String()})
	span.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	span.End()
	trace.End()

	logger.Flush()
}

func TestGenerationAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	gen := trace.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4",
	})
	gen.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	gen.End()
	trace.End()

	logger.Flush()
}

func TestLoggerTraceAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	traceId := uuid.New().String()
	logger.Trace(&logging.TraceConfig{Id: traceId})
	logger.TraceAddAttachment(traceId, &logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	logger.EndTrace(traceId)

	logger.Flush()
}

func TestLoggerSpanAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	spanId := uuid.New().String()
	trace.AddSpan(&logging.SpanConfig{Id: spanId})
	logger.SpanAddAttachment(spanId, &logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	logger.EndSpan(spanId)
	trace.End()

	logger.Flush()
}

func TestLoggerGenerationAddAttachment(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	genId := uuid.New().String()
	trace.AddGeneration(&logging.GenerationConfig{
		Id:       genId,
		Provider: "openai",
		Model:    "gpt-4",
	})
	logger.GenerationAddAttachment(genId, &logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	logger.EndGeneration(genId)
	trace.End()

	logger.Flush()
}

func TestAddAttachmentWithFileDataAttachment(t *testing.T) {
	data, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(&logging.FileDataAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeFileData,
		Data:                data,
	})
	trace.End()

	logger.Flush()
}

func TestAddAttachmentWithMap(t *testing.T) {
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(map[string]interface{}{
		"id":   uuid.New().String(),
		"type": logging.AttachmentTypeURL,
		"url":  testImageURL,
	})
	trace.End()

	logger.Flush()
}

func TestAttachmentFlow_UrlAttachment_EnqueuesAddAttachment(t *testing.T) {
	capture := &captureServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sdk/v1/log-repositories/attachments/upload-url", capture.handleUploadURL)
	mux.HandleFunc("/api/sdk/v3/log", capture.handlePushLogs)

	server := httptest.NewServer(mux)
	defer server.Close()

	logger := logging.NewLogger(server.URL, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	attachmentId := uuid.New().String()
	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: attachmentId},
		Type:                logging.AttachmentTypeURL,
		URL:                 testImageURL,
	})
	trace.End()

	logger.Flush()

	capture.mu.Lock()
	pushCount := len(capture.pushLogsReqs)
	capture.mu.Unlock()

	if pushCount == 0 {
		t.Error("expected at least one push logs request")
	}

	// URL attachments don't need GetUploadUrl; add-attachment should be in push logs
	body := capture.pushLogsReqs[0].body
	if body == "" {
		t.Error("expected non-empty push logs body")
	}
	if !contains(body, "add-attachment") {
		t.Errorf("expected add-attachment in push logs body: %s", body)
	}
	if !contains(body, attachmentId) {
		t.Errorf("expected attachment id in push logs: %s", body)
	}
}

func TestAttachmentFlow_FileDataAttachment_UploadsAndEnqueues(t *testing.T) {
	capture := &captureServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sdk/v1/log-repositories/attachments/upload-url", capture.handleUploadURL)
	mux.HandleFunc("/api/sdk/v3/log", capture.handlePushLogs)

	// Simulate signed URL upload - redirect to our server
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.handleUpload(w, r)
	}))
	defer uploadServer.Close()

	logServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sdk/v1/log-repositories/attachments/upload-url" {
			// Track the request in capture
			capture.mu.Lock()
			capture.uploadURLReqs = append(capture.uploadURLReqs, r)
			capture.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"url": uploadServer.URL},
			})
			return
		}
		if r.URL.Path == "/api/sdk/v3/log" {
			capture.handlePushLogs(w, r)
			return
		}
	}))
	defer logServer.Close()

	logger := logging.NewLogger(logServer.URL, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	data, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(&logging.FileDataAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeFileData,
		Data:                data,
	})
	trace.End()

	logger.Flush()

	capture.mu.Lock()
	uploadURLCount := len(capture.uploadURLReqs)
	uploadCount := len(capture.uploadReqs)
	pushCount := len(capture.pushLogsReqs)
	capture.mu.Unlock()

	if uploadURLCount == 0 {
		t.Error("expected GetUploadUrl to be called for file data attachment")
	}
	if uploadCount == 0 {
		t.Error("expected UploadToSignedUrl to be called for file data attachment")
	}
	if pushCount == 0 {
		t.Error("expected PushLogs to be called with add-attachment")
	}
}

func TestAttachmentFlow_FileAttachment_UploadsFromPath(t *testing.T) {
	if _, err := os.Stat(testImagePath); err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	capture := &captureServer{}
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.handleUpload(w, r)
	}))
	defer uploadServer.Close()

	logServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sdk/v1/log-repositories/attachments/upload-url" {
			// Track the request in capture
			capture.mu.Lock()
			capture.uploadURLReqs = append(capture.uploadURLReqs, r)
			capture.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"url": uploadServer.URL},
			})
			return
		}
		if r.URL.Path == "/api/sdk/v3/log" {
			capture.handlePushLogs(w, r)
			return
		}
	}))
	defer logServer.Close()

	logger := logging.NewLogger(logServer.URL, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	content, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Fatalf("failed to read test image: %v", err)
	}

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.AddAttachment(&logging.FileAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{ID: uuid.New().String()},
		Type:                logging.AttachmentTypeFile,
		Path:                testImagePath,
	})
	trace.End()

	logger.Flush()

	capture.mu.Lock()
	uploadCount := len(capture.uploadReqs)
	capture.mu.Unlock()

	if uploadCount == 0 {
		t.Error("expected file to be uploaded")
	} else if len(capture.uploadReqs[0].body) != len(content) {
		t.Errorf("expected uploaded body size %d, got %d", len(content), len(capture.uploadReqs[0].body))
	}
}

func TestGetUploadUrl_Integration(t *testing.T) {
	if apiKey == "" || logRepoId == "" {
		t.Skipf("skipping integration test: MAXIM_API_KEY and MAXIM_LOG_REPO_ID required")
	}
	url, err := apis.GetUploadUrl(baseUrl, apiKey, logRepoId+"/trace/t1/files/original/f1", "image/png", 100)
	if err != nil {
		t.Fatalf("GetUploadUrl failed: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty upload URL")
	}
}

func ptr(b bool) *bool {
	return &b
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
