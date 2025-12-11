<div align="center">

<img src="cmd/server/static/loom-logo.png" width="128" alt="Loom Logo">

# Loom

[Quick Start](#quick-start) • [Configuration](#configuration) • [Features](#features) • [Technology](#technology-stack)

[OAuth2 Setup](#oauth2-provider-setup) • [Troubleshooting](#troubleshooting) • [Contributing](#contributing)

A self-hosted, minimalistic dashboard for links and notes with a beautiful Fizzy-like interface. Built to become your browser's home.

</div>

---

### Features

- **Multiple Boards** - Organize links and notes across boards for different contexts
- **Fizzy-inspired Interface** - Draggable lists with horizontal and vertical drag-and-drop
- **Markdown Notes** - Add markdown-formatted notes with custom color syntax
- **Auto Favicons** - Automatically fetches and displays site favicons
- **Copy/Move Lists** - Transfer lists between boards with all items intact
- **Mobile Responsive** - Full feature access on mobile devices with touch optimization
- **Stealth UI** - Minimal navigation that fades in when needed

### Built for Performance

- **Lightweight** - Go backend with SQLite, AlpineJS front end.
- **Minimal Footprint** - Docker image under 20MB
- **Instant Load** - ~100ms full page load.
- **Fast** - Less than 20kb with cache loaded, ~150kb including cache hydration.

---

## Prerequisites

**⚠️ OAuth2 authentication is required.** 

Loom uses OAuth2/OIDC for authentication and does not support local password authentication. You must have an OAuth2 provider (such as Authentik, Keycloak, or any OIDC-compliant provider) configured before running Loom.

---

## Quick Start

<details>
<summary><strong>🚀 Docker Compose (Recommended)</strong></summary>
<br>

**1. Configure OAuth2 Provider & Generate Keys**

See the [OAuth2 Provider Setup](#oauth2-provider-setup) section below for detailed instructions on configuring Authentik or another OIDC provider.

Generate session keys:
```bash
openssl rand -hex 32  # SESSION_KEY
openssl rand -hex 32  # ENCRYPTION_KEY
```

**2. Configure Environment**

```bash
cp .env.example .env
# Edit .env with your OAuth2 credentials and session keys
```

Required variables in `.env`:
```bash
OAUTH2_ISSUER_URL=https://auth.example.com/application/o/loom/
OAUTH2_CLIENT_ID=your_client_id
OAUTH2_CLIENT_SECRET=your_client_secret
OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback
SESSION_KEY=your_64_char_hex_string
ENCRYPTION_KEY=your_64_char_hex_string
```

**3. Start Loom**

```bash
docker-compose up -d
```

**4. Access & Login**

Open [http://localhost:8080](http://localhost:8080) and click "Login using OAuth2". You'll be redirected to your OAuth2 provider, and upon successful login, your user account and default board will be created automatically.

<hr>
</details>

<details>
<summary><strong>🐳 Docker Run</strong></summary>
<br>

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

<hr>
</details>

---

## OAuth2 Provider Setup

<details>
<summary><strong>🔐 Authentik Configuration</strong></summary>
<br>

**1. Create OAuth2/OIDC Application**

- Go to Authentik admin panel → **Applications** → **Providers**
- Click **"Create"** → Select **"OAuth2/OpenID Provider"**

**2. Configure Provider**

- **Name**: `Loom`
- **Authorization flow**: Explicit consent or Implicit consent
- **Redirect URIs**: `http://localhost:8080/auth/callback` (adjust for your domain)
- **Scopes**: Ensure `openid`, `profile`, and `email` are included
- **⚠️ ID Token encryption**: **DISABLE** (leave ID token as signed JWT, not encrypted)

**3. Save Credentials**

- Note the **Client ID** and **Client Secret**
- Copy the **OpenID Configuration Issuer** URL
  - Example: `https://auth.example.com/application/o/loom/`
  - ⚠️ **Do NOT include** `/.well-known/openid-configuration` (Loom appends this automatically)

<hr>
</details>

<details>
<summary><strong>🔑 Other OIDC Providers (Keycloak, etc.)</strong></summary>
<br>

Loom works with any OIDC-compliant provider. Ensure your provider:

- Supports **OpenID Connect Discovery** (`/.well-known/openid-configuration`)
- Includes **`openid`, `profile`, and `email`** scopes
- Returns an **`email` claim** in the ID token
- Uses **signed JWT tokens** (not encrypted JWE tokens)

Configure the redirect URI to: `http://your-domain:8080/auth/callback`

<hr>
</details>

---

## Configuration

<details>
<summary><strong>⚙️ Environment Variables</strong></summary>
<br>

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `OAUTH2_ISSUER_URL` | OAuth2 provider issuer URL (without .well-known suffix) | `https://auth.example.com/application/o/loom/` |
| `OAUTH2_CLIENT_ID` | OAuth2 client ID from your provider | `abc123` |
| `OAUTH2_CLIENT_SECRET` | OAuth2 client secret from your provider | `secret123` |
| `OAUTH2_REDIRECT_URL` | OAuth2 callback URL (must match provider config) | `http://localhost:8080/auth/callback` |
| `SESSION_KEY` | 32-byte hex key for session signing (64 chars) | Generate with `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | 32-byte hex key for cookie encryption (64 chars) | Generate with `openssl rand -hex 32` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_PATH` | Path to SQLite database file | `./data/loom.db` |
| `PORT` | HTTP server port | `8080` |
| `SESSION_MAX_AGE` | Session duration in seconds | `31536000` (1 year) |
| `SECURE_COOKIE` | Enable secure cookies (HTTPS only) | `false` |

See [`.env.example`](.env.example) for a complete example configuration file.

<hr>
</details>

---

## User Management

**🔄 Auto-Provisioning** - Users are automatically created when they first log in via OAuth2. No manual user management is required.

- Users are identified by their **email address** from the OAuth2 provider
- On first login, a new user account is created automatically
- A default board is created for each new user
- Existing users (identified by email) will log in to their existing account

---

## Usage Guide

<details>
<summary><strong>📝 Creating Lists & Links</strong></summary>
<br>

**Creating Lists**

1. Click the "+ Add List" button on the right
2. A new list card appears, flipped to show the configuration panel
3. Enter a title (required)
4. Optionally change the color:
   - Click one of the 8 preset color buttons
   - Or use the color picker for any custom hex color
   - Default: Blue (#3D6D95)
5. Click "Save" or press ESC to cancel

**Adding Links**

1. Click "+ Add Link" at the bottom of any list
2. Enter a URL (required) and title (required)
3. Click "Save" or press ESC to cancel
4. Favicon fetched automatically

<hr>
</details>

<details>
<summary><strong>🎯 Organizing & Navigation</strong></summary>
<br>

**Drag & Drop**
- **Drag lists horizontally** by header to reorder (auto-scrolls near edges)
- **Drag links vertically** within and between lists
- **Drag-to-scroll**: Click and drag whitespace to scroll horizontally
- **Mobile**: Long-press (200ms) to initiate drag

**List Management**
- **Click list header** to collapse/expand
- **Gear icon (⚙️)** to configure:
  - Edit title
  - Change color
  - Copy/move to other boards
  - Delete list

**Link Management**
- **Gear icon (⚙️)** to configure:
  - Edit title and URL
  - Delete link

**Keyboard Shortcuts**
- `ESC` - Close configuration panel
- `Enter` - Save changes

<hr>
</details>

<details>
<summary><strong>💾 Import/Export</strong></summary>
<br>

**Export**
- Click "Export" to download your links as JSON
- Backup your entire board structure

**Import**
- Click "Import" and choose a JSON file
- **Merge mode**: Adds new data, updates existing by ID
- **Replace mode**: Deletes all data and imports fresh

<hr>
</details>

---

## Technology Stack

**Backend**
- Go 1.23+ • Chi router • SQLite (pure Go, CGO-free)
- Gorilla sessions • OAuth2/OIDC (go-oidc, golang.org/x/oauth2)

**Frontend**
- Alpine.js (~15KB) • Pico.css (~10KB) • SortableJS (~2KB) • Marked.js (markdown rendering)

---

## Development

<details>
<summary><strong>🛠️ Local Development Setup</strong></summary>
<br>

**Prerequisites**
- Go 1.23 or later
- Node.js (for frontend bundling)
- OAuth2 provider configured (see [OAuth2 Provider Setup](#oauth2-provider-setup))

**Build & Run**

```bash
# Clone repository
git clone <repository-url>
cd loom

# Install Go dependencies
go mod download

# Build frontend bundle
cd cmd/server
node build.js
cd ../..

# Set environment variables
export OAUTH2_ISSUER_URL=https://auth.example.com/application/o/loom/
export OAUTH2_CLIENT_ID=your_client_id
export OAUTH2_CLIENT_SECRET=your_client_secret
export OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback
export SESSION_KEY=$(openssl rand -hex 32)
export ENCRYPTION_KEY=$(openssl rand -hex 32)

# Build and run
go build -o bin/server ./cmd/server
./bin/server
```

Access at [http://localhost:8080](http://localhost:8080)

<hr>
</details>

<details>
<summary><strong>🏗️ Build for Production</strong></summary>
<br>

**Docker Build**

```bash
docker build -t loom:latest .
```

The Dockerfile uses multi-stage builds:
- Stage 1: `golang:1.24-alpine` builder (CGO disabled for static binaries)
- Stage 2: `scratch` runtime (final image < 15MB)
- Frontend assets embedded via `//go:embed static`

**Binary Build**

```bash
CGO_ENABLED=0 go build -o loom ./cmd/server
```

<hr>
</details>

---

## Security Considerations

🔒 **Production Checklist**
- Use HTTPS (reverse proxy with nginx, Caddy, or Traefik)
- Set `SECURE_COOKIE=true` when using HTTPS
- Generate strong `SESSION_KEY` and `ENCRYPTION_KEY` (never reuse) in .env
- Keep `OAUTH2_CLIENT_SECRET` secret (never commit to git)
- Use HTTPS for OAuth2 redirect URL in production
- Disable ID token encryption in OAuth2 provider
- Enable rate limiting at reverse proxy level
- Regularly backup SQLite database

---

## Troubleshooting

<details>
<summary><strong>❌ OAuth2 Login Issues</strong></summary>
<br>

**"Failed to initialize OAuth2 client"**
- Check `OAUTH2_ISSUER_URL` is correct
- Do NOT include `/.well-known/openid-configuration` in URL
- Verify provider is accessible

**"Invalid session state"**
- Check cookies are enabled
- Loom uses `SameSite=Lax` for OAuth2 compatibility
- Clear browser cookies and try again

**"Failed to verify ID token"**
- ⚠️ **Disable ID token encryption** in OAuth2 provider
- ID token must be signed JWT, not encrypted JWE
- Check provider scopes include `openid`, `profile`, `email`

**Redirect issues**
- Ensure `OAUTH2_REDIRECT_URL` matches provider configuration exactly
- Check for trailing slashes

<hr>
</details>

<details>
<summary><strong>🔧 General Issues</strong></summary>
<br>

**Cannot access application**
- Check server is running: `docker ps` or `docker logs loom`
- Verify port is free: `lsof -i :8080`
- Ensure all required OAuth2 env vars are set

**Auto-provisioning fails**
- Check logs for "Failed to provision user"
- Verify OAuth2 provider sends `email` claim in ID token
- Check database is writable

**Favicons not loading**
- Requires outbound HTTPS to Google's favicon service
- Some sites may not have favicons

**Session expires too quickly**
- Check `SESSION_MAX_AGE` environment variable
- Default: 31536000 (1 year)

<hr>
</details>

---

## Contributing

This is a personal project, but suggestions and bug reports are welcome! Feel free to:

- 🐛 Report bugs via [GitHub Issues](https://github.com/crueber/loom/issues)
- 💡 Suggest features or improvements
- 🔧 Submit pull requests

---

## License

[MIT License](LICENSE) - feel free to use and modify as needed.

---

<div align="center">

Built with ❤️ using Go and AlpineJS by 👨[crueber](https://x.com/crueber) and 🤖[Claude](https://claude.ai).

**[⬆ Back to Top](#loom)**

</div>
