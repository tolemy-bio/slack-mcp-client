package slackbot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/tolemy-bio/slack-mcp-client/internal/common/logging"
)

// FileAttachment represents a downloaded and parsed file attachment from Slack
type FileAttachment struct {
	Name     string
	Mimetype string
	Filetype string
	Size     int
	Content  string
}

const (
	maxFileSize       = 10 * 1024 * 1024 // 10MB max download
	maxContentChars   = 50000            // ~50k chars max injected into prompt
	supportedTextTypes = "text csv tsv json xml yaml yml md markdown txt log html htm py go js ts java c cpp h hpp rb rs"
)

// isSupportedTextType checks if a file type can be read as plain text
func isSupportedTextType(filetype string) bool {
	for _, t := range strings.Fields(supportedTextTypes) {
		if strings.EqualFold(filetype, t) {
			return true
		}
	}
	return false
}

// isSupportedMimetype checks if a MIME type can be processed
func isSupportedMimetype(mimetype string) bool {
	if strings.HasPrefix(mimetype, "text/") {
		return true
	}
	switch mimetype {
	case "application/json", "application/xml", "application/x-yaml",
		"application/pdf", "application/csv",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel":
		return true
	}
	return false
}

// ExtractFilesFromMessageEvent extracts file metadata from a MessageEvent
func ExtractFilesFromMessageEvent(ev *slackevents.MessageEvent) []slackevents.File {
	if ev == nil || len(ev.Files) == 0 {
		return nil
	}
	return ev.Files
}

// ExtractFilesFromSlackMessages extracts file metadata from Slack thread reply messages
func ExtractFilesFromSlackMessages(messages []slack.Message) []slack.File {
	var files []slack.File
	for _, msg := range messages {
		files = append(files, msg.Files...)
	}
	return files
}

// DownloadAndParseFile downloads a file from Slack and extracts its text content.
// Uses the bot token for authentication against Slack's private URLs.
func DownloadAndParseFile(botToken string, file slackevents.File, logger *logging.Logger) (*FileAttachment, error) {
	downloadURL := file.URLPrivateDownload
	if downloadURL == "" {
		downloadURL = file.URLPrivate
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("no download URL for file %s", file.Name)
	}

	if file.Size > maxFileSize {
		return &FileAttachment{
			Name:     file.Name,
			Mimetype: file.Mimetype,
			Filetype: file.Filetype,
			Size:     file.Size,
			Content:  fmt.Sprintf("[File too large to process: %s (%d bytes)]", file.Name, file.Size),
		}, nil
	}

	logger.InfoKV("Downloading Slack file", "name", file.Name, "type", file.Filetype, "mime", file.Mimetype, "size", file.Size)

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", file.Name, err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", file.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: HTTP %d", file.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFileSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", file.Name, err)
	}

	content := extractText(file, body, logger)

	if len(content) > maxContentChars {
		content = content[:maxContentChars] + fmt.Sprintf("\n\n[... truncated at %d characters]", maxContentChars)
	}

	return &FileAttachment{
		Name:     file.Name,
		Mimetype: file.Mimetype,
		Filetype: file.Filetype,
		Size:     file.Size,
		Content:  content,
	}, nil
}

// DownloadAndParseSlackFile downloads a slack.File (from thread replies) and extracts text content.
func DownloadAndParseSlackFile(botToken string, file slack.File, logger *logging.Logger) (*FileAttachment, error) {
	evFile := slackevents.File{
		ID:                 file.ID,
		Name:               file.Name,
		Mimetype:           file.Mimetype,
		Filetype:           file.Filetype,
		Size:               file.Size,
		URLPrivate:         file.URLPrivate,
		URLPrivateDownload: file.URLPrivateDownload,
	}
	return DownloadAndParseFile(botToken, evFile, logger)
}

// extractText attempts to extract readable text from file bytes based on type
func extractText(file slackevents.File, data []byte, logger *logging.Logger) string {
	// PDF handling
	if file.Filetype == "pdf" || file.Mimetype == "application/pdf" {
		text := extractPDFText(data, logger)
		if text != "" {
			return text
		}
		return fmt.Sprintf("[PDF file: %s (%d bytes) - could not extract text]", file.Name, len(data))
	}

	// Plain text and code files
	if isSupportedTextType(file.Filetype) || strings.HasPrefix(file.Mimetype, "text/") {
		return string(data)
	}

	// Excel/spreadsheet - return a note about the file
	if strings.Contains(file.Mimetype, "spreadsheet") || strings.Contains(file.Mimetype, "excel") ||
		file.Filetype == "xlsx" || file.Filetype == "xls" {
		return fmt.Sprintf("[Spreadsheet file: %s (%d bytes) - please export as CSV for full text extraction]", file.Name, len(data))
	}

	// Images - describe them
	if strings.HasPrefix(file.Mimetype, "image/") {
		return fmt.Sprintf("[Image file: %s (%s, %d bytes)]", file.Name, file.Mimetype, len(data))
	}

	// Fallback: try to read as text if it looks like text
	if isLikelyText(data) {
		return string(data)
	}

	return fmt.Sprintf("[Binary file: %s (%s, %d bytes) - unsupported format]", file.Name, file.Mimetype, len(data))
}

// extractPDFText extracts text from PDF bytes using a simple approach.
// Looks for text between BT/ET markers and stream/endstream blocks.
func extractPDFText(data []byte, logger *logging.Logger) string {
	content := string(data)

	var textParts []string

	// Method 1: Extract text from BT...ET blocks (text objects)
	for {
		btIdx := strings.Index(content, "BT\n")
		if btIdx == -1 {
			btIdx = strings.Index(content, "BT\r")
		}
		if btIdx == -1 {
			break
		}
		etIdx := strings.Index(content[btIdx:], "ET")
		if etIdx == -1 {
			break
		}

		block := content[btIdx : btIdx+etIdx+2]
		text := extractTextFromPDFBlock(block)
		if text != "" {
			textParts = append(textParts, text)
		}
		content = content[btIdx+etIdx+2:]
	}

	if len(textParts) > 0 {
		result := strings.Join(textParts, "\n")
		result = cleanPDFText(result)
		if len(strings.TrimSpace(result)) > 20 {
			return result
		}
	}

	// Method 2: Look for stream/endstream with FlateDecode - we can't decompress here,
	// so log a note and return empty
	logger.DebugKV("PDF text extraction yielded minimal results, may need a full PDF library", "parts_found", len(textParts))
	return ""
}

// extractTextFromPDFBlock extracts text strings from a PDF BT...ET text block
func extractTextFromPDFBlock(block string) string {
	var parts []string
	i := 0
	for i < len(block) {
		// Look for text in parentheses: (text) Tj or (text) TJ
		if block[i] == '(' {
			depth := 1
			start := i + 1
			i++
			for i < len(block) && depth > 0 {
				if block[i] == '(' && (i == 0 || block[i-1] != '\\') {
					depth++
				} else if block[i] == ')' && (i == 0 || block[i-1] != '\\') {
					depth--
				}
				i++
			}
			if depth == 0 {
				text := block[start : i-1]
				text = strings.ReplaceAll(text, "\\(", "(")
				text = strings.ReplaceAll(text, "\\)", ")")
				text = strings.ReplaceAll(text, "\\\\", "\\")
				if strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
		} else if block[i] == '<' && i+1 < len(block) && block[i+1] != '<' {
			// Hex-encoded text: <hex> Tj
			end := strings.Index(block[i:], ">")
			if end != -1 {
				i += end + 1
			} else {
				i++
			}
		} else {
			i++
		}
	}
	return strings.Join(parts, "")
}

// cleanPDFText removes excessive whitespace and control characters from PDF text
func cleanPDFText(text string) string {
	// Replace common PDF artifacts
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// isLikelyText checks if bytes are likely to be human-readable text
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 1024 {
		sample = sample[:1024]
	}
	nonPrintable := 0
	for _, b := range sample {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(len(sample)) < 0.1
}

// FormatFileAttachmentsForPrompt formats downloaded file attachments into a string
// that can be prepended to the user's message for LLM context
func FormatFileAttachmentsForPrompt(attachments []FileAttachment) string {
	if len(attachments) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("\n\n--- ATTACHED FILES ---\n")
	for i, att := range attachments {
		if i > 0 {
			buf.WriteString("\n---\n")
		}
		buf.WriteString(fmt.Sprintf("File: %s (type: %s, size: %d bytes)\n", att.Name, att.Filetype, att.Size))
		buf.WriteString("Content:\n")
		buf.WriteString(att.Content)
		buf.WriteString("\n")
	}
	buf.WriteString("--- END ATTACHED FILES ---\n")
	return buf.String()
}
