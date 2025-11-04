package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cato-logger/internal/logging"
)

const (
	queryEventsFeed = `query eventsFeed($accountIDs: [ID!]!, $marker: String) {
		eventsFeed(accountIDs: $accountIDs, marker: $marker) {
			marker
			fetchedCount
			accounts {
				id
				errorString
				records {
					fieldsMap
				}
			}
		}
	}`
)

// Client handles communication with the Cato Networks API
type Client struct {
	apiURL    string
	apiKey    string
	accountID string
	timeout   time.Duration
	logger    *logging.Logger
}

// NewClient creates a new API client
func NewClient(apiURL, apiKey, accountID string, timeout time.Duration, logger *logging.Logger) *Client {
	return &Client{
		apiURL:    apiURL,
		apiKey:    apiKey,
		accountID: accountID,
		timeout:   timeout,
		logger:    logger,
	}
}

// FetchEventsPage retrieves a single page of events from the API
func (c *Client) FetchEventsPage(marker string) (*EventsPage, error) {
	reqBody, err := c.buildRequest(marker)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("User-Agent", "Cato-CEF-Forwarder/3.2")

	client := &http.Client{Timeout: c.timeout}

	c.logger.Debug("sending API request", "url", c.apiURL, "has_marker", marker != "")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.logger.Debug("received API response", "status", resp.StatusCode, "body_size", len(body))

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(resp.StatusCode, body)
	}

	var response EventsFeedResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Handle GraphQL errors
	if len(response.Errors) > 0 {
		c.logger.Error("GraphQL error received", "error", response.Errors[0].Message)
		return nil, fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	// Extract events and marker
	events := c.extractEvents(&response)
	page := &EventsPage{
		Events: events,
	}

	if response.Data.EventsFeed.Marker != nil {
		page.NewMarker = *response.Data.EventsFeed.Marker
		// If we got a new marker and events, there might be more data
		page.HasMore = len(events) > 0 && page.NewMarker != ""
	} else {
		page.HasMore = false
	}

	c.logger.Debug("parsed API response",
		"event_count", len(page.Events),
		"has_more", page.HasMore,
		"new_marker", page.NewMarker != "")

	return page, nil
}

// buildRequest constructs the GraphQL request body
func (c *Client) buildRequest(marker string) ([]byte, error) {
	variables := map[string]interface{}{
		"accountIDs": []string{c.accountID},
	}
	if marker != "" {
		variables["marker"] = marker
	}

	req := Request{
		Query:     queryEventsFeed,
		Variables: variables,
	}

	return json.Marshal(req)
}

// extractEvents extracts event records from all accounts in the response
func (c *Client) extractEvents(response *EventsFeedResponse) []map[string]string {
	var allRecords []map[string]string

	for _, account := range response.Data.EventsFeed.Accounts {
		if account.ErrorString != "" {
			c.logger.Warn("account error in response", "account_id", account.ID, "error", account.ErrorString)
			continue
		}

		for _, record := range account.Records {
			allRecords = append(allRecords, record.FieldsMap)
		}
	}

	return allRecords
}

// handleHTTPError provides detailed error messages for different HTTP status codes
func (c *Client) handleHTTPError(statusCode int, body []byte) error {
	c.logger.Error("API HTTP error", "status", statusCode, "body", string(body))

	switch statusCode {
	case 401:
		return fmt.Errorf("authentication failed (401) - check your API key")
	case 403:
		return fmt.Errorf("access forbidden (403) - ensure Events Integration is enabled and API key has eventsFeed permissions")
	case 429:
		return fmt.Errorf("rate limit exceeded (429) - reduce polling frequency or maxEvents")
	case 500, 502, 503, 504:
		return fmt.Errorf("server error (%d) - Cato API experiencing issues", statusCode)
	default:
		return fmt.Errorf("API returned status %d: %s", statusCode, string(body))
	}
}
