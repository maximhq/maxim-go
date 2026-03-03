package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPopulateAttachmentFields_FileAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	attach := &FileAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "file-1"},
		Type:                AttachmentTypeFile,
		Path:                filePath,
	}

	result, typ, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeFile {
		t.Errorf("expected type %q, got %q", AttachmentTypeFile, typ)
	}
	if result["id"] != "file-1" {
		t.Errorf("expected id file-1, got %v", result["id"])
	}
	if result["name"] != "test.txt" {
		t.Errorf("expected name test.txt, got %v", result["name"])
	}
	if result["mimeType"] != "text/plain" {
		t.Errorf("expected mimeType text/plain, got %v", result["mimeType"])
	}
	if result["size"] != int64(len(content)) {
		t.Errorf("expected size %d, got %v", len(content), result["size"])
	}
	if result["path"] != filePath {
		t.Errorf("expected path %q, got %v", filePath, result["path"])
	}
}

func TestPopulateAttachmentFields_FileAttachmentWithExplicitFields(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.pdf")
	if err := os.WriteFile(filePath, []byte("pdf content"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	attach := &FileAttachment{
		BaseAttachmentProps: BaseAttachmentProps{
			ID:       "doc-1",
			Name:     "custom-name.pdf",
			MimeType: "application/pdf",
			Size:     100,
		},
		Type: AttachmentTypeFile,
		Path: filePath,
	}

	result, typ, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeFile {
		t.Errorf("expected type %q, got %q", AttachmentTypeFile, typ)
	}
	if result["name"] != "custom-name.pdf" {
		t.Errorf("expected explicit name, got %v", result["name"])
	}
	if result["mimeType"] != "application/pdf" {
		t.Errorf("expected explicit mimeType, got %v", result["mimeType"])
	}
	if result["size"] != int64(100) {
		t.Errorf("expected explicit size 100, got %v", result["size"])
	}
}

func TestPopulateAttachmentFields_FileAttachmentGeneratesIDWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	attach := &FileAttachment{
		BaseAttachmentProps: BaseAttachmentProps{},
		Type:                AttachmentTypeFile,
		Path:                filePath,
	}

	result, _, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected auto-generated ID when empty")
	}
}

func TestPopulateAttachmentFields_FileDataAttachment(t *testing.T) {
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	attach := &FileDataAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "img-1"},
		Type:                AttachmentTypeFileData,
		Data:                pngMagic,
	}

	result, typ, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeFileData {
		t.Errorf("expected type %q, got %q", AttachmentTypeFileData, typ)
	}
	if result["id"] != "img-1" {
		t.Errorf("expected id img-1, got %v", result["id"])
	}
	if result["mimeType"] != "image/png" {
		t.Errorf("expected mimeType image/png from magic bytes, got %v", result["mimeType"])
	}
	if result["size"] != int64(len(pngMagic)) {
		t.Errorf("expected size %d, got %v", len(pngMagic), result["size"])
	}
}

func TestPopulateAttachmentFields_FileDataAttachmentJPEG(t *testing.T) {
	jpegMagic := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	attach := &FileDataAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "jpeg-1"},
		Type:                AttachmentTypeFileData,
		Data:                jpegMagic,
	}

	result, _, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if result["mimeType"] != "image/jpeg" {
		t.Errorf("expected mimeType image/jpeg, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_FileDataAttachmentSmallData(t *testing.T) {
	attach := &FileDataAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "tiny"},
		Type:                AttachmentTypeFileData,
		Data:                []byte{0x01, 0x02},
	}

	result, _, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if result["mimeType"] != "application/octet-stream" {
		t.Errorf("expected application/octet-stream for small data, got %v", result["mimeType"])
	}
	if result["size"] != int64(2) {
		t.Errorf("expected size 2, got %v", result["size"])
	}
}

func TestPopulateAttachmentFields_UrlAttachment(t *testing.T) {
	attach := &UrlAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "url-1"},
		Type:                AttachmentTypeURL,
		URL:                 "https://example.com/path/to/image.png",
	}

	result, typ, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeURL {
		t.Errorf("expected type %q, got %q", AttachmentTypeURL, typ)
	}
	if result["id"] != "url-1" {
		t.Errorf("expected id url-1, got %v", result["id"])
	}
	if result["url"] != "https://example.com/path/to/image.png" {
		t.Errorf("expected url, got %v", result["url"])
	}
	if result["name"] != "image.png" {
		t.Errorf("expected name image.png from URL path, got %v", result["name"])
	}
	if result["mimeType"] != "image/png" {
		t.Errorf("expected mimeType image/png from extension, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_UrlAttachmentWithQuery(t *testing.T) {
	attach := &UrlAttachment{
		BaseAttachmentProps: BaseAttachmentProps{ID: "url-2"},
		Type:                AttachmentTypeURL,
		URL:                 "https://cdn.example.com/files/doc.pdf?token=abc",
	}

	result, _, err := PopulateAttachmentFields(attach)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if result["mimeType"] != "application/pdf" {
		t.Errorf("expected mimeType application/pdf, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_MapFileType(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.json")
	if err := os.WriteFile(filePath, []byte(`{"x":1}`), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	m := map[string]interface{}{
		"id":   "map-file-1",
		"type": AttachmentTypeFile,
		"path": filePath,
	}

	result, typ, err := PopulateAttachmentFields(m)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeFile {
		t.Errorf("expected type %q, got %q", AttachmentTypeFile, typ)
	}
	if result["name"] != "data.json" {
		t.Errorf("expected name data.json, got %v", result["name"])
	}
	if result["mimeType"] != "application/json" {
		t.Errorf("expected mimeType application/json, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_MapFileDataType(t *testing.T) {
	m := map[string]interface{}{
		"id":   "map-data-1",
		"type": AttachmentTypeFileData,
		"data": []byte("plain text content"),
	}

	result, typ, err := PopulateAttachmentFields(m)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeFileData {
		t.Errorf("expected type %q, got %q", AttachmentTypeFileData, typ)
	}
	if result["size"] != int64(18) {
		t.Errorf("expected size 18, got %v", result["size"])
	}
	if result["mimeType"] != "text/plain" {
		t.Errorf("expected mimeType text/plain, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_MapUrlType(t *testing.T) {
	m := map[string]interface{}{
		"id":   "map-url-1",
		"type": AttachmentTypeURL,
		"url":  "https://example.org/file.html",
	}

	result, typ, err := PopulateAttachmentFields(m)
	if err != nil {
		t.Fatalf("PopulateAttachmentFields failed: %v", err)
	}
	if typ != AttachmentTypeURL {
		t.Errorf("expected type %q, got %q", AttachmentTypeURL, typ)
	}
	if result["name"] != "file.html" {
		t.Errorf("expected name file.html, got %v", result["name"])
	}
	if result["mimeType"] != "text/html" {
		t.Errorf("expected mimeType text/html, got %v", result["mimeType"])
	}
}

func TestPopulateAttachmentFields_InvalidType(t *testing.T) {
	result, typ, err := PopulateAttachmentFields("not an attachment")
	if err == nil {
		t.Fatalf("expected non-nil error for invalid input, got err=nil")
	}
	if result != nil {
		t.Errorf("expected nil result for invalid input, got %v", result)
	}
	if typ != "" {
		t.Errorf("expected empty type for invalid input, got %q", typ)
	}
}

func TestPopulateAttachmentFields_MapInvalidType(t *testing.T) {
	m := map[string]interface{}{
		"id":   "bad",
		"type": "invalid",
	}
	result, typ, err := PopulateAttachmentFields(m)
	if err == nil {
		t.Fatalf("expected non-nil error for invalid map type, got err=nil")
	}
	if result != nil {
		t.Errorf("expected nil result for invalid map type, got %v", result)
	}
	if typ != "" {
		t.Errorf("expected empty type for invalid map type, got %q", typ)
	}
}
