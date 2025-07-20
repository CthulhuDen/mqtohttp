# MQTT to HTTP Bridge

A lightweight service that subscribes to MQTT topics and forwards received messages to an HTTP endpoint. 
Perfect for integrating MQTT-based IoT devices or services with HTTP APIs.

## Features

- Subscribe to multiple MQTT topics
- Forward MQTT messages to HTTP endpoint
- Support for MQTT authentication
- Persistent MQTT session support
- Docker support with multi-arch images (amd64, arm64)

## Installation

### Using Docker (Recommended)

Pull the latest version from GitHub Container Registry:

```bash
docker pull ghcr.io/cthulhuden/mqtohttp:0.1
```

### Building from source

```bash
go build -o mqtohttp
```

## Configuration

The service can be configured using environment variables:

| Environment Variable | Required | Default                                            | Description |
|---------------------|----------|----------------------------------------------------|-------------|
| MQTT_ENDPOINT | Yes | -                                                  | MQTT server endpoint (e.g., `wss://mqtt.example.com`) |
| MQTT_USER | No | -                                                  | Username for MQTT authentication |
| MQTT_PASSWORD | No | -                                                  | Password for MQTT authentication |
| MQTT_SESSION_FILE | No | `session-id.txt` / `/data/session-id.txt` (docker) | File to store MQTT session ID |
| MQTT_KEEPALIVE | No | 20                                                 | Keepalive period in seconds |
| MQTT_SESSION_EXPIRY | No | 604800                                             | Session expiry in seconds (default 7 days) |
| MQTT_TOPICS | Yes | -                                                  | MQTT topics to subscribe to (comma-separated) |
| HTTP_ENDPOINT | Yes | -                                                  | HTTP endpoint to forward messages to |

## Usage Examples

### Basic Example with Environment Variables

```bash
export MQTT_ENDPOINT=wss://mqtt.example.com
export MQTT_TOPICS=sensors/#,home/#
export HTTP_ENDPOINT=http://localhost:3000/api/webhook
./mqtohttp
```

### Running with Docker

```bash
docker run -d \
  -e MQTT_ENDPOINT=wss://mqtt.example.com \
  -e MQTT_TOPICS=sensors/# \
  -e HTTP_ENDPOINT=http://api.example.com/webhook \
  -v mqtt_data:/data \
  ghcr.io/yourusername/mqtohttp:0.1
```

### Docker Compose configuration

```yaml
services:
  mqtohttp:
    image: ghcr.io/yourusername/mqtohttp:0.1
    environment:
      - MQTT_ENDPOINT=wss://mqtt.example.com
      - MQTT_USER=your_username
      - MQTT_PASSWORD=your_password
      - MQTT_TOPICS=topic1/#,topic2/#
      - HTTP_ENDPOINT=https://api.example.com/webhook
      - MQTT_SESSION_EXPIRY=3600
    volumes:
      - mqtt_data:/data
    restart: unless-stopped

volumes:
  mqtt_data:
```

## Message Flow

1. Service connects to the MQTT broker and subscribes to configured topics
2. When a message is received on a subscribed topic, it's forwarded to the HTTP endpoint
3. The message is sent as a POST request with Content-Type: application/json
4. The service maintains a persistent session with the MQTT broker to handle disconnections

## Error Handling

- The service will automatically reconnect to the MQTT broker on connection loss
- Failed HTTP requests will trigger a service restart
- Session persistence ensures no messages are lost during temporary disconnections

## Building Docker Image Locally

```bash
docker build -t mqtohttp .
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
```
