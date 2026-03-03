package logging

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	uuidpkg "github.com/google/uuid"
)

// Attachment type constants
const (
	AttachmentTypeFile     = "file"
	AttachmentTypeFileData = "fileData"
	AttachmentTypeURL      = "url"
)

// BaseAttachmentProps contains shared fields for all attachment types
type BaseAttachmentProps struct {
	ID       string            `json:"id"`
	Name     string            `json:"name,omitempty"`
	MimeType string            `json:"mimeType,omitempty"`
	Size     int64             `json:"size,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// FileAttachment references a file on the local filesystem
type FileAttachment struct {
	BaseAttachmentProps
	Type string `json:"type"` // "file"
	Path string `json:"path"`
}

// FileDataAttachment contains in-memory binary data
type FileDataAttachment struct {
	BaseAttachmentProps
	Type string `json:"type"` // "fileData"
	Data []byte `json:"data"`
}

// UrlAttachment references an external URL
type UrlAttachment struct {
	BaseAttachmentProps
	Type string `json:"type"` // "url"
	URL  string `json:"url"`
}

// Common MIME type mappings by extension
var mimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".json": "application/json",
	".html": "text/html",
	".htm":  "text/html",
	".txt":  "text/plain",
}

// Magic bytes for MIME detection (hex)
const (
	magicPNG  = "89504e47"
	magicJPEG = "ffd8ff"
	magicGIF  = "47494638"
	magicPDF  = "25504446"
	magicZIP  = "504b0304"
)

// PopulateAttachmentFields auto-fills missing fields (name, mimeType, size) based on attachment type.
// Accepts *FileAttachment, *FileDataAttachment, *UrlAttachment, or map[string]interface{}.
// Returns the populated attachment as map[string]interface{} for use in add-attachment log.
func PopulateAttachmentFields(attachment interface{}) (map[string]interface{}, string, error) {
	result := make(map[string]interface{})

	switch a := attachment.(type) {
	case *FileAttachment:
		if a == nil {
			return nil, "", ErrInvalidAttachmentType
		}
		populateFileAttachment(a, result)
		return result, AttachmentTypeFile, nil
	case FileAttachment:
		populateFileAttachment(&a, result)
		return result, AttachmentTypeFile, nil
	case *FileDataAttachment:
		if a == nil {
			return nil, "", ErrInvalidAttachmentType
		}
		populateFileDataAttachment(a, result)
		return result, AttachmentTypeFileData, nil
	case FileDataAttachment:
		populateFileDataAttachment(&a, result)
		return result, AttachmentTypeFileData, nil
	case *UrlAttachment:
		if a == nil {
			return nil, "", ErrInvalidAttachmentType
		}
		populateUrlAttachment(a, result)
		return result, AttachmentTypeURL, nil
	case UrlAttachment:
		populateUrlAttachment(&a, result)
		return result, AttachmentTypeURL, nil
	case map[string]interface{}:
		return populateFromMap(a)
	default:
		return nil, "", ErrInvalidAttachmentType
	}
}

func populateFileAttachment(a *FileAttachment, result map[string]interface{}) {
	if a.ID != "" {
		result["id"] = a.ID
	} else {
		result["id"] = uuidpkg.New().String()
	}
	result["type"] = AttachmentTypeFile
	result["path"] = a.Path
	if a.Name != "" {
		result["name"] = a.Name
	} else {
		result["name"] = filepath.Base(a.Path)
	}
	if a.MimeType != "" {
		result["mimeType"] = a.MimeType
	} else {
		result["mimeType"] = mimeFromPath(a.Path)
	}
	if a.Size > 0 {
		result["size"] = a.Size
	} else {
		if info, err := os.Stat(a.Path); err == nil {
			result["size"] = info.Size()
		}
	}
	if len(a.Tags) > 0 {
		result["tags"] = a.Tags
	}
	if len(a.Metadata) > 0 {
		result["metadata"] = a.Metadata
	}
}

func populateFileDataAttachment(a *FileDataAttachment, result map[string]interface{}) {
	if a.ID != "" {
		result["id"] = a.ID
	} else {
		result["id"] = uuidpkg.New().String()
	}
	result["type"] = AttachmentTypeFileData
	result["data"] = a.Data
	if a.Size > 0 {
		result["size"] = a.Size
	} else if len(a.Data) > 0 {
		result["size"] = int64(len(a.Data))
	}
	if a.MimeType != "" {
		result["mimeType"] = a.MimeType
	} else if len(a.Data) >= 4 {
		result["mimeType"] = mimeFromMagicBytes(a.Data)
	} else {
		result["mimeType"] = "application/octet-stream"
	}
	if a.Name != "" {
		result["name"] = a.Name
	}
	if len(a.Tags) > 0 {
		result["tags"] = a.Tags
	}
	if len(a.Metadata) > 0 {
		result["metadata"] = a.Metadata
	}
}

func populateUrlAttachment(a *UrlAttachment, result map[string]interface{}) {
	if a.ID != "" {
		result["id"] = a.ID
	} else {
		result["id"] = uuidpkg.New().String()
	}
	result["type"] = AttachmentTypeURL
	result["url"] = a.URL
	if a.Name != "" {
		result["name"] = a.Name
	} else if u, err := url.Parse(a.URL); err == nil {
		result["name"] = filepath.Base(u.Path)
		if result["name"] == "" || result["name"] == "." {
			result["name"] = u.Hostname()
		}
	}
	if a.MimeType != "" {
		result["mimeType"] = a.MimeType
	} else if u, err := url.Parse(a.URL); err == nil {
		result["mimeType"] = mimeFromPath(u.Path)
	}
	if a.Size > 0 {
		result["size"] = a.Size
	}
	if len(a.Tags) > 0 {
		result["tags"] = a.Tags
	}
	if len(a.Metadata) > 0 {
		result["metadata"] = a.Metadata
	}
}

func populateFromMap(m map[string]interface{}) (map[string]interface{}, string, error) {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	if result["id"] == nil || result["id"] == "" {
		result["id"] = uuidpkg.New().String()
	}
	typ, ok := result["type"].(string)
	if !ok {
		return nil, "", ErrInvalidAttachmentType
	}
	switch typ {
	case AttachmentTypeFile:
		path, pathOk := result["path"].(string)
		if !pathOk || path == "" {
			return nil, "", fmt.Errorf("%w: %s attachment requires non-empty \"path\"", ErrInvalidAttachmentType, AttachmentTypeFile)
		}
		if result["name"] == nil || result["name"] == "" {
			result["name"] = filepath.Base(path)
		}
		if result["mimeType"] == nil || result["mimeType"] == "" {
			result["mimeType"] = mimeFromPath(path)
		}
		if result["size"] == nil {
			if info, err := os.Stat(path); err == nil {
				result["size"] = info.Size()
			}
		}
		return result, AttachmentTypeFile, nil
	case AttachmentTypeFileData:
		data, dataOk := result["data"].([]byte)
		if !dataOk {
			return nil, "", fmt.Errorf("%w: %s attachment requires \"data\" of type []byte", ErrInvalidAttachmentType, AttachmentTypeFileData)
		}
		if result["size"] == nil {
			result["size"] = int64(len(data))
		}
		if result["mimeType"] == nil || result["mimeType"] == "" {
			if len(data) >= 4 {
				result["mimeType"] = mimeFromMagicBytes(data)
			} else {
				result["mimeType"] = "application/octet-stream"
			}
		}
		return result, AttachmentTypeFileData, nil
	case AttachmentTypeURL:
		urlStr, urlOk := result["url"].(string)
		if !urlOk || urlStr == "" {
			return nil, "", fmt.Errorf("%w: %s attachment requires non-empty \"url\"", ErrInvalidAttachmentType, AttachmentTypeURL)
		}
		if result["name"] == nil || result["name"] == "" {
			if u, err := url.Parse(urlStr); err == nil {
				result["name"] = filepath.Base(u.Path)
				if result["name"] == "" || result["name"] == "." {
					result["name"] = u.Hostname()
				}
			}
		}
		if result["mimeType"] == nil || result["mimeType"] == "" {
			if u, err := url.Parse(urlStr); err == nil {
				result["mimeType"] = mimeFromPath(u.Path)
			}
		}
		return result, AttachmentTypeURL, nil
	default:
		return nil, "", ErrInvalidAttachmentType
	}
}

func mimeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := mimeByExt[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func mimeFromMagicBytes(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	header := hex.EncodeToString(data[:4])
	switch {
	case strings.HasPrefix(header, magicPNG):
		return "image/png"
	case strings.HasPrefix(header, magicJPEG):
		return "image/jpeg"
	case strings.HasPrefix(header, magicGIF):
		return "image/gif"
	case strings.HasPrefix(header, magicPDF) || (len(data) >= 5 && string(data[:5]) == "%PDF-"):
		return "application/pdf"
	case strings.HasPrefix(header, magicZIP):
		return "application/zip"
	default:
		// Check if text
		for i := 0; i < len(data) && i < 512; i++ {
			b := data[i]
			if (b < 32 || b > 126) && b != 9 && b != 10 && b != 13 {
				return "application/octet-stream"
			}
		}
		// Try JSON
		if len(data) > 0 {
			s := string(data)
			if strings.TrimSpace(s) != "" {
				var _ interface{}
				if err := parseJSON(s); err == nil {
					return "application/json"
				}
				if strings.Contains(strings.ToLower(s), "<html") || strings.Contains(strings.ToLower(s), "<!doctype") {
					return "text/html"
				}
				return "text/plain"
			}
		}
		return "application/octet-stream"
	}
}

func parseJSON(s string) error {
	var v interface{}
	return json.Unmarshal([]byte(s), &v)
}
