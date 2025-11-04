package api

import (
	"fmt"
	"time"
)

// FetchWithRetry attempts to fetch events with retry logic
func (c *Client) FetchWithRetry(marker string, maxAttempts int, retryDelay time.Duration) (*EventsPage, error) {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			c.logger.Info("retrying API request",
				"attempt", attempt+1,
				"max_attempts", maxAttempts,
				"delay", retryDelay.String())
			time.Sleep(retryDelay)
		}

		page, err := c.FetchEventsPage(marker)
		if err == nil {
			if attempt > 0 {
				c.logger.Info("API request recovered", "retries", attempt)
			}
			return page, nil
		}

		lastErr = err
		c.logger.Warn("API request failed",
			"attempt", attempt+1,
			"error", err.Error())
	}

	return nil, fmt.Errorf("all %d retry attempts failed, last error: %w", maxAttempts, lastErr)
}
