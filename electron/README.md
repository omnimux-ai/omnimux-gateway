# OmniMux Electron Desktop App

This directory contains the Electron wrapper for OmniMux, providing a native desktop application with system tray support for Windows, macOS, and Linux.

## Prerequisites

### 1. Go Binary (Required)
The Electron app requires the compiled Go binary to function. You have two options:

**Option A: Use existing binary (without Go installed)**
```bash
# If you have a pre-built binary (e.g., omnimux-macos)
cp ../omnimux-macos ../omnimux
```

**Option B: Build from source (requires Go)**
TODO

### 2. Electron Dependencies
```bash
cd electron
npm install
```

## Development

Start the backend, the frontend, and Electron in separate terminals:
```bash
# Repository root
go run main.go

# Repository root
make dev-web

# electron/
npm run dev-app
```

This will:
- Use the Go backend on port 3000
- Use the Rsbuild frontend development server on port 5173
- Open an Electron window with DevTools enabled
- Create a system tray icon (menu bar on macOS)
- Store database in `../data/omnimux.db`

## Building for Production

### Quick Build
```bash
# From electron/, build the frontend, Go binary, and desktop package
./build.sh

# Or package an existing binary for the current platform
npm run build

# Platform-specific builds
npm run build:mac    # Creates .dmg and .zip
npm run build:win    # Creates .exe installer
npm run build:linux  # Creates .AppImage and .deb
```

### Build Output
- Built applications are in `electron/dist/`
- macOS: `.dmg` (installer) and `.zip` (portable)
- Windows: `.exe` (installer) and portable exe
- Linux: `.AppImage` and `.deb`

## Configuration

### Port
Default port is 3000. To change, edit `main.js`:
```javascript
const PORT = 3000; // Change to desired port
```

### Database Location
- **Development**: `../data/omnimux.db` (project directory)
- **Production**:
  - macOS: `~/Library/Application Support/OmniMux/data/`
  - Windows: `%APPDATA%/OmniMux/data/`
  - Linux: `~/.config/OmniMux/data/`

## Local network isolation

The packaged application forces its bundled gateway to listen on `127.0.0.1`, including the optional diagnostic server. Other devices cannot reach its first-run setup directly over the network. An inherited `LISTEN_HOST` cannot override this desktop setting. Development mode still uses developer-managed servers. Standalone gateway deployments retain wildcard listening by default and may set `LISTEN_HOST` to an explicit host address. This does not isolate the gateway from other processes on the same machine.
