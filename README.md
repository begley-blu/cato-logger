# Cato Networks CEF Log Forwarder

A production-ready Go application that polls the Cato Networks API for security events and forwards them as CEF (Common Event Format) messages to a syslog server.

**Version:** 3.2
**Language:** Go (stdlib only, zero external dependencies)
**Runtime:** Systemd service

## Features

- ✅ **Zero External Dependencies** - Uses only Go standard library
- ✅ **Production Ready** - Systemd integration, graceful shutdown, panic recovery
- ✅ **Pre-Flight Checks** - Tests API, syslog, and file access before startup
- ✅ **Resilient** - Auto-retry, exponential backoff, connection management
- ✅ **Resumable** - Marker-based event tracking prevents duplicates
- ✅ **Observable** - Structured JSON/text logging with detailed metrics
- ✅ **Simple Configuration** - Single JSON file, no environment variables
- ✅ **Secure** - Dedicated user, no port exposure, secure file permissions

## Quick Start

### Prerequisites

- Ubuntu Linux (other distributions may work but are not officially supported)
- Systemd init system
- Cato Networks API key with `eventsFeed` permissions
- Syslog server (TCP or UDP)

### Quick Install (Recommended)

The easiest way to install Cato Logger is with the automated installation script:

```bash
# Download and run installer
curl -fsSL https://raw.githubusercontent.com/begley-blu/cato-logger/main/install.sh | sudo bash
```

Or if you've already cloned the repository:

```bash
# From project directory
sudo ./install.sh
```

The installer will:
- Prompt for your Cato API key, Account ID, and Syslog server (optional)
- Create the `cato-logger` service user
- Install the pre-compiled binary to `/usr/local/bin/`
- Set up configuration in `/etc/cato-logger/`
- Install and enable the systemd service
- Validate all connections (API, syslog, file access)

After installation, start the service:

```bash
sudo systemctl start cato-logger
sudo systemctl status cato-logger
sudo journalctl -u cato-logger -f
```

### Manual Installation

If you prefer to build from source:

```bash
# Clone repository
git clone https://github.com/begley-blu/cato-logger.git
cd cato-logger

# Build
make build

# Install (requires sudo)
sudo make install

# Configure
sudo cp /etc/cato-logger/config.json.example /etc/cato-logger/config.json
sudo nano /etc/cato-logger/config.json
# Set: cato.api_key, cato.account_id, syslog.server, etc.

# Set secure permissions
sudo chmod 600 /etc/cato-logger/config.json

# Install and start service
sudo make install-service
sudo systemctl start cato-logger

# Check status
sudo systemctl status cato-logger
sudo journalctl -u cato-logger -f
```

## Project Structure

```
cato-logger/
├── cmd/
│   └── cato-logger/     # Main application entry point
│       └── main.go              # 186 lines (down from 980+)
│
├── internal/                    # Private application packages
│   ├── api/                    # Cato API client
│   │   ├── client.go           # HTTP/GraphQL client with structured logging
│   │   ├── retry.go            # Retry logic with exponential backoff
│   │   └── types.go            # API data structures
│   │
│   ├── cef/                    # CEF formatting
│   │   ├── formatter.go        # CEF message builder
│   │   └── types.go            # Minimal types
│   │
│   ├── config/                 # Configuration management
│   │   ├── config.go           # JSON-based config loading
│   │   └── validation.go       # Config validation
│   │
│   ├── logging/                # Structured logging
│   │   └── logger.go           # JSON/text logger (stdlib only)
│   │
│   ├── marker/                 # Event position tracking
│   │   └── marker.go           # Marker file manager
│   │
│   ├── processor/              # Event processing pipeline
│   │   ├── processor.go        # Main processing logic
│   │   └── stats.go            # Service statistics
│   │
│   └── syslog/                 # Syslog integration
│       └── writer.go           # TCP/UDP connection manager
│
├── configs/                    # Configuration files
│   └── config.json             # Single JSON configuration
│
├── deployments/                # Deployment resources
│   └── systemd/               # Systemd unit files
│       └── cato-logger.service
│
├── go.mod                      # Go module definition
├── README.md                   # This file
```

## Configuration

Configuration is managed through a single JSON file: `/etc/cato-logger/config.json`

### Configuration File Structure Example
** Always use the latest config from the configs/ directory.

```json
{
  "cato": {
    "api_url": "https://api.catonetworks.com/api/v1/graphql2",
    "api_key": "your_api_key_here",
    "account_id": "your_account_id"
  },
  "syslog": {
    "server": "syslog.example.com",
    "port": 514,
    "protocol": "tcp",
    "max_message_size": 8192,
    "use_event_ip_as_source": false,
    "custom_source_ip": ""
  },
  "cef": {
    "vendor": "Cato Networks",
    "product": "SASE Platform",
    "version": "1.0",
    "field_mappings": {
      "event_type": "cat",
      "event_sub_type": "act",
      "src_ip": "src",
      "dst_ip": "dst",
      "src_port": "spt",
      "dst_port": "dpt",
      "protocol": "proto",
      "action": "act",
      "app": "app",
      "timestamp": "start"
    },
    "ordered_fields": [
      "start", "end", "src", "dst", "spt", "dpt",
      "proto", "act", "app", "cat"
    ]
  },
  "processing": {
    "fetch_interval_seconds": 60,
    "max_events_per_request": 1000,
    "max_pagination_requests": 10,
    "retry_attempts": 3,
    "retry_delay_seconds": 2,
    "connection_timeout_seconds": 30,
    "max_backoff_delay_seconds": 300
  },
  "state": {
    "marker_file": "/etc/cato-logger/last_marker.txt"
  },
  "logging": {
    "level": "info",
    "format": "json",
    "output": "stdout"
  }
}
```

### Configuration Sections

| Section | Description |
|---------|-------------|
| `cato` | Cato Networks API credentials and endpoint |
| `syslog` | Syslog server connection settings |
| `cef` | CEF formatting rules and field mappings |
| `processing` | Event fetching and retry behavior |
| `state` | Marker file location for resumable processing |
| `logging` | Application logging configuration |

### Configuration File Search Order

The application searches for configuration in this order:

1. `--config` flag (if specified)
2. `./config.json` (current directory - for development)
3. `/etc/cato-logger/config.json` (system default - for production)

### Logging Configuration

**Log Levels:** `debug`, `info`, `warn`, `error`
**Log Formats:** `json` (machine-readable), `text` (human-readable)
**Log Output:** `stdout`, `stderr`, or file path

Example structured log output (JSON format):
```json
{"time":"2025-11-03T15:20:45Z","level":"info","msg":"starting Cato Networks CEF Forwarder","version":"3.2","pid":12345}
{"time":"2025-11-03T15:20:46Z","level":"info","msg":"processing cycle complete","duration_ms":1234,"events_processed":150}
```

## Manual Usage

### CLI Flags

The application accepts only two flags:

```bash
# Specify custom config file
cato-logger --config=/path/to/config.json

# Enable debug logging (overrides config.json log level)
cato-logger --verbose
```

## Monitoring

### Logging using Journald

The application uses structured logging with detailed metrics:

```bash
# Follow logs
sudo journalctl -fu cato-logger

# Recent logs
sudo journalctl -u cato-logger -n 100

# Errors only
sudo journalctl -u cato-logger -p err

# View statistics
sudo journalctl -u cato-logger | grep "processing cycle complete"
```

### Log Output Examples

**JSON format** (default, machine-readable):
```json
{"time":"2025-11-03T15:20:45Z","level":"info","msg":"starting Cato Networks CEF Forwarder","version":"3.2","pid":12345}
{"time":"2025-11-03T15:20:46Z","level":"info","msg":"running pre-flight checks"}
{"time":"2025-11-03T15:20:47Z","level":"info","msg":"pre-flight check passed","check":"Marker File Access","message":"marker file is readable and writable: /etc/cato-logger/last_marker.txt"}
{"time":"2025-11-03T15:20:47Z","level":"info","msg":"pre-flight check passed","check":"Syslog Connectivity","message":"syslog server is reachable at tcp://syslog.example.com:514"}
{"time":"2025-11-03T15:20:48Z","level":"info","msg":"pre-flight check passed","check":"Cato API Connectivity","message":"Cato API is accessible and authenticated (account: 12345)"}
{"time":"2025-11-03T15:20:48Z","level":"info","msg":"pre-flight checks complete","passed":3,"failed":0,"total":3}
{"time":"2025-11-03T15:20:48Z","level":"info","msg":"all pre-flight checks passed"}
{"time":"2025-11-03T15:20:49Z","level":"info","msg":"all components initialized successfully"}
{"time":"2025-11-03T15:25:49Z","level":"info","msg":"processing cycle complete","duration_ms":1234,"events_processed":150,"total_events":1500,"events_per_second":"121.54"}
```

**Text format** (human-readable):
```
2025-11-03T15:20:45Z INFO starting Cato Networks CEF Forwarder version=3.2 pid=12345
2025-11-03T15:20:46Z INFO running pre-flight checks
2025-11-03T15:20:47Z INFO pre-flight check passed check="Marker File Access" message="marker file is readable and writable: /etc/cato-logger/last_marker.txt"
2025-11-03T15:20:47Z INFO pre-flight check passed check="Syslog Connectivity" message="syslog server is reachable at tcp://syslog.example.com:514"
2025-11-03T15:20:48Z INFO pre-flight check passed check="Cato API Connectivity" message="Cato API is accessible and authenticated (account: 12345)"
2025-11-03T15:20:48Z INFO pre-flight checks complete passed=3 failed=0 total=3
2025-11-03T15:20:48Z INFO all pre-flight checks passed
2025-11-03T15:20:49Z INFO all components initialized successfully
2025-11-03T15:25:49Z INFO processing cycle complete duration_ms=1234 events_processed=150 total_events=1500 events_per_second=121.54
```

### Key Metrics in Logs

- `events_processed` - Events forwarded in this cycle
- `total_events` - Total events forwarded since startup
- `total_api_requests` - API requests made
- `failed_api_requests` - Failed API requests
- `duration_ms` - Processing cycle duration
- `events_per_second` - Throughput rate

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -xeu cato-logger

# Verify config
sudo cat /etc/cato-logger/config.json

# Test configuration manually
sudo /usr/local/bin/cato-logger --config=/etc/cato-logger/config.json
```

Common errors:
- `missing required configuration fields` - Check all required fields are set
- `invalid log level` - Must be: debug, info, warn, error
- `invalid syslog protocol` - Must be: tcp or udp
- `pre-flight checks failed` - See detailed error messages below:
  - **Marker File Access failed**: Check directory permissions and disk space
  - **Syslog Connectivity failed**: Verify syslog server address, port, and firewall rules
  - **Cato API Connectivity failed**: Check API key, account ID, and network connectivity

### No Events Forwarding

```bash
# Enable debug logging
sudo nano /etc/cato-logger/config.json
# Set: "logging": { "level": "debug", "format": "text" }
sudo systemctl restart cato-logger

# Watch detailed logs
sudo journalctl -fu cato-logger
```

### Permission Errors

```bash
# Fix config file permissions
sudo chmod 600 /etc/cato-logger/config.json
sudo chown cato-logger:cato-logger /etc/cato-logger/config.json

# Fix marker file permissions
sudo chown cato-logger:cato-logger /etc/cato-logger/last_marker.txt
```

### Connection Issues

- **API Connection**: Verify `cato.api_key` and `cato.account_id` in config.json
- **Syslog Connection**: Test with `nc -v $SYSLOG_SERVER $SYSLOG_PORT`
- **Firewall**: Ensure outbound HTTPS (443) and syslog port access

### Reset Event Position

If you need to start processing from the beginning:
```bash
sudo systemctl stop cato-logger
sudo rm /etc/cato-logger/last_marker.txt
sudo systemctl start cato-logger
```

## Development

### Requirements

- Go 1.18+
- No external dependencies (stdlib only)

### Code Organization

- **`cmd/`** - Application entry points
- **`internal/`** - Private packages (cannot be imported externally)
- **`configs/`** - Configuration templates
- **`deployments/`** - Deployment resources
- **`docs/`** - Additional documentation

### NOTICE

This code is provided as-is to help security administrators access Cato Networks through a syslog server.