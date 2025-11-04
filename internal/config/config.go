package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// Config holds all the program configuration
type Config struct {
	// Cato API
	CatoAPIURL    string
	CatoAPIKey    string
	CatoAccountID string

	// Syslog
	SyslogServer   string
	SyslogPort     int
	SyslogProtocol string
	MaxMsgSize     int
	UseEventIP     bool
	CustomSourceIP string

	// CEF
	CEFVendor     string
	CEFProduct    string
	CEFVersion    string
	FieldMappings map[string]string
	OrderedFields []string

	// Processing
	FetchInterval   int
	MaxEvents       int
	MaxPagination   int
	RetryAttempts   int
	RetryDelay      int
	MaxBackoffDelay int
	ConnTimeout     int

	// State
	MarkerFile string

	// Logging
	LogLevel  string
	LogFormat string
	LogOutput string

	// Runtime (not from JSON)
	Verbose    bool
	ConfigPath string
}

// jsonConfig represents the JSON structure
type jsonConfig struct {
	Cato struct {
		APIURL    string `json:"api_url"`
		APIKey    string `json:"api_key"`
		AccountID string `json:"account_id"`
	} `json:"cato"`
	Syslog struct {
		Server             string `json:"server"`
		Port               int    `json:"port"`
		Protocol           string `json:"protocol"`
		MaxMessageSize     int    `json:"max_message_size"`
		UseEventIPAsSource bool   `json:"use_event_ip_as_source"`
		CustomSourceIP     string `json:"custom_source_ip"`
	} `json:"syslog"`
	CEF struct {
		Vendor        string            `json:"vendor"`
		Product       string            `json:"product"`
		Version       string            `json:"version"`
		FieldMappings map[string]string `json:"field_mappings"`
		OrderedFields []string          `json:"ordered_fields"`
	} `json:"cef"`
	Processing struct {
		FetchIntervalSeconds     int `json:"fetch_interval_seconds"`
		MaxEventsPerRequest      int `json:"max_events_per_request"`
		MaxPaginationRequests    int `json:"max_pagination_requests"`
		RetryAttempts            int `json:"retry_attempts"`
		RetryDelaySeconds        int `json:"retry_delay_seconds"`
		MaxBackoffDelaySeconds   int `json:"max_backoff_delay_seconds"`
		ConnectionTimeoutSeconds int `json:"connection_timeout_seconds"`
	} `json:"processing"`
	State struct {
		MarkerFile string `json:"marker_file"`
	} `json:"state"`
	Logging struct {
		Level  string `json:"level"`
		Format string `json:"format"`
		Output string `json:"output"`
	} `json:"logging"`
}

// Load reads configuration from JSON file
func Load() (*Config, error) {
	// Parse minimal CLI flags
	configPath := flag.String("config", "", "Path to config.json file")
	verbose := flag.Bool("verbose", false, "Enable verbose debug output")
	flag.Parse()

	// Find config file
	path, err := findConfigFile(*configPath)
	if err != nil {
		return nil, err
	}

	// Load from JSON
	cfg, err := loadFromJSON(path)
	if err != nil {
		return nil, err
	}

	// Set runtime flags
	cfg.Verbose = *verbose
	cfg.ConfigPath = path

	// Override log level to debug if verbose flag is set
	if cfg.Verbose {
		cfg.LogLevel = "debug"
	}

	return cfg, nil
}

// findConfigFile searches for config file in order of precedence
func findConfigFile(explicitPath string) (string, error) {
	// 1. Explicit path from --config flag (highest precedence)
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err == nil {
			return explicitPath, nil
		}
		return "", fmt.Errorf("specified config file not found: %s", explicitPath)
	}

	// 2. Current directory (./config.json)
	localPath := "./config.json"
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// 3. System location (/etc/cato-logger/config.json)
	systemPath := "/etc/cato-logger/config.json"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath, nil
	}

	return "", fmt.Errorf("no config file found (searched: %s, %s)", localPath, systemPath)
}

// loadFromJSON reads and parses the JSON config file
func loadFromJSON(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var jc jsonConfig
	if err := json.Unmarshal(data, &jc); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Flatten nested structure into Config struct
	cfg := &Config{
		// Cato
		CatoAPIURL:    jc.Cato.APIURL,
		CatoAPIKey:    jc.Cato.APIKey,
		CatoAccountID: jc.Cato.AccountID,

		// Syslog
		SyslogServer:   jc.Syslog.Server,
		SyslogPort:     jc.Syslog.Port,
		SyslogProtocol: jc.Syslog.Protocol,
		MaxMsgSize:     jc.Syslog.MaxMessageSize,
		UseEventIP:     jc.Syslog.UseEventIPAsSource,
		CustomSourceIP: jc.Syslog.CustomSourceIP,

		// CEF
		CEFVendor:     jc.CEF.Vendor,
		CEFProduct:    jc.CEF.Product,
		CEFVersion:    jc.CEF.Version,
		FieldMappings: jc.CEF.FieldMappings,
		OrderedFields: jc.CEF.OrderedFields,

		// Processing
		FetchInterval:   jc.Processing.FetchIntervalSeconds,
		MaxEvents:       jc.Processing.MaxEventsPerRequest,
		MaxPagination:   jc.Processing.MaxPaginationRequests,
		RetryAttempts:   jc.Processing.RetryAttempts,
		RetryDelay:      jc.Processing.RetryDelaySeconds,
		MaxBackoffDelay: jc.Processing.MaxBackoffDelaySeconds,
		ConnTimeout:     jc.Processing.ConnectionTimeoutSeconds,

		// State
		MarkerFile: jc.State.MarkerFile,

		// Logging
		LogLevel:  jc.Logging.Level,
		LogFormat: jc.Logging.Format,
		LogOutput: jc.Logging.Output,
	}

	// Enforce max events limit
	if cfg.MaxEvents > 5000 {
		cfg.MaxEvents = 5000
	}

	return cfg, nil
}

// SyslogAddress returns the formatted syslog server address
func (c *Config) SyslogAddress() string {
	return fmt.Sprintf("%s:%d", c.SyslogServer, c.SyslogPort)
}
