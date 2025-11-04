package marker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cato-logger/internal/logging"
)

// Manager handles reading and writing event markers
type Manager struct {
	filePath string
	marker   string
	logger   *logging.Logger
}

// New creates a new marker manager
func New(filePath string, logger *logging.Logger) (*Manager, error) {
	m := &Manager{
		filePath: filePath,
		logger:   logger,
	}

	// Load existing marker if it exists
	if err := m.Load(); err != nil {
		// If file doesn't exist, that's okay - we'll start fresh
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load marker: %w", err)
		}
		logger.Info("no existing marker file found, starting fresh", "path", filePath)
	} else {
		logger.Info("loaded marker from file", "path", filePath, "has_marker", m.marker != "")
	}

	return m, nil
}

// Load reads the marker from the file
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}
	m.marker = strings.TrimSpace(string(data))
	return nil
}

// Save writes the marker to the file
func (m *Manager) Save(marker string) error {
	if marker == "" {
		return nil // Don't save empty markers
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for marker file: %w", err)
	}

	if err := os.WriteFile(m.filePath, []byte(marker), 0644); err != nil {
		return fmt.Errorf("failed to write marker file: %w", err)
	}

	m.marker = marker
	m.logger.Debug("saved marker to file", "path", m.filePath)
	return nil
}

// Get returns the current marker
func (m *Manager) Get() string {
	return m.marker
}

// Update updates the marker and saves it
func (m *Manager) Update(marker string) error {
	if marker == "" || marker == m.marker {
		return nil
	}
	return m.Save(marker)
}
