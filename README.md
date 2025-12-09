# Loom

A self-hosted, minimalistic bookmark manager with a beautiful Trello-like interface. Built with Go and designed for containerized deployment.

## Features

### Core Functionality
- **Multiple Boards**: Organize bookmarks across separate boards for different contexts (work, personal, projects)
- **Trello-like Interface**: Organize bookmarks in draggable lists with horizontal and vertical drag-and-drop
- **Notes with Markdown**: Add markdown-formatted notes alongside bookmarks with custom color syntax
- **Card Flip UI**: Clean configuration interface - click the gear icon (⚙️) to flip cards and edit
- **Copy/Move Lists**: Transfer lists between boards with all items intact
- **Collapsible Lists**: Save screen space by collapsing lists to vertical tabs
- **Adaptive Compression**: Bookmark URLs automatically hide when lists have 7+ bookmarks for compact display

### User Experience
- **Mobile Responsive**: Hamburger menu (< 768px) with full feature access on mobile devices
- **Touch-Optimized**: Long-press to drag (200ms), always-visible controls, 44px touch targets
- **Drag-to-Scroll**: Click and drag the whitespace to smoothly scroll through your lists horizontally
- **Keyboard Shortcuts**: ESC to close configuration panels, Enter to save changes
- **Inline Confirmations**: No browser confirm dialogs - all destructive actions use inline UI
- **Instant Loading**: LocalStorage caching with background refresh for instant page loads
- **Automatic Favicons**: Automatically fetches and displays favicons for your bookmarks
- **Stealth UI**: Minimal, unobtrusive navigation that fades in when needed

### Technical Features
- **Multi-user Support**: Each user has their own isolated bookmarks and boards
- **Import/Export**: Backup and restore your bookmarks as JSON
- **Dark Mode**: Beautiful dark theme with readable color palette
- **Minimal Footprint**: Docker image < 15MB
- **Secure**: Argon2id password hashing, secure cookie-based sessions
- **Fast**: Lightweight Go backend with SQLite database

## Quick Start

### Docker (Recommended)

1. **Start the container:**
   ```bash
   docker-compose up -d
   ```

2. **Create your first user:**
   ```bash
   docker exec -it loom /user create admin
   ```
   You'll be prompted to enter a password.

3. **Access the application:**
   Open your browser to [http://localhost:8080](http://localhost:8080)

### Docker Run

```bash
# Create a volume for data persistence
docker volume create loom-data

# Run the container
docker run -d \
  --name loom \
  -p 8080:8080 \
  -v loom-data:/data \
  loom:latest

# Create a user
docker exec -it loom /user create admin
```

### Local Development

1. **Prerequisites:**
   - Go 1.23 or later
   - Make (optional)

2. **Clone and build:**
   ```bash
   git clone <repository-url>
   cd loom
   go mod download
   go build -o bin/server ./cmd/server
   go build -o bin/user ./cmd/user
   ```

3. **Create a user:**
   ```bash
   ./bin/user create admin
   ```

4. **Run the server:**
   ```bash
   ./bin/server
   ```

5. **Access the application:**
   Open [http://localhost:8080](http://localhost:8080)

## Configuration

Configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_PATH` | Path to SQLite database file | `./data/bookmarks.db` |
| `PORT` | HTTP server port | `8080` |
| `SESSION_KEY` | 32-byte hex key for session signing | Auto-generated |
| `ENCRYPTION_KEY` | 32-byte hex key for cookie encryption | Auto-generated |
| `SESSION_MAX_AGE` | Session duration in seconds | `31536000` (1 year) |
| `SECURE_COOKIE` | Enable secure cookies (HTTPS only) | `false` |

### Generating Secure Keys (Production)

```bash
# Generate session key
openssl rand -hex 32

# Generate encryption key
openssl rand -hex 32
```

Add these to your `docker-compose.yml` or environment:

```yaml
environment:
  - SESSION_KEY=your_generated_session_key
  - ENCRYPTION_KEY=your_generated_encryption_key
  - SECURE_COOKIE=true  # If behind HTTPS
```

## User Management

The `user` CLI tool provides user management commands:

```bash
# Create a new user
./bin/user create <username>

# Delete a user
./bin/user delete <username>

# List all users
./bin/user list

# Reset a user's password
./bin/user reset-password <username>
```

**With Docker:**

```bash
docker exec -it loom /user <command>
```

## Usage Guide

### Creating Lists

1. Click the "+ Add List" button on the right
2. A new list card appears, flipped to show the configuration panel
3. Enter a title (required)
4. Optionally change the color from the default (Blue)
5. Click "Save" to create the list, or "Cancel" / press ESC to discard
   - Saving with an empty title will cancel the operation

### Adding Bookmarks

1. Click "+ Add Bookmark" at the bottom of any list
2. A new bookmark card appears, flipped to show the configuration panel
3. Enter a URL (required) and title (required)
4. Click "Save" to create the bookmark, or "Cancel" / press ESC to discard
   - Saving with empty fields will cancel the operation
5. Favicon will be fetched automatically after creation

### Organizing

- **Drag lists horizontally** by their header to reorder them (auto-scrolls near edges)
- **Drag-to-scroll**: Click and drag on list whitespace or container background to scroll horizontally
- **Drag bookmarks vertically** within and between lists
- **Click list header** to collapse/expand (lists collapse to vertical tabs)
- **Configure lists**: Click the gear icon (⚙️) to flip the card and:
  - Edit list title
  - Change color from 8-color inline picker
  - Delete list (with confirmation)
- **Configure bookmarks**: Click the gear icon (⚙️) to flip the card and:
  - Edit bookmark title and URL
  - Delete bookmark (with confirmation)
- **Keyboard shortcuts**:
  - `ESC` - Close any open configuration panel
  - `Enter` - Save changes (when in input fields)
- **Mobile**: Long-press (200ms) to initiate drag, tap normally to open links or configurations

### Import/Export

- **Export**: Click "Export" to download your bookmarks as JSON
- **Import**: Click "Import", choose a file, and select merge or replace mode
  - **Merge**: Adds new data, updates existing by ID
  - **Replace**: Deletes all data and imports fresh

## Architecture

### Technology Stack

**Backend:**
- Go 1.23+
- Chi router (lightweight HTTP router)
- SQLite (modernc.org/sqlite - pure Go, CGO-free)
- Gorilla sessions (secure cookie-based sessions)
- Argon2id (password hashing)

**Frontend:**
- Vanilla JavaScript (no frameworks)
- Pico.css (minimal CSS framework, ~10KB)
- SortableJS (drag-and-drop, ~2KB)

**Deployment:**
- Docker multi-stage build
- Scratch base image for minimal size
- Single binary deployment

### Project Structure

```
loom/
├── cmd/
│   ├── server/          # Main server application
│   │   ├── main.go
│   │   └── static/      # Embedded frontend assets
│   └── user/            # User management CLI
├── internal/
│   ├── api/             # HTTP handlers
│   ├── auth/            # Authentication & sessions
│   ├── db/              # Database layer
│   ├── favicon/         # Favicon fetching
│   └── models/          # Data models
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## Security Considerations

1. **HTTPS**: Use a reverse proxy (Caddy, nginx, Traefik) for HTTPS in production
2. **Secure Cookies**: Set `SECURE_COOKIE=true` when using HTTPS
3. **Session Keys**: Generate and set `SESSION_KEY` and `ENCRYPTION_KEY` in production
4. **Rate Limiting**: Consider adding rate limiting at the reverse proxy level
5. **Backups**: Regularly backup your SQLite database and exported JSON

## Reverse Proxy Setup

### Caddy Example

```caddyfile
bookmarks.example.com {
    reverse_proxy localhost:8080
}
```

### nginx Example

```nginx
server {
    listen 443 ssl http2;
    server_name bookmarks.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Color Palette

Darker, more readable colors optimized for dark mode:

- Blue: `#3D6D95`
- Green: `#4D7831`
- Orange: `#B85720`
- Red: `#A43529`
- Purple: `#6B3D7D`
- Pink: `#924F7D`
- Teal: `#358178`
- Gray: `#697374`

## Database Schema

### users
- `id` - Primary key
- `username` - Unique username
- `password_hash` - Argon2id hash
- `created_at` - Timestamp

### lists
- `id` - Primary key
- `user_id` - Foreign key to users
- `title` - List title
- `color` - Hex color code
- `position` - Sort order
- `collapsed` - Boolean collapsed state
- `created_at` - Timestamp

### bookmarks
- `id` - Primary key
- `list_id` - Foreign key to lists
- `title` - Bookmark title
- `url` - Bookmark URL
- `favicon_url` - Cached favicon URL
- `position` - Sort order within list
- `created_at` - Timestamp

## Troubleshooting

### Cannot access the application

- Check if the server is running: `docker ps`
- Check logs: `docker logs loom`
- Verify port is not in use: `lsof -i :8080`

### User creation fails

- Ensure database directory exists and is writable
- Check container logs for database errors

### Favicons not loading

- Favicon fetching uses Google's favicon service
- Requires outbound HTTPS access
- Some sites may not have favicons

### Session expires too quickly

- Check `SESSION_MAX_AGE` environment variable
- Default is 1 year (31536000 seconds)

## Performance

- Targets 20-30 bookmarks per list, ~10 lists
- Supports up to 1000 bookmarks per user
- Sub-50ms API response times
- Minimal memory footprint (~20MB runtime)

## License

MIT License - feel free to use and modify as needed.

## Contributing

This is a personal project, but suggestions and bug reports are welcome!

## Roadmap

Future enhancements to consider:
- [ ] Browser extension for quick bookmark additions
- [ ] Bookmark tags and search
- [ ] Bookmark screenshots/thumbnails
- [ ] Shared lists between users
- [ ] API keys for automation
- [ ] Mobile app

---

Built with ❤️ using Go and vanilla JavaScript.
