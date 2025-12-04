# Home Links

A self-hosted, minimalistic bookmark manager with a beautiful Trello-like interface. Built with Go and designed for containerized deployment.

## Features

- **Trello-like Interface**: Organize bookmarks in draggable lists with horizontal and vertical drag-and-drop
- **Collapsible Lists**: Save screen space by collapsing lists horizontally
- **Automatic Favicons**: Automatically fetches and displays favicons for your bookmarks
- **Multi-user Support**: Each user has their own isolated bookmarks
- **Import/Export**: Backup and restore your bookmarks as JSON
- **Dark Mode**: Beautiful dark theme powered by Pico.css
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
   docker exec -it home-links /user create admin
   ```
   You'll be prompted to enter a password.

3. **Access the application:**
   Open your browser to [http://localhost:8080](http://localhost:8080)

### Docker Run

```bash
# Create a volume for data persistence
docker volume create home-links-data

# Run the container
docker run -d \
  --name home-links \
  -p 8080:8080 \
  -v home-links-data:/data \
  home-links:latest

# Create a user
docker exec -it home-links /user create admin
```

### Local Development

1. **Prerequisites:**
   - Go 1.23 or later
   - Make (optional)

2. **Clone and build:**
   ```bash
   git clone <repository-url>
   cd home-links
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
docker exec -it home-links /user <command>
```

## Usage Guide

### Creating Lists

1. Click the "+ Add List" button on the right
2. Enter a list title
3. Choose a color from the palette

### Adding Bookmarks

1. Click "+ Add Bookmark" at the bottom of any list
2. Enter the URL and title
3. Favicon will be fetched automatically

### Organizing

- **Drag lists horizontally** to reorder them
- **Drag bookmarks vertically** within and between lists
- **Click list header** to collapse/expand
- **Edit items** using the edit icon (✏️)
- **Change list colors** using the color palette icon (🎨)
- **Delete items** using the trash icon (🗑️)

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
home-links/
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

- Blue: `#0072ce`
- Green: `#61bd4f`
- Orange: `#ff9f1a`
- Red: `#eb5a46`
- Purple: `#c377e0`
- Pink: `#ff78cb`
- Teal: `#00c2e0`
- Gray: `#b3bac5`

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
- Check logs: `docker logs home-links`
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
