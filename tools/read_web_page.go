package tools

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/honganh1206/tinker/schema"
)

//go:embed read_web_page.md
var readWebPagePrompt string

var ReadWebPageDefinition = ToolDefinition{
	Name:        ToolNameReadWebPage,
	Description: readWebPagePrompt,
	InputSchema: ReadWebPageInputSchema,
	Function:    ReadWebPage,
}

type ReadWebPageInput struct {
	URL string `json:"url" jsonschema_description:"The URL of the web page to fetch and read."`
}

var ReadWebPageInputSchema = schema.Generate[ReadWebPageInput]()

const maxBodyBytes = 200 * 1024

var (
	reHTMLTags   = regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>|<style[^>]*>[\s\S]*?</style>|<[^>]+>`)
	reWhitespace = regexp.MustCompile(`[ \t]+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

func ReadWebPage(input ToolInput) (string, error) {
	pageInput, err := schema.DecodeRaw[ReadWebPageInput](input.RawInput)
	if err != nil {
		return "", fmt.Errorf("failed to parse read_web_page input: %w", err)
	}

	if pageInput.URL == "" {
		return "", fmt.Errorf("invalid url parameter: url cannot be empty")
	}

	if !strings.HasPrefix(pageInput.URL, "http://") && !strings.HasPrefix(pageInput.URL, "https://") {
		pageInput.URL = "https://" + pageInput.URL
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", pageInput.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Tinker/1.0 (CLI Agent)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	text := extractText(string(body))

	const maxChars = 30000
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n... (content truncated)"
	}

	if strings.TrimSpace(text) == "" {
		return "Page returned no readable text content.", nil
	}

	return text, nil
}

func extractText(html string) string {
	text := reHTMLTags.ReplaceAllString(html, " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = reWhitespace.ReplaceAllString(text, " ")
	text = reBlankLines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
