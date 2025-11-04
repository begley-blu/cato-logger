package syslog

import (
	"fmt"
	"os"
	"time"
)

// FormatMessage creates a syslog-formatted message
func FormatMessage(hostname, message string) string {
	priority := "134" // local0.info
	timestamp := time.Now().Format("Jan  2 15:04:05")
	return fmt.Sprintf("<%s>%s %s %s", priority, timestamp, hostname, message)
}

// ExtractSourceIP attempts to extract the source IP from event data
func ExtractSourceIP(fieldsMap map[string]string) string {
	candidates := []string{"client_ip", "src_ip", "source_ip", "host_ip", "user_ip"}

	for _, field := range candidates {
		if ip, exists := fieldsMap[field]; exists && ip != "" {
			return ip
		}
	}

	return ""
}

// DetermineHostname determines the hostname to use for syslog messages
func DetermineHostname(useEventIP bool, customSourceIP string, fieldsMap map[string]string) string {
	if useEventIP {
		sourceIP := ExtractSourceIP(fieldsMap)
		if sourceIP != "" {
			return sourceIP
		}
		return "unknown-host"
	}

	if customSourceIP != "" {
		return customSourceIP
	}

	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return "cato-forwarder"
}
