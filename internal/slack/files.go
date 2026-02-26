package slackbot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
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
	maxFileSize        = 10 * 1024 * 1024 // 10MB max download
	maxContentChars    = 50000            // ~50k chars max injected into prompt
	supportedTextTypes = "text csv tsv json xml yaml yml md markdown txt log html htm py go js ts java c cpp h hpp rb rs"
)

func isSupportedTextType(filetype string) bool {
	for _, t := range strings.Fields(supportedTextTypes) {
		if strings.EqualFold(filetype, t) {
			return true
		}
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

func extractText(file slackevents.File, data []byte, logger *logging.Logger) string {
	if file.Filetype == "pdf" || file.Mimetype == "application/pdf" {
		text := extractPDFText(data, logger)
		if text != "" {
			return text
		}
		return fmt.Sprintf("[PDF file: %s (%d bytes) - could not extract text]", file.Name, len(data))
	}

	if isSupportedTextType(file.Filetype) || strings.HasPrefix(file.Mimetype, "text/") {
		return string(data)
	}

	if strings.Contains(file.Mimetype, "spreadsheet") || strings.Contains(file.Mimetype, "excel") ||
		file.Filetype == "xlsx" || file.Filetype == "xls" {
		return fmt.Sprintf("[Spreadsheet file: %s (%d bytes) - please export as CSV for full text extraction]", file.Name, len(data))
	}

	if strings.HasPrefix(file.Mimetype, "image/") {
		return fmt.Sprintf("[Image file: %s (%s, %d bytes)]", file.Name, file.Mimetype, len(data))
	}

	if isLikelyText(data) {
		return string(data)
	}

	return fmt.Sprintf("[Binary file: %s (%s, %d bytes) - unsupported format]", file.Name, file.Mimetype, len(data))
}

// extractPDFText uses ledongthuc/pdf to properly extract text from PDF bytes,
// handling compressed streams (FlateDecode) and all standard PDF encodings.
func extractPDFText(data []byte, logger *logging.Logger) string {
	reader := bytes.NewReader(data)
	r, err := pdflib.NewReader(reader, int64(len(data)))
	if err != nil {
		logger.WarnKV("PDF reader initialization failed", "error", err)
		return ""
	}

	numPages := r.NumPage()
	if numPages == 0 {
		return ""
	}

	// Method 1: Try GetPlainText for a clean extraction
	plainReader, err := r.GetPlainText()
	if err == nil {
		var buf bytes.Buffer
		buf.ReadFrom(plainReader)
		text := strings.TrimSpace(buf.String())
		if len(text) > 20 {
			logger.InfoKV("PDF text extracted via GetPlainText", "pages", numPages, "chars", len(text))
			return text
		}
	}

	// Method 2: Fall back to page-by-page row extraction for better layout
	var allText []string
	for pageIdx := 1; pageIdx <= numPages; pageIdx++ {
		p := r.Page(pageIdx)
		if p.V.IsNull() {
			continue
		}

		rows, err := p.GetTextByRow()
		if err != nil {
			logger.DebugKV("Failed to get text by row for page", "page", pageIdx, "error", err)
			continue
		}

		for _, row := range rows {
			var rowParts []string
			for _, word := range row.Content {
				if strings.TrimSpace(word.S) != "" {
					rowParts = append(rowParts, word.S)
				}
			}
			if len(rowParts) > 0 {
				allText = append(allText, strings.Join(rowParts, " "))
			}
		}

		if pageIdx < numPages {
			allText = append(allText, "")
		}
	}

	result := strings.Join(allText, "\n")
	result = strings.TrimSpace(result)
	if len(result) > 20 {
		logger.InfoKV("PDF text extracted via row-by-row", "pages", numPages, "chars", len(result))
		return result
	}

	logger.WarnKV("PDF text extraction yielded minimal results", "pages", numPages)
	return ""
}

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
