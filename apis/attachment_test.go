package apis

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUploadUrl_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/sdk/v1/log-repositories/attachments/upload-url" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-maxim-api-key") != "test-key" {
			t.Errorf("expected x-maxim-api-key header")
		}
		key := r.URL.Query().Get("key")
		if key != "repo/trace/123/files/original/file-1" {
			t.Errorf("unexpected key: %s", key)
		}
		mimeType := r.URL.Query().Get("mimeType")
		if mimeType != "image/png" {
			t.Errorf("unexpected mimeType: %s", mimeType)
		}
		size := r.URL.Query().Get("size")
		if size != "1024" {
			t.Errorf("unexpected size: %s", size)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{
				"url": "https://signed-url.example.com/upload",
			},
		})
	}))
	defer server.Close()

	url, err := GetUploadUrl(server.URL, "test-key", "repo/trace/123/files/original/file-1", "image/png", 1024)
	if err != nil {
		t.Fatalf("GetUploadUrl failed: %v", err)
	}
	if url != "https://signed-url.example.com/upload" {
		t.Errorf("expected signed URL, got %s", url)
	}
}

func TestGetUploadUrl_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	_, err := GetUploadUrl(server.URL, "bad-key", "key", "image/png", 100)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if err.Error() != "API error: Invalid API key" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetUploadUrl_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
	}))
	defer server.Close()

	_, err := GetUploadUrl(server.URL, "key", "key", "image/png", 100)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestGetUploadUrl_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := GetUploadUrl(server.URL, "key", "key", "image/png", 100)
	if err == nil {
		t.Fatal("expected error for 401 status")
	}
}

func TestGetUploadUrl_InvalidBaseURL(t *testing.T) {
	_, err := GetUploadUrl("://invalid", "key", "key", "image/png", 100)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestUploadToSignedUrl_Success(t *testing.T) {
	var receivedData []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		receivedContentType = r.Header.Get("Content-Type")
		receivedData = make([]byte, r.ContentLength)
		r.Body.Read(receivedData)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	data := []byte("test file content")
	err := UploadToSignedUrl(server.URL, data, "application/octet-stream")
	if err != nil {
		t.Fatalf("UploadToSignedUrl failed: %v", err)
	}
	if !bytes.Equal(receivedData, data) {
		t.Errorf("expected data %q, got %q", data, receivedData)
	}
	if receivedContentType != "application/octet-stream" {
		t.Errorf("expected Content-Type application/octet-stream, got %s", receivedContentType)
	}
}

func TestUploadToSignedUrl_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := UploadToSignedUrl(server.URL, []byte("data"), "text/plain")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}
