package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed web_search.md
var webSearchPrompt string

var WebSearchDefinition = ToolDefinition{
	Name:        ToolNameWebSearch,
	Description: webSearchPrompt,
	InputSchema: WebSearchInputSchema,
	Function:    RunWebSearchTool,
}

type WebSearchInput struct {
	Query string `json:"query" jsonschema_description:"The search query to look up on the web."`
}

var WebSearchInputSchema = generate[WebSearchInput]()

type braveSearchResponse struct {
	Web struct {
		Results []braveSearchResult `json:"results"`
	} `json:"web"`
}

type braveSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func RunWebSearchTool(ctx context.Context, args json.RawMessage) (string, error) {
	searchInput, err := decode[WebSearchInput](args)
	if err != nil {
		return "", fmt.Errorf("failed to parse web_search input: %w", err)
	}

	if searchInput.Query == "" {
		return "", fmt.Errorf("invalid query parameter: query cannot be empty")
	}

	// TODO: There should be a registry for all these keys
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("BRAVE_SEARCH_API_KEY environment variable is not set")
	}

	endpoint := "https://api.search.brave.com/res/v1/web/search"
	params := url.Values{}
	params.Set("q", searchInput.Query)
	params.Set("count", "5")

	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("web search returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var searchResp braveSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(searchResp.Web.Results) == 0 {
		return "No results found for: " + searchInput.Query, nil
	}

	var sb strings.Builder
	for i, r := range searchResp.Web.Results {
		fmt.Fprintf(&sb, "%d. %s\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}

	return sb.String(), nil
}
