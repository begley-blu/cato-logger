package syslog

import (
	"fmt"
	"net"
	"time"

	"cato-logger/internal/logging"
)

// Writer manages a resilient connection to a syslog server
type Writer struct {
	protocol       string
	address        string
	conn           net.Conn
	reconnectCount int
	lastReconnect  time.Time
	maxReconnects  int
	reconnectDelay time.Duration
	connTimeout    time.Duration
	logger         *logging.Logger
}

// NewWriter creates a new syslog writer
func NewWriter(protocol, address string, connTimeout time.Duration, logger *logging.Logger) (*Writer, error) {
	conn, err := net.DialTimeout(protocol, address, connTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog server: %w", err)
	}

	logger.Info("connected to syslog server", "protocol", protocol, "address", address)

	return &Writer{
		protocol:       protocol,
		address:        address,
		conn:           conn,
		maxReconnects:  10,
		reconnectDelay: 5 * time.Second,
		connTimeout:    connTimeout,
		logger:         logger,
	}, nil
}

// Write sends a message to the syslog server
func (w *Writer) Write(message string) error {
	if w.conn == nil {
		return fmt.Errorf("no connection available")
	}

	_, err := fmt.Fprintln(w.conn, message)
	if err != nil {
		w.logger.Debug("syslog write failed", "error", err.Error())
	}
	return err
}

// Close closes the syslog connection
func (w *Writer) Close() error {
	if w.conn != nil {
		w.logger.Info("closing syslog connection")
		return w.conn.Close()
	}
	return nil
}

// Reconnect attempts to reconnect to the syslog server
func (w *Writer) Reconnect() error {
	// Implement connection rate limiting
	if time.Since(w.lastReconnect) < w.reconnectDelay {
		return fmt.Errorf("reconnection rate limited")
	}

	if w.reconnectCount >= w.maxReconnects {
		w.logger.Error("max reconnection attempts exceeded",
			"count", w.reconnectCount,
			"max", w.maxReconnects)
		return fmt.Errorf("max reconnection attempts exceeded")
	}

	if w.conn != nil {
		w.conn.Close()
	}

	w.logger.Info("attempting syslog reconnection",
		"attempt", w.reconnectCount+1,
		"address", w.address)

	conn, err := net.DialTimeout(w.protocol, w.address, w.connTimeout)
	if err != nil {
		w.reconnectCount++
		w.lastReconnect = time.Time{}
		w.logger.Warn("syslog reconnection failed",
			"attempt", w.reconnectCount,
			"error", err.Error())
		return fmt.Errorf("failed to reconnect to syslog server: %w", err)
	}

	w.conn = conn
	w.reconnectCount = 0 // Reset on successful reconnection
	w.lastReconnect = time.Now()
	w.logger.Info("syslog reconnection successful")
	return nil
}

// ReconnectCount returns the current reconnection attempt count
func (w *Writer) ReconnectCount() int {
	return w.reconnectCount
}
