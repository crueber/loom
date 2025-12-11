# Loom

A self-hosted, minimalistic bookmark manager with a beautiful Fizzy-like interface. Built with Go and designed for containerized deployment.

## Features

### Core Functionality
- **Multiple Boards**: Organize bookmarks across separate boards for different contexts (work, personal, projects)
- **Fizzy-like Interface**: Organize bookmarks in draggable lists with horizontal and vertical drag-and-drop
- **Notes with Markdown**: Add markdown-formatted notes alongside bookmarks with custom color syntax
- **Card Flip UI**: Clean configuration interface - click the gear icon (⚙️) to flip cards and edit
- **Flexible Color Picker**: Choose from 8 preset colors or select any custom hex color for list headers
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
- **OAuth2 Authentication**: Secure authentication via OpenID Connect (OIDC) providers like Authentik
- **Import/Export**: Backup and restore your bookmarks as JSON
- **Dark Mode**: Beautiful dark theme with readable color palette
- **Minimal Footprint**: Docker image < 15MB
- **Secure**: OAuth2/OIDC authentication, secure cookie-based sessions
- **Fast**: Lightweight Go backend with SQLite database

## Prerequisites

**IMPORTANT: OAuth2 authentication is required.** Loom uses OAuth2/OIDC for authentication and does not support local password authentication. You must have an OAuth2 provider (such as Authentik, Keycloak, or any OIDC-compliant provider) configured before running Loom.

## Quick Start

### Step 1: Configure OAuth2 Provider (Authentik Example)

1. **Create an OAuth2/OIDC Application in Authentik:**
   - Go to your Authentik admin panel → Applications → Providers
   - Click "Create" → Select "OAuth2/OpenID Provider"
   - Configure the provider:
     - **Name**: Loom
     - **Authorization flow**: Explicit consent or Implicit consent
     - **Redirect URIs**: `http://localhost:8080/auth/callback` (adjust for your domain)
     - **Scopes**: Make sure `openid`, `profile`, and `email` are included
     - **ID Token encryption**: **Disable** (leave ID token as signed JWT, not encrypted)
   - Save and note the **Client ID** and **Client Secret**

2. **Find your Issuer URL:**
   - In Authentik, go to your provider details
   - Copy the **OpenID Configuration Issuer** URL
   - Example: `https://auth.example.com/application/o/loom/`
   - **Note**: Do not include `/.well-known/openid-configuration` - Loom will append this automatically

### Step 2: Generate Session Keys

Generate secure keys for session management:

```bash
# Generate session key (32 bytes = 64 hex characters)
openssl rand -hex 32

# Generate encryption key (32 bytes = 64 hex characters)
openssl rand -hex 32
```

Save these keys - you'll need them in the next step.

### Step 3: Configure Environment Variables

1. **Copy the example environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with your OAuth2 configuration:**
   ```bash
   # OAuth2 Configuration (REQUIRED)
   OAUTH2_ISSUER_URL=https://auth.example.com/application/o/loom/
   OAUTH2_CLIENT_ID=your_client_id_from_authentik
   OAUTH2_CLIENT_SECRET=your_client_secret_from_authentik
   OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback

   # Session Keys (REQUIRED - Use the keys generated in Step 2)
   SESSION_KEY=your_64_character_hex_string_here
   ENCRYPTION_KEY=your_64_character_hex_string_here

   # Optional Configuration
   DATABASE_PATH=/data/bookmarks.db
   PORT=8080
   SESSION_MAX_AGE=31536000
   SECURE_COOKIE=false  # Set to true in production with HTTPS
   ```

### Step 4: Start the Application

1. **Start the container:**
   ```bash
   docker-compose up -d
   ```

2. **Access the application:**
   Open your browser to [http://localhost:8080](http://localhost:8080)

3. **First Login:**
   - Click "Log In"
   - You'll be redirected to your OAuth2 provider (Authentik)
   - Log in with your provider credentials
   - You'll be redirected back to Loom
   - **Auto-provisioning**: A new user account and default board will be created automatically on first login

### Alternative: Docker Run (Without docker-compose)

```bash
# Create a volume for data persistence
docker volume create loom-data

# Run the container with environment variables
docker run -d \
  --name loom \
  -p 8080:8080 \
  -v loom-data:/data \
  -e OAUTH2_ISSUER_URL=https://auth.example.com/application/o/loom/ \
  -e OAUTH2_CLIENT_ID=your_client_id \
  -e OAUTH2_CLIENT_SECRET=your_client_secret \
  -e OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback \
  -e SESSION_KEY=your_64_char_hex_string \
  -e ENCRYPTION_KEY=your_64_char_hex_string \
  loom:latest
```

### Local Development

**Note**: Even in local development, OAuth2 configuration is required.

1. **Prerequisites:**
   - Go 1.23 or later
   - OAuth2 provider (Authentik, Keycloak, etc.) configured

2. **Clone and build:**
   ```bash
   git clone <repository-url>
   cd loom
   go mod download
   go build -o bin/server ./cmd/server
   ```

3. **Set environment variables:**
   ```bash
   export OAUTH2_ISSUER_URL=https://auth.example.com/application/o/loom/
   export OAUTH2_CLIENT_ID=your_client_id
   export OAUTH2_CLIENT_SECRET=your_client_secret
   export OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback
   export SESSION_KEY=$(openssl rand -hex 32)
   export ENCRYPTION_KEY=$(openssl rand -hex 32)
   ```

4. **Run the server:**
   ```bash
   ./bin/server
   ```

5. **Access the application:**
   Open [http://localhost:8080](http://localhost:8080) and click "Log In"

## Configuration

Configuration is done via environment variables:

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OAUTH2_ISSUER_URL` | OAuth2 provider issuer URL (without .well-known suffix) | `https://auth.example.com/application/o/loom/` |
| `OAUTH2_CLIENT_ID` | OAuth2 client ID from your provider | `abc123` |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret from your provider | `secret123` |
| `OAUTH2_REDIRECT_URL` | OAuth2 callback URL (must match provider config) | `http://localhost:8080/auth/callback` |
| `SESSION_KEY` | 32-byte hex key for session signing (64 chars) | Generate with `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | 32-byte hex key for cookie encryption (64 chars) | Generate with `openssl rand -hex 32` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_PATH` | Path to SQLite database file | `./data/bookmarks.db` |
| `PORT` | HTTP server port | `8080` |
| `SESSION_MAX_AGE` | Session duration in seconds | `31536000` (1 year) |
| `SECURE_COOKIE` | Enable secure cookies (HTTPS only) | `false` |

### Example .env File

See [`.env.example`](.env.example) for a complete example configuration file.

## User Management

**Auto-Provisioning**: Users are automatically created when they first log in via OAuth2. No manual user management is required.

- Users are identified by their **email address** from the OAuth2 provider
- On first login, a new user account is created automatically
- A default board "My Bookmarks" is created for each new user
- Existing users (identified by email) will log in to their existing account

## Usage Guide

### Creating Lists

1. Click the "+ Add List" button on the right
2. A new list card appears, flipped to show the configuration panel
3. Enter a title (required)
4. Optionally change the color:
   - Click one of the 8 preset color buttons for quick selection
   - Or use the color picker to choose any custom hex color
   - Default color is Blue (#3D6D95)
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
  - Change color from 8 preset colors or choose any custom hex color
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

## Technology Stack

**Backend:**
- Go 1.23+
- Chi router (lightweight HTTP router)
- SQLite (modernc.org/sqlite - pure Go, CGO-free)
- Gorilla sessions (secure cookie-based sessions)
- OAuth2/OIDC authentication (go-oidc, golang.org/x/oauth2)

**Frontend:**
- Alpine JS (~15KB)
- Pico.css (minimal CSS framework, ~10KB)
- SortableJS (drag-and-drop, ~2KB)

## Security Considerations

1. **HTTPS**: Use a reverse proxy (Caddy, nginx, Traefik) for HTTPS in production
2. **Secure Cookies**: Set `SECURE_COOKIE=true` when using HTTPS
3. **Session Keys**: Generate strong random keys for `SESSION_KEY` and `ENCRYPTION_KEY` in production
4. **OAuth2 Configuration**:
   - Keep `OAUTH2_CLIENT_SECRET` secret and never commit to version control
   - Use HTTPS for your OAuth2 redirect URL in production
   - Disable ID token encryption in your OAuth2 provider (use signed tokens instead)
   - Ensure your OAuth2 provider includes `openid`, `profile`, and `email` scopes
5. **Rate Limiting**: Consider adding rate limiting at the reverse proxy level
6. **Backups**: Regularly backup your SQLite database and exported JSON

## Troubleshooting

### Cannot access the application

- Check if the server is running: `docker ps`
- Check logs: `docker logs loom`
- Verify port is not in use: `lsof -i :8080`
- Ensure all required OAuth2 environment variables are set

### OAuth2 login fails

- **"Failed to initialize OAuth2 client"**: Check that `OAUTH2_ISSUER_URL` is correct and does not include `/.well-known/openid-configuration`
- **"Invalid session state"**: This can happen if cookies are blocked or if using SameSite=Strict. Loom uses SameSite=Lax which should work correctly.
- **"Failed to verify ID token"**: Ensure ID token encryption is **disabled** in your OAuth2 provider. The ID token should be signed (JWT) but not encrypted (JWE).
- **Check redirect URL**: Ensure `OAUTH2_REDIRECT_URL` matches exactly what's configured in your OAuth2 provider

### Auto-provisioning fails

- Check logs for "Failed to provision user" errors
- Ensure the OAuth2 provider is sending an `email` claim in the ID token
- Verify database is writable

### Favicons not loading

- Favicon fetching uses Google's favicon service
- Requires outbound HTTPS access
- Some sites may not have favicons

### Session expires too quickly

- Check `SESSION_MAX_AGE` environment variable
- Default is 1 year (31536000 seconds)

## Development Goals

- Target 20-30 bookmarks per list, ~10 lists per page.
- Aims for 1000 links per user while keeping near 100ms response times.
- Small codebase, memory footprint, both in browser and server.
- Less than 40kb cache size (under 150kb fresh) for page load.

## License

MIT License - feel free to use and modify as needed.

## Contributing

This is a personal project, but suggestions and bug reports are welcome!

---

Built with ❤️ using Go and vanilla JavaScript.
