package processor

import (
	"context"
	"fmt"
	"time"

	"cato-logger/internal/api"
	"cato-logger/internal/cef"
	"cato-logger/internal/config"
	"cato-logger/internal/logging"
	"cato-logger/internal/marker"
	"cato-logger/internal/syslog"
)

// Processor orchestrates the event fetching and forwarding pipeline
type Processor struct {
	cfg           *config.Config
	apiClient     *api.Client
	syslogWriter  *syslog.Writer
	cefFormatter  *cef.Formatter
	markerManager *marker.Manager
	stats         *Stats
	logger        *logging.Logger
}

// New creates a new event processor
func New(
	cfg *config.Config,
	apiClient *api.Client,
	syslogWriter *syslog.Writer,
	cefFormatter *cef.Formatter,
	markerManager *marker.Manager,
	stats *Stats,
	logger *logging.Logger,
) *Processor {
	return &Processor{
		cfg:           cfg,
		apiClient:     apiClient,
		syslogWriter:  syslogWriter,
		cefFormatter:  cefFormatter,
		markerManager: markerManager,
		stats:         stats,
		logger:        logger,
	}
}

// ProcessEvents fetches and forwards all available events with pagination
func (p *Processor) ProcessEvents(ctx context.Context) error {
	totalEventsProcessed := 0
	paginationCount := 0
	currentMarker := p.markerManager.Get()
	markerUpdates := 0

	p.stats.IncrementAPIRequests()

	pollStart := time.Now()
	pollEnd := pollStart
	numErrors := 0

	p.logger.Debug("starting event processing cycle", "has_marker", currentMarker != "")

	for paginationCount < p.cfg.MaxPagination {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during pagination")
		default:
		}

		// Fetch events page with retry logic
		page, err := p.apiClient.FetchWithRetry(
			currentMarker,
			p.cfg.RetryAttempts,
			time.Duration(p.cfg.RetryDelay)*time.Second,
		)

		if err != nil {
			numErrors++
			p.logger.Error("failed to fetch events page",
				"page", paginationCount+1,
				"error", err.Error())
			break
		}

		paginationCount++
		pollEnd = time.Now()

		p.logger.Debug("fetched events page",
			"page", paginationCount,
			"event_count", len(page.Events),
			"has_more", page.HasMore)

		if len(page.Events) > 0 {
			forwarded, err := p.forwardEvents(page.Events)
			if err != nil {
				numErrors++
				p.logger.Error("failed to forward events",
					"page", paginationCount,
					"error", err.Error())
				continue
			}
			totalEventsProcessed += forwarded
			p.stats.IncrementEventsForwarded(int64(forwarded))
		}

		// Update marker if it changed
		if page.NewMarker != "" && page.NewMarker != currentMarker {
			currentMarker = page.NewMarker
			if err := p.markerManager.Update(currentMarker); err != nil {
				numErrors++
				p.logger.Error("failed to save marker", "error", err.Error())
			} else {
				markerUpdates++
			}
		}

		if !page.HasMore {
			p.logger.Debug("no more events available")
			break
		}

		// Brief pause between pagination requests
		if paginationCount < p.cfg.MaxPagination {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Calculate statistics
	duration := pollEnd.Sub(pollStart)
	eventsPerSecond := 0.0
	if duration.Seconds() > 0 && totalEventsProcessed > 0 {
		eventsPerSecond = float64(totalEventsProcessed) / duration.Seconds()
	}

	p.logger.Info("processing cycle complete",
		"duration_ms", duration.Milliseconds(),
		"events_processed", totalEventsProcessed,
		"total_events", p.stats.GetTotalEvents(),
		"events_per_second", fmt.Sprintf("%.2f", eventsPerSecond),
		"pages", paginationCount,
		"errors", numErrors,
		"marker_updates", markerUpdates)

	return nil
}

// forwardEvents sends events to syslog as CEF messages
func (p *Processor) forwardEvents(events []map[string]string) (int, error) {
	var forwardedCount int

	for _, fieldsMap := range events {
		// Determine hostname/source IP
		hostname := syslog.DetermineHostname(
			p.cfg.UseEventIP,
			p.cfg.CustomSourceIP,
			fieldsMap,
		)

		// Format as CEF
		cefMessage := p.cefFormatter.Format(fieldsMap)

		// Format as syslog
		syslogMessage := syslog.FormatMessage(hostname, cefMessage)

		// Truncate if necessary
		if len(syslogMessage) > p.cfg.MaxMsgSize {
			p.logger.Debug("truncating oversized message",
				"original_size", len(syslogMessage),
				"max_size", p.cfg.MaxMsgSize)
			syslogMessage = syslogMessage[:p.cfg.MaxMsgSize]
		}

		// Send to syslog with retry on failure
		if err := p.syslogWriter.Write(syslogMessage); err != nil {
			p.logger.Warn("syslog write failed, attempting reconnect", "error", err.Error())

			if reconnectErr := p.syslogWriter.Reconnect(); reconnectErr != nil {
				return forwardedCount, fmt.Errorf("reconnection failed: %w", reconnectErr)
			}

			// Retry write after reconnect
			if err = p.syslogWriter.Write(syslogMessage); err != nil {
				return forwardedCount, fmt.Errorf("write failed after reconnect: %w", err)
			}
		}

		forwardedCount++
	}

	p.logger.Debug("forwarded events batch", "count", forwardedCount)
	return forwardedCount, nil
}

// ProcessWithRecovery wraps ProcessEvents with panic recovery
func (p *Processor) ProcessWithRecovery(ctx context.Context) bool {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("PANIC recovered in event processing", "panic", r)
			p.stats.IncrementFailedAPIRequests()
		}
	}()

	err := p.ProcessEvents(ctx)
	if err != nil {
		p.logger.Error("event processing failed", "error", err.Error())
		p.stats.IncrementFailedAPIRequests()
		return false
	}

	return true
}
