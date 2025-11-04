package preflight

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cato-logger/internal/logging"
)

// CheckResult represents the result of a pre-flight check
type CheckResult struct {
	Name    string
	Passed  bool
	Message string
	Error   error
}

// Checker runs all pre-flight checks before starting the service
type Checker struct {
	logger *logging.Logger
}

// New creates a new pre-flight checker
func New(logger *logging.Logger) *Checker {
	return &Checker{
		logger: logger,
	}
}

// RunAll executes all pre-flight checks and returns results
func (c *Checker) RunAll(
	apiURL, apiKey, accountID string,
	syslogProtocol, syslogAddress string,
	markerFile string,
	timeout time.Duration,
) []CheckResult {
	c.logger.Info("running pre-flight checks")

	results := []CheckResult{
		c.CheckMarkerFileAccess(markerFile),
		c.CheckSyslogConnectivity(syslogProtocol, syslogAddress, timeout),
		c.CheckAPIConnectivity(apiURL, apiKey, accountID, timeout),
	}

	// Summary
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Passed {
			passed++
			c.logger.Info("pre-flight check passed", "check", result.Name, "message", result.Message)
		} else {
			failed++
			c.logger.Error("pre-flight check failed", "check", result.Name, "message", result.Message, "error", result.Error)
		}
	}

	c.logger.Info("pre-flight checks complete", "passed", passed, "failed", failed, "total", len(results))

	return results
}

// CheckMarkerFileAccess verifies we can read/write the marker file
func (c *Checker) CheckMarkerFileAccess(markerFile string) CheckResult {
	result := CheckResult{
		Name: "Marker File Access",
	}

	// Check if directory exists, create if not
	dir := filepath.Dir(markerFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Message = fmt.Sprintf("cannot create marker directory: %s", dir)
		result.Error = err
		return result
	}

	// Try to write a test marker
	testData := []byte("preflight-test")
	if err := os.WriteFile(markerFile, testData, 0644); err != nil {
		result.Message = fmt.Sprintf("cannot write to marker file: %s", markerFile)
		result.Error = err
		return result
	}

	// Try to read it back
	data, err := os.ReadFile(markerFile)
	if err != nil {
		result.Message = fmt.Sprintf("cannot read from marker file: %s", markerFile)
		result.Error = err
		return result
	}

	// Verify content
	if !bytes.Equal(data, testData) {
		result.Message = "marker file read/write mismatch"
		result.Error = fmt.Errorf("wrote '%s' but read '%s'", testData, data)
		return result
	}

	// Clean up test data (but keep the file for the marker manager)
	if err := os.WriteFile(markerFile, []byte(""), 0644); err != nil {
		c.logger.Warn("could not clear test data from marker file", "error", err)
	}

	result.Passed = true
	result.Message = fmt.Sprintf("marker file is readable and writable: %s", markerFile)
	return result
}

// CheckSyslogConnectivity tests connection to the syslog server
func (c *Checker) CheckSyslogConnectivity(protocol, address string, timeout time.Duration) CheckResult {
	result := CheckResult{
		Name: "Syslog Connectivity",
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, protocol, address)
	if err != nil {
		result.Message = fmt.Sprintf("cannot connect to syslog server at %s://%s", protocol, address)
		result.Error = err
		return result
	}
	defer conn.Close()

	// Try sending a test message
	testMsg := []byte("<14>1 " + time.Now().Format(time.RFC3339) + " preflight-test cato-logger - - - Pre-flight connectivity test\n")
	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		result.Message = "cannot set write deadline on syslog connection"
		result.Error = err
		return result
	}

	if _, err := conn.Write(testMsg); err != nil {
		result.Message = fmt.Sprintf("cannot write to syslog server at %s://%s", protocol, address)
		result.Error = err
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("syslog server is reachable at %s://%s", protocol, address)
	return result
}

// CheckAPIConnectivity tests connection to the Cato API with a minimal query
func (c *Checker) CheckAPIConnectivity(apiURL, apiKey, accountID string, timeout time.Duration) CheckResult {
	result := CheckResult{
		Name: "Cato API Connectivity",
	}

	// Create a minimal GraphQL query to test authentication and account access
	// We'll request just the marker and fetchedCount without passing a marker (starts from beginning)
	query := map[string]interface{}{
		"query": `query eventsFeed($accountIDs: [ID!]!) {
			eventsFeed(accountIDs: $accountIDs) {
				marker
				fetchedCount
			}
		}`,
		"variables": map[string]interface{}{
			"accountIDs": []string{accountID},
		},
	}

	bodyBytes, err := json.Marshal(query)
	if err != nil {
		result.Message = "failed to marshal API request"
		result.Error = err
		return result
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		result.Message = "failed to create API request"
		result.Error = err
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("User-Agent", "Cato-CEF-Forwarder/3.2-preflight")

	// Execute request with timeout
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("cannot connect to Cato API at %s", apiURL)
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = "failed to read API response"
		result.Error = err
		return result
	}

	// Check HTTP status
	if resp.StatusCode == 401 {
		result.Message = "API authentication failed - check your API key"
		result.Error = fmt.Errorf("HTTP 401: invalid or missing API key")
		return result
	}

	if resp.StatusCode == 403 {
		result.Message = "API access forbidden - ensure Events Integration is enabled and API key has eventsFeed permissions"
		result.Error = fmt.Errorf("HTTP 403: insufficient permissions")
		return result
	}

	if resp.StatusCode != 200 {
		result.Message = fmt.Sprintf("API returned unexpected status: %d", resp.StatusCode)
		result.Error = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		return result
	}

	// Parse response to check for GraphQL errors
	var apiResponse struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
		Data struct {
			EventsFeed struct {
				Marker       *string `json:"marker"`
				FetchedCount int     `json:"fetchedCount"`
			} `json:"eventsFeed"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		result.Message = "failed to parse API response"
		result.Error = err
		return result
	}

	// Check for GraphQL errors
	if len(apiResponse.Errors) > 0 {
		errMsg := apiResponse.Errors[0].Message
		result.Message = fmt.Sprintf("API GraphQL error: %s", errMsg)
		result.Error = fmt.Errorf("GraphQL error: %s", errMsg)
		return result
	}

	// Check if we got valid data structure
	if apiResponse.Data.EventsFeed.FetchedCount < 0 {
		result.Message = "API returned invalid data structure"
		result.Error = fmt.Errorf("unexpected fetchedCount: %d", apiResponse.Data.EventsFeed.FetchedCount)
		return result
	}

	result.Passed = true
	result.Message = fmt.Sprintf("Cato API is accessible and authenticated (account: %s)", accountID)
	return result
}

// HasFailures checks if any check failed
func HasFailures(results []CheckResult) bool {
	for _, result := range results {
		if !result.Passed {
			return true
		}
	}
	return false
}

// FormatFailures returns a formatted string of all failures
func FormatFailures(results []CheckResult) string {
	var failures []string
	for _, result := range results {
		if !result.Passed {
			failures = append(failures, fmt.Sprintf("  - %s: %s", result.Name, result.Message))
		}
	}
	if len(failures) == 0 {
		return ""
	}
	return fmt.Sprintf("Pre-flight checks failed:\n%s", joinStrings(failures, "\n"))
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
