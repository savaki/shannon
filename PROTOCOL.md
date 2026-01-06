# Shannon Communication Protocol

This document describes the HTTP API protocol used by Shannon for remote control of Claude Code sessions.

## Overview

Shannon exposes a RESTful HTTP API over Tailscale for secure remote access. All endpoints (except health check) require authentication via a pre-shared key (PSK).

## Authentication

### Pre-Shared Key (PSK)

All authenticated endpoints require a Bearer token in the `Authorization` header:

```
Authorization: Bearer <psk>
```

The PSK is generated on server startup and displayed via QR code for mobile app pairing.

### QR Code Pairing

The QR code contains a JSON payload with connection details:

```json
{
  "host": "100.64.x.x",
  "port": 8080,
  "psk": "base64-encoded-key"
}
```

## Base URL

```
http://<tailscale-ip>:<port>
```

Default port: `8080`

---

## Endpoints

### Health Check

Check server availability. Does not require authentication.

```
GET /health
```

**Response** `200 OK`
```json
{
  "status": "ok"
}
```

---

### Sessions

#### Create Session

Start a new Claude Code session.

```
POST /sessions
```

**Request Body**
```json
{
  "repo_url": "https://github.com/user/repo.git",
  "branch": "main",
  "prompt": "Initial prompt for Claude"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repo_url` | string | No | Git repository URL to clone |
| `branch` | string | No | Branch to checkout (default: default branch) |
| `prompt` | string | No | Initial prompt to send to Claude |

**Response** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "work_dir": "/data/sessions/550e8400-e29b-41d4-a716-446655440000",
  "repo_url": "https://github.com/user/repo.git",
  "status": "running",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### List Sessions

Get all sessions.

```
GET /sessions
```

**Response** `200 OK`
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "work_dir": "/data/sessions/550e8400-...",
    "repo_url": "https://github.com/user/repo.git",
    "status": "running",
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

#### Get Session

Get details of a specific session.

```
GET /sessions/{id}
```

**Response** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "work_dir": "/data/sessions/550e8400-...",
  "repo_url": "https://github.com/user/repo.git",
  "status": "waiting_input",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Response** `404 Not Found`
```json
{
  "error": "session not found"
}
```

#### Delete Session

Stop and remove a session.

```
DELETE /sessions/{id}
```

**Response** `204 No Content`

#### Send Input

Send user input to a running session.

```
POST /sessions/{id}/input
```

**Request Body**
```json
{
  "input": "User message to Claude"
}
```

**Response** `200 OK`
```json
{
  "status": "sent"
}
```

**Response** `400 Bad Request`
```json
{
  "error": "input is required"
}
```

#### Get History

Get conversation history for a session.

```
GET /sessions/{id}/history
```

**Response** `200 OK`
```json
[
  {
    "id": "msg-001",
    "session_id": "550e8400-...",
    "role": "user",
    "content": "Hello Claude",
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "id": "msg-002",
    "session_id": "550e8400-...",
    "role": "assistant",
    "content": "Hello! How can I help you?",
    "created_at": "2024-01-15T10:30:05Z"
  }
]
```

#### Stream Events (SSE)

Subscribe to real-time session events via Server-Sent Events.

```
GET /sessions/{id}/stream
```

**Headers**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**Event Format**
```
event: <event_type>
data: <json_payload>

```

**Event Types**

| Event | Description |
|-------|-------------|
| `connected` | Initial connection established |
| `output` | Regular output from Claude |
| `question` | Claude is asking for user input |
| `tool` | Tool usage notification |
| `status` | Session status change |
| `error` | Error occurred |
| `closed` | Session has ended |

**Example Stream**
```
event: connected
data: {"session_id":"550e8400-..."}

event: output
data: {"session_id":"550e8400-...","type":"output","content":"Hello!","timestamp":"2024-01-15T10:30:00Z"}

event: question
data: {"session_id":"550e8400-...","type":"question","content":"Would you like to continue?","timestamp":"2024-01-15T10:30:05Z"}

event: closed
data: {"reason":"session ended"}
```

---

### Devices (Push Notifications)

> **Note:** Push notification endpoints are not yet implemented.

#### Register Device

Register a device for push notifications.

```
POST /devices
```

**Request Body**
```json
{
  "token": "fcm-device-token",
  "platform": "ios"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `token` | string | Yes | FCM device token |
| `platform` | string | Yes | `ios` or `android` |

**Response** `501 Not Implemented`

#### Unregister Device

Remove a device from push notifications.

```
DELETE /devices/{id}
```

**Response** `501 Not Implemented`

---

## Session Status Values

| Status | Description |
|--------|-------------|
| `pending` | Session created, not yet started |
| `running` | Claude is actively processing |
| `waiting_input` | Claude is waiting for user response |
| `stopped` | Session has been stopped |
| `error` | An error occurred |

## Message Roles

| Role | Description |
|------|-------------|
| `user` | Message from the user |
| `assistant` | Response from Claude |
| `system` | System message |

---

## Error Responses

All errors follow this format:

```json
{
  "error": "error message"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| `200` | Success |
| `201` | Created |
| `204` | No Content (success, no body) |
| `400` | Bad Request (invalid input) |
| `401` | Unauthorized (missing/invalid PSK) |
| `404` | Not Found |
| `500` | Internal Server Error |
| `501` | Not Implemented |

---

## Example Usage

### curl Examples

**Health Check**
```bash
curl http://localhost:8080/health
```

**Create Session**
```bash
curl -X POST http://localhost:8080/sessions \
  -H "Authorization: Bearer $PSK" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello Claude!"}'
```

**Send Input**
```bash
curl -X POST http://localhost:8080/sessions/$SESSION_ID/input \
  -H "Authorization: Bearer $PSK" \
  -H "Content-Type: application/json" \
  -d '{"input": "What files are in this directory?"}'
```

**Stream Events**
```bash
curl -N http://localhost:8080/sessions/$SESSION_ID/stream \
  -H "Authorization: Bearer $PSK"
```

**Delete Session**
```bash
curl -X DELETE http://localhost:8080/sessions/$SESSION_ID \
  -H "Authorization: Bearer $PSK"
```

---

## WebSocket Alternative

Shannon uses Server-Sent Events (SSE) for streaming rather than WebSockets because:

1. SSE works over standard HTTP, simplifying proxy and firewall traversal
2. Automatic reconnection is built into the browser SSE API
3. Simpler server implementation
4. One-way streaming (server to client) matches the use case

For bidirectional communication, use the `/sessions/{id}/input` endpoint to send messages while maintaining an SSE connection for receiving responses.
