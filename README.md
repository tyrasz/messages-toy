# Messenger

A modern messaging app with built-in content moderation, inspired by MSN Messenger. Features a Go backend with real-time WebSocket messaging and a Flutter mobile app.

## Features

- **Real-time messaging** via WebSocket
- **Message status** (sent → delivered → read)
- **Typing indicators**
- **Online/offline presence**
- **Contact management**
- **Image sharing** with content moderation
- **JWT authentication**

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│   Flutter App   │◄───────►│   Go Backend    │
│   (iOS/Android) │   WS    │   (Fiber)       │
└─────────────────┘         └────────┬────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
              ┌──────────┐    ┌──────────┐    ┌──────────────┐
              │  SQLite  │    │  Media   │    │ Google Cloud │
              │    DB    │    │ Storage  │    │   Vision     │
              └──────────┘    └──────────┘    └──────────────┘
```

## Content Moderation

Images are scanned before delivery using a quarantine pipeline:

1. **Upload** → stored in quarantine
2. **Scan** → Google Cloud Vision SafeSearch API
3. **Decision**:
   - `VERY_UNLIKELY` explicit → **Approved**
   - `POSSIBLE` explicit → **Human Review**
   - `LIKELY`/`VERY_LIKELY` explicit → **Blocked**
4. **Approved** media moved to permanent storage

In development mode, a mock scanner auto-approves all images.

## Getting Started

### Backend

```bash
cd backend

# Install dependencies
go mod tidy

# Run server
go run cmd/server/main.go
```

Server starts at `http://localhost:8080`

### Flutter App

```bash
cd app

# Install dependencies
flutter pub get

# Run on device/emulator
flutter run
```

## API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Create account |
| POST | `/api/auth/login` | Login, get JWT |
| POST | `/api/auth/refresh` | Refresh token |

### Contacts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/contacts` | List contacts |
| POST | `/api/contacts` | Add contact |
| DELETE | `/api/contacts/:id` | Remove contact |

### Messages
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/messages/conversations` | List conversations |
| GET | `/api/messages/:userId` | Get message history |

### Media
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/media/upload` | Upload image |
| GET | `/api/media/:id` | Get approved media |

### WebSocket
| Endpoint | Description |
|----------|-------------|
| `WS /ws?token=<jwt>` | Real-time messaging |

## WebSocket Messages

```json
// Send message
{"type": "message", "to": "user_id", "content": "Hello!"}

// Typing indicator
{"type": "typing", "to": "user_id", "typing": true}

// Read receipt
{"type": "ack", "message_id": "...", "status": "read"}
```

## Project Structure

```
├── backend/
│   ├── cmd/server/          # Entry point
│   └── internal/
│       ├── api/             # HTTP handlers & routes
│       ├── database/        # SQLite setup
│       ├── models/          # Data models
│       ├── services/        # Business logic
│       └── websocket/       # Real-time messaging
│
└── app/
    └── lib/
        ├── models/          # Data models
        ├── providers/       # Riverpod state
        ├── screens/         # UI screens
        └── services/        # API & WebSocket clients
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `JWT_SECRET` | JWT signing key | `your-secret-key...` |
| `USE_MOCK_MODERATION` | Skip real scanning | `true` |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP credentials path | - |

## Tech Stack

**Backend:**
- Go 1.21+
- Fiber (web framework)
- GORM (ORM)
- SQLite
- Google Cloud Vision API

**Mobile:**
- Flutter 3.x
- Riverpod (state management)
- Dio (HTTP client)
- web_socket_channel

## License

MIT
