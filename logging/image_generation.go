package logging

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	uuidpkg "github.com/google/uuid"
	"github.com/maximhq/maxim-go/schemas"
)

// isImageGenerationResult detects an OpenAI-style image generation response:
// top-level "data" array with url or b64_json entries, and no "choices" (chat completions).
func isImageGenerationResult(result interface{}) (*schemas.MaximImageGenerationResult, bool) {
	b, err := json.Marshal(result)
	if err != nil {
		return nil, false
	}
	var top map[string]interface{}
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, false
	}
	if _, hasChoices := top["choices"]; hasChoices {
		return nil, false
	}
	rawData, ok := top["data"].([]interface{})
	if !ok || len(rawData) == 0 {
		return nil, false
	}
	if !isImageGenerationDataArray(rawData) {
		return nil, false
	}
	var resp schemas.MaximImageGenerationResult
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, false
	}
	if len(resp.Data) == 0 {
		return nil, false
	}
	return &resp, true
}

func isImageGenerationDataArray(data []interface{}) bool {
	for _, item := range data {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if emb, ok := m["embedding"]; ok && emb != nil {
			return false
		}
		if u, ok := m["url"].(string); ok && strings.TrimSpace(u) != "" {
			return true
		}
		if b, ok := m["b64_json"].(string); ok && strings.TrimSpace(b) != "" {
			return true
		}
	}
	return false
}

func mimeFromOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	default:
		return ""
	}
}

func extFromMime(mime string) string {
	switch mime {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return "png"
	}
}

func urlAttachmentForGeneratedImage(imageURL, outputFormat string, index int) *UrlAttachment {
	mime := mimeFromOutputFormat(outputFormat)
	if mime == "" {
		if u, err := url.Parse(imageURL); err == nil {
			if q := u.Query().Get("rsct"); q != "" {
				mime = mimeFromOutputFormat(q)
			}
		}
	}
	if mime == "" {
		mime = mimeFromPath(imageURL)
	}
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	ext := extFromMime(mime)
	name := "image." + ext
	if index > 0 {
		name = fmt.Sprintf("image_%d.%s", index, ext)
	}
	return &UrlAttachment{
		BaseAttachmentProps: BaseAttachmentProps{
			ID:       uuidpkg.New().String(),
			Name:     name,
			MimeType: mime,
		},
		Type: AttachmentTypeURL,
		URL:  imageURL,
	}
}

func fileDataAttachmentFromB64JSON(b64, outputFormat string, index int) *FileDataAttachment {
	if b64 == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	mime := mimeFromOutputFormat(outputFormat)
	if mime == "" {
		mime = http.DetectContentType(raw)
	}
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = "image/png"
	}
	ext := extFromMime(mime)
	name := "image." + ext
	if index > 0 {
		name = fmt.Sprintf("image_%d.%s", index, ext)
	}
	return &FileDataAttachment{
		BaseAttachmentProps: BaseAttachmentProps{
			ID:       uuidpkg.New().String(),
			Name:     name,
			MimeType: mime,
		},
		Type: AttachmentTypeFileData,
		Data: raw,
	}
}

func imageGenerationResultToMaximLLMResult(resp *schemas.MaximImageGenerationResult) *schemas.MaximLLMResult {
	content := assistantContentFromImagesResponse(resp)
	u := normalisedGenerationUsage(resp.Usage)
	return &schemas.MaximLLMResult{
		ID:      resp.ID,
		Model:   resp.Model,
		Created: resp.Created,
		Choices: []schemas.MaximLLMChoice{
			{
				Message: schemas.MaximLLMChoiceMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: u,
	}
}

func assistantContentFromImagesResponse(resp *schemas.MaximImageGenerationResult) string {
	for _, d := range resp.Data {
		if strings.TrimSpace(d.RevisedPrompt) != "" {
			return d.RevisedPrompt
		}
	}
	var b strings.Builder
	for _, d := range resp.Data {
		if u := strings.TrimSpace(d.URL); u != "" {
			b.WriteString("\n\n")
			b.WriteString(fmt.Sprintf("![image](%s)", u))
		}
	}
	return b.String()
}

func normalisedGenerationUsage(u *schemas.MaximImageGenerationUsage) schemas.Usage {
	if u == nil {
		return schemas.Usage{}
	}
	total := u.TotalTokens
	if total == 0 && (u.InputTokens != 0 || u.OutputTokens != 0) {
		total = u.InputTokens + u.OutputTokens
	}
	return schemas.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      total,
	}
}

// commitImageGenerationAttachments enqueues upload-attachment commits for each generated image.
func commitImageGenerationAttachments(l *Logger, generationID string, resp *schemas.MaximImageGenerationResult) {
	outFmt := resp.OutputFormat
	for i, d := range resp.Data {
		idx := d.Index
		if idx == 0 && i > 0 {
			idx = i
		}
		switch {
		case strings.TrimSpace(d.URL) != "":
			att := urlAttachmentForGeneratedImage(d.URL, outFmt, idx)
			if att != nil {
				l.writer.commit(newCommitLog(EntityGeneration, generationID, "upload-attachment", att))
			}
		case strings.TrimSpace(d.B64JSON) != "":
			att := fileDataAttachmentFromB64JSON(d.B64JSON, outFmt, idx)
			if att != nil {
				l.writer.commit(newCommitLog(EntityGeneration, generationID, "upload-attachment", att))
			}
		}
	}
}
