# Messenger

A modern messaging app with built-in content moderation, inspired by MSN Messenger and WhatsApp. Features a Go backend with real-time WebSocket messaging and a Flutter mobile app.

## Features

### Core Messaging
- **Real-time messaging** via WebSocket
- **Message status** (sent, delivered, read)
- **Typing indicators**
- **Online/offline presence**
- **Message reactions** with emoji
- **Message forwarding**
- **Message replies** with preview
- **Message editing and deletion**
- **Disappearing messages** (auto-delete after set time)
- **Scheduled messages**

### Media & Content
- **Image sharing** with content moderation
- **Video sharing** with thumbnail generation
- **Document sharing** (PDF, etc.)
- **Voice messages** with waveform visualization
- **Location sharing**
- **Link previews**

### Groups & Social
- **Group chats** with admin roles
- **Broadcast lists**
- **User blocking**
- **Contact management**
- **Stories/Status updates**
- **Polls**

### Organization
- **Starred messages**
- **Pinned messages**
- **Conversation archiving**
- **Message search**
- **Chat export**
- **Custom themes**

### Notifications
- **Push notifications** (Firebase, APNs, Web Push)
- **Provider-agnostic architecture** - no vendor lock-in

### Offline Support
- **Offline message storage**
- **Automatic sync** when back online
- **Network status monitoring**

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
                                     │
                              ┌──────┴──────┐
                              ▼             ▼
                        ┌──────────┐ ┌──────────┐
                        │   FCM    │ │   APNs   │
                        └──────────┘ └──────────┘
```

## Content Moderation

Images and media are scanned before delivery using a quarantine pipeline:

1. **Upload** → stored in quarantine
2. **Scan** → Google Cloud Vision SafeSearch API
3. **Decision**:
   - `VERY_UNLIKELY` explicit → **Approved**
   - `POSSIBLE` explicit → **Human Review**
   - `LIKELY`/`VERY_LIKELY` explicit → **Blocked**
4. **Approved** media moved to permanent storage

In development mode, a mock scanner auto-approves all images.

## Getting Started

### Prerequisites

- Go 1.21+
- Flutter 3.x
- SQLite (embedded)
- ffmpeg (for video/audio processing, optional)

### Backend Setup

```bash
cd backend

# Install dependencies
go mod tidy

# Run server (development mode)
go run cmd/server/main.go

# Or build and run
go build -o messenger cmd/server/main.go
./messenger
```

Server starts at `http://localhost:8080`

### Flutter App Setup

```bash
cd app

# Install dependencies
flutter pub get

# Run on device/emulator
flutter run
```

### Running Tests

```bash
# Backend tests
cd backend
go test ./... -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## API Endpoints

### Health & Monitoring
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Detailed health status |
| GET | `/healthz` | Kubernetes liveness probe |
| GET | `/readyz` | Kubernetes readiness probe |
| GET | `/metrics` | JSON metrics |
| GET | `/metrics/prometheus` | Prometheus format metrics |

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Create account |
| POST | `/api/auth/login` | Login, get JWT |
| POST | `/api/auth/refresh` | Refresh token |

### Contacts & Users
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/contacts` | List contacts |
| POST | `/api/contacts` | Add contact |
| DELETE | `/api/contacts/:id` | Remove contact |
| GET | `/api/users/search` | Search users |

### Blocking
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/blocks` | List blocked users |
| POST | `/api/blocks/:userId` | Block user |
| DELETE | `/api/blocks/:userId` | Unblock user |
| GET | `/api/blocks/:userId` | Check if blocked |

### Messages
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/messages/conversations` | List conversations |
| GET | `/api/messages/:userId` | Get message history |
| GET | `/api/messages/search` | Search messages |
| GET | `/api/messages/export` | Export chat |
| POST | `/api/messages/location` | Send location |
| POST | `/api/messages/:id/forward` | Forward message |
| GET | `/api/messages/:id/reactions` | Get reactions |
| POST | `/api/messages/:id/reactions` | Add reaction |
| DELETE | `/api/messages/:id/reactions` | Remove reaction |

### Scheduled Messages
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/messages/scheduled` | List scheduled |
| POST | `/api/messages/scheduled` | Schedule message |
| DELETE | `/api/messages/scheduled/:id` | Cancel scheduled |

### Groups
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/groups` | Create group |
| GET | `/api/groups` | List groups |
| GET | `/api/groups/:id` | Get group |
| POST | `/api/groups/:id/members` | Add member |
| DELETE | `/api/groups/:id/members/:userId` | Remove member |
| POST | `/api/groups/:id/leave` | Leave group |
| GET | `/api/groups/:id/messages` | Get group messages |

### Media
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/media/upload` | Upload media |
| GET | `/api/media/:id` | Get media |
| GET | `/api/media/:id/thumbnail` | Get thumbnail |

### Push Notifications
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/notifications/register` | Register device |
| POST | `/api/notifications/unregister` | Unregister token |
| DELETE | `/api/notifications/all` | Remove all tokens |
| GET | `/api/notifications/tokens` | List tokens |
| POST | `/api/notifications/test` | Send test push |

### Starred Messages
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/starred` | List starred |
| POST | `/api/starred/:messageId` | Star message |
| DELETE | `/api/starred/:messageId` | Unstar message |

### Pinned Messages
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/pinned` | Get pinned |
| POST | `/api/pinned` | Pin message |
| DELETE | `/api/pinned` | Unpin message |

### Stories
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/stories` | Create story |
| GET | `/api/stories` | List stories |
| GET | `/api/stories/mine` | My stories |
| POST | `/api/stories/:id/view` | View story |
| GET | `/api/stories/:id/views` | Get viewers |
| DELETE | `/api/stories/:id` | Delete story |

### Polls
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/polls` | Create poll |
| GET | `/api/polls/:id` | Get poll |
| POST | `/api/polls/:id/vote` | Vote |
| POST | `/api/polls/:id/close` | Close poll |

### Broadcast Lists
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/broadcast` | Create list |
| GET | `/api/broadcast` | List broadcasts |
| POST | `/api/broadcast/:id/send` | Send to list |

### Settings
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings/conversation` | Get settings |
| POST | `/api/settings/disappearing` | Set disappearing |
| POST | `/api/settings/mute` | Mute conversation |

### Profile
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/profile` | Get my profile |
| PUT | `/api/profile` | Update profile |
| GET | `/api/profile/:userId` | Get user profile |

### Archive
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/archive` | Archive chat |
| DELETE | `/api/archive` | Unarchive chat |
| GET | `/api/archive` | List archived |

### Read Receipts
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/receipts/read` | Mark as read |
| GET | `/api/receipts/:messageId` | Get receipts |
| GET | `/api/receipts/unread` | Unread count |

### Themes
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/themes` | Get theme |
| POST | `/api/themes` | Set theme |
| GET | `/api/themes/presets` | Get presets |

### Admin (Moderation)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/admin/review` | Get pending review |
| POST | `/api/admin/review/:id` | Review content |

### WebSocket
| Endpoint | Description |
|----------|-------------|
| `WS /ws?token=<jwt>` | Real-time messaging |

## WebSocket Messages

### Send Message
```json
{"type": "message", "to": "user_id", "content": "Hello!"}
```

### Group Message
```json
{"type": "message", "group_id": "group_id", "content": "Hello group!"}
```

### Typing Indicator
```json
{"type": "typing", "to": "user_id", "typing": true}
```

### Read Receipt
```json
{"type": "ack", "message_id": "...", "status": "read"}
```

### Reaction
```json
{"type": "reaction", "message_id": "...", "emoji": "👍", "action": "add"}
```

### Edit Message
```json
{"type": "message_edit", "message_id": "...", "content": "Updated content"}
```

### Delete Message
```json
{"type": "message_delete", "message_id": "...", "delete_for": "everyone"}
```

## Project Structure

```
├── backend/
│   ├── cmd/server/          # Entry point
│   └── internal/
│       ├── api/             # HTTP handlers & routes
│       │   ├── handlers/    # Request handlers
│       │   └── middleware/  # Auth, rate limiting, roles
│       ├── database/        # SQLite setup
│       ├── models/          # Data models
│       ├── services/        # Business logic
│       │   ├── push.go          # Push coordination
│       │   ├── push_firebase.go # FCM provider
│       │   ├── push_apns.go     # APNs provider
│       │   └── push_webpush.go  # Web Push provider
│       └── websocket/       # Real-time messaging
│
└── app/
    └── lib/
        ├── models/          # Data models
        ├── providers/       # Riverpod state
        ├── screens/         # UI screens
        ├── services/        # API & WebSocket clients
        └── widgets/         # Reusable UI components
```

## Environment Variables

### Required
| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `JWT_SECRET` | JWT signing key | Random (dev only) |

### Content Moderation
| Variable | Description | Default |
|----------|-------------|---------|
| `USE_MOCK_MODERATION` | Skip real scanning | `true` |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP credentials path | - |

### Push Notifications (Firebase)
| Variable | Description |
|----------|-------------|
| `FIREBASE_CREDENTIALS_PATH` | Path to service account JSON |
| `FIREBASE_CREDENTIALS_JSON` | Inline JSON credentials |

### Push Notifications (APNs - Direct)
| Variable | Description |
|----------|-------------|
| `APNS_BUNDLE_ID` | iOS app bundle ID |
| `APNS_KEY_PATH` | Path to .p8 key file |
| `APNS_KEY_ID` | Key ID from Apple |
| `APNS_TEAM_ID` | Team ID from Apple |
| `APNS_DEVELOPMENT` | Use sandbox (`true`/`false`) |

### Push Notifications (Web Push)
| Variable | Description |
|----------|-------------|
| `VAPID_PUBLIC_KEY` | VAPID public key |
| `VAPID_PRIVATE_KEY` | VAPID private key |
| `VAPID_SUBSCRIBER` | Contact email (mailto:...) |

## Push Notification Setup

The app supports multiple push providers. You can configure one or more:

### Firebase Cloud Messaging (Recommended for mobile)
1. Create a Firebase project
2. Download service account JSON
3. Set `FIREBASE_CREDENTIALS_PATH` or `FIREBASE_CREDENTIALS_JSON`
4. Add `google-services.json` (Android) and `GoogleService-Info.plist` (iOS) to the Flutter app

### Direct APNs (iOS without Firebase)
1. Create an APNs key in Apple Developer Console
2. Set the APNS environment variables

### Web Push (Browsers without Firebase)
1. Generate VAPID keys: run `services.GenerateVAPIDKeys()` or use online generator
2. Set VAPID environment variables

## Rate Limiting

The API implements rate limiting to prevent abuse:

- **Authentication**: 5 requests/minute per IP
- **General API**: 100 requests/minute per user
- **Media uploads**: 10 uploads/minute per user

## Monitoring

### Health Check
```bash
curl http://localhost:8080/health
```

### Metrics (JSON)
```bash
curl http://localhost:8080/metrics
```

### Prometheus Metrics
```bash
curl http://localhost:8080/metrics/prometheus
```

### Kubernetes Probes
- Liveness: `GET /healthz`
- Readiness: `GET /readyz`

## Tech Stack

**Backend:**
- Go 1.21+
- Fiber (web framework)
- GORM (ORM)
- SQLite
- Google Cloud Vision API
- Firebase Admin SDK
- APNs HTTP/2 client
- Web Push (VAPID)

**Mobile:**
- Flutter 3.x
- Riverpod (state management)
- Dio (HTTP client)
- web_socket_channel
- Drift (offline database)
- connectivity_plus

## License

MIT
