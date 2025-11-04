package config

import (
	"fmt"
)

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	missing := []string{}

	// Required Cato API settings
	if c.CatoAPIKey == "" {
		missing = append(missing, "cato.api_key")
	}
	if c.CatoAccountID == "" {
		missing = append(missing, "cato.account_id")
	}

	// Required Syslog settings
	if c.SyslogServer == "" {
		missing = append(missing, "syslog.server")
	}
	if c.SyslogPort <= 0 {
		missing = append(missing, "syslog.port")
	}
	if c.SyslogProtocol == "" {
		missing = append(missing, "syslog.protocol")
	}

	// Required CEF settings
	if len(c.FieldMappings) == 0 {
		missing = append(missing, "cef.field_mappings")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration fields: %v", missing)
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", c.LogLevel)
	}

	// Validate log format
	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validLogFormats[c.LogFormat] {
		return fmt.Errorf("invalid log format '%s', must be one of: json, text", c.LogFormat)
	}

	// Validate syslog protocol
	validProtocols := map[string]bool{
		"tcp": true,
		"udp": true,
	}
	if !validProtocols[c.SyslogProtocol] {
		return fmt.Errorf("invalid syslog protocol '%s', must be tcp or udp", c.SyslogProtocol)
	}

	// Validate processing settings
	if c.FetchInterval < 10 {
		return fmt.Errorf("fetch_interval_seconds must be at least 10 seconds, got %d", c.FetchInterval)
	}

	if c.MaxEvents < 1 || c.MaxEvents > 5000 {
		return fmt.Errorf("max_events_per_request must be between 1 and 5000, got %d", c.MaxEvents)
	}

	if c.MaxPagination < 1 {
		return fmt.Errorf("max_pagination_requests must be at least 1, got %d", c.MaxPagination)
	}

	if c.RetryAttempts < 0 {
		return fmt.Errorf("retry_attempts cannot be negative, got %d", c.RetryAttempts)
	}

	if c.ConnTimeout < 1 {
		return fmt.Errorf("connection_timeout_seconds must be at least 1, got %d", c.ConnTimeout)
	}

	return nil
}
