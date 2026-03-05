package apis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const attachmentUploadTimeout = 120 * time.Second

// GetUploadUrlResponse represents the response from the upload URL API
type GetUploadUrlResponse struct {
	Data  *struct{ URL string } `json:"data,omitempty"`
	Error *MaximError           `json:"error,omitempty"`
}

// GetUploadUrl fetches a signed URL for uploading an attachment to a log repository.
//
// Parameters:
//   - baseUrl: The base URL of the API endpoint
//   - apiKey: The API key for authentication
//   - key: The storage key for the attachment (format: {repoId}/{entity}/{entityId}/files/original/{fileId})
//   - mimeType: The MIME type of the file
//   - size: The size of the file in bytes
//
// Returns the signed upload URL or an error.
func GetUploadUrl(baseUrl, apiKey, key, mimeType string, size int64) (string, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = "/api/sdk/v1/log-repositories/attachments/upload-url"
	q := u.Query()
	q.Set("key", key)
	q.Set("mimeType", mimeType)
	q.Set("size", strconv.FormatInt(size, 10))
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-maxim-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result GetUploadUrlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if result.Data == nil || result.Data.URL == "" {
		return "", fmt.Errorf("no upload URL in response")
	}
	return result.Data.URL, nil
}

// UploadToSignedUrl uploads binary data to a pre-signed URL (PUT request).
// Uses a 2-minute timeout for large file uploads.
//
// Parameters:
//   - uploadURL: The signed URL from GetUploadUrl
//   - data: The binary content to upload
//   - mimeType: The MIME type for the Content-Type header
func UploadToSignedUrl(uploadURL string, data []byte, mimeType string) error {
	client := &http.Client{Timeout: attachmentUploadTimeout}
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}
	return nil
}
