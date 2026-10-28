# Storytel Player for Desktop

![](https://img.shields.io/github/release/debba/storytel-player.svg?style=flat)
![](https://img.shields.io/github/downloads/debba/storytel-player/total.svg?style=flat)
![Build & Release](https://github.com/debba/storytel-player/workflows/Build%20&%20Release/badge.svg)
[![Known Vulnerabilities](https://snyk.io//test/github/debba/storytel-player/badge.svg?targetFile=package.json)](https://snyk.io//test/github/debba/storytel-player?targetFile=package.json)
[![Discord](https://img.shields.io/discord/1470772941296894128?color=5865F2&logo=discord&logoColor=white)](https://discord.gg/YrZPHAwMSG)
![i18n](https://img.shields.io/badge/i18n-EN%20|%20IT%20|%20DE%20|%20ES%20|%20FR%20|%20SV-blue)

**Unoffical Storytel Player for Desktop** — a cross-platform desktop application for playing Storytel audiobooks, built with React, TypeScript, Go and [Wails](https://wails.io/).

**Discord** - [Join our discord server](https://discord.gg/YrZPHAwMSG) and chat with the maintainers.

<img src="docs/assets/og_image.png" width="90%"></img>

## Release Download:
[![Windows](https://img.shields.io/badge/Windows-Download-blue?logo=windows)](https://github.com/debba/storytel-player/releases/download/v1.4.1/Storytel-Player-Setup-1.4.1.exe) [![macOS Intel](https://img.shields.io/badge/macOS_Intel-Download-black?logo=apple)](https://github.com/debba/storytel-player/releases/download/v1.4.1/Storytel-Player-1.4.1-mac-x64.dmg) [![macOS Apple Silicon](https://img.shields.io/badge/macOS_Apple_Silicon-Download-black?logo=apple)](https://github.com/debba/storytel-player/releases/download/v1.4.1/Storytel-Player-1.4.1-mac-arm64.dmg) [![Linux](https://img.shields.io/badge/Linux-Download-green?logo=linux)](https://github.com/debba/storytel-player/releases/download/v1.4.1/Storytel-Player-1.4.1.AppImage)

## What's New

See the [CHANGELOG](CHANGELOG.md) for the full list of changes across all releases.

---

✅ **Modern Desktop Interface**: Dark theme with Storytel Orange accents, rounded cards and filter pills, laid out for a 1280×800 desktop window

✅ **Catalog Search**: Search the whole Storytel catalog by title, author or narrator (Ctrl/Cmd+K). Filters, results and a live preview side by side; add to your library or start listening in one click. Results are scoped to your Storytel market and limited to audiobooks

✅ **Background Playback**: Audio keeps playing while you navigate; a persistent mini-player keeps play/pause, skip, speed, chapters and sleep timer within reach

✅ **Native Desktop App**: Cross-platform application with system tray integration and single instance lock

✅ **Audiobook Library**: Browse your Storytel bookshelf with cover art, progress tracking and status filters

✅ **Audio Player**: Chapters, bookmarks, playback speed, sleep timer, multiple audio tracks and automatic resume

✅ **Session Management**: Secure authentication with persistent local storage, automatic session refresh and transparent retry

✅ **Internationalization**: English, Italian, German, Spanish, French, Swedish and Finnish (i18next) with automatic language detection

✅ **Cross-Platform**: Available for Windows, macOS (x64/ARM64), and Linux (x64/ARM64/ARMv7)

✅ **Action Logging**: Internal logging of HTTP requests, responses, and user actions, exportable for debugging. Log rotation is applied automatically (max 10 MB per file, up to 5 archived files)

✅ **Offline Listening**: Download audiobooks for offline playback

> [!NOTE]
> This is an independent project, not affiliated with Storytel

## Architecture

The app is a single native binary built with Wails 3. It uses the operating
system's webview (WKWebView on macOS, WebView2 on Windows, WebKitGTK on Linux)
instead of bundling Chromium and Node, so the macOS app is about 11 MB.

- **Frontend** (`client/`): React 18, TypeScript, Tailwind CSS and Vite
- **Backend** (`server/`): a Go module that is both the desktop shell and the API
  - `main.go`: window, system tray, single instance lock, `/native/*` bridge
  - `api.go`: the `/api/*` routes (auth, bookshelf, streaming, downloads, logs)
  - `storytel.go`: the Storytel API client
  - `store.go`, `i18n.go`, `logger.go`: settings, tray translations, action log
- **No network port**: the React app calls `/api/*` and `/native/*` on its own
  origin; Wails routes those requests straight to the Go handlers in-process
- **Audio Streaming**: Storytel's signed CDN URLs, or local downloads served
  with HTTP Range support
- **Logging**: `app.log` in the config directory, with HTTP traffic and user actions

## Prerequisites

- Go 1.26+
- Node.js 22+ and npm (to build the frontend; see `.nvmrc`)
- Platform webview dependencies, see the
  [Wails installation guide](https://v3.wails.io/getting-started/installation/)

## Installation

### macOS

#### Homebrew (Recommended)

To add our tap, run:

```bash
brew tap debba/storytel-player
```

Then install:

```bash
brew install --cask storytel-player
```

[![Homebrew](https://img.shields.io/badge/Homebrew-Repository-orange?logo=homebrew)](https://github.com/debba/homebrew-storytel-player)

#### Direct Download

When you install storytel-player on macOS, you need to allow accessibility access (Privacy & Security) to the storytel-player app.

If you are upgrading and you already have storytel-player on the allowed list you will need to manually remove them before accessibility access can be granted to the new version.

macOS users who download directly from releases may need to run:

```bash
xattr -c "/Applications/Storytel Player.app"
```

after copying the app to the Applications directory.

### Arch Linux (AUR)

```bash
yay -S storytel-player-bin
```

## Updates

The Wails build does not have an auto-updater yet. Download new versions from
the [Releases page](https://github.com/debba/storytel-player/releases).

## Development

```bash
npm run install-all   # install frontend dependencies
npm run dev           # build the frontend, then run the app with `go run`
```

There is no hot reload yet: rerun `npm run dev` after changes. To work on the UI
alone, `npm run client` starts the Vite dev server on port 3000, but it needs a
backend on `VITE_BACKEND_API_URL` (see `client/.env`).

## Building

```bash
npm run dist:mac      # macOS: build/bin/Storytel Player.app (ad-hoc signed)
```

The frontend is built into `server/frontend/` and embedded into the Go binary,
together with `server/locales/` and `server/assets/`. Windows and Linux
packaging are not set up yet (see [MISSING.md](MISSING.md)).

## Testing

```bash
npm test              # Go unit tests
npm run test:smoke    # live Storytel requests with an existing login token
```

## Scripts

| Command | Description |
|---------|-------------|
| `npm run install-all` | Install frontend dependencies |
| `npm run dev` | Build the frontend and run the app |
| `npm run client` | Run the Vite dev server only |
| `npm run build` | Build the frontend into `server/frontend/` |
| `npm run dist:mac` | Build the macOS app bundle |
| `npm test` | Run the Go unit tests |
| `npm run test:smoke` | Run the live smoke test |

## Configuration

### Data locations

- Settings and login: `config.json` in `storytel-player-wails/` under the user
  config directory (`~/Library/Application Support` on macOS). On first run it
  is copied from the Electron build's `storytel-player/config.json`, so an
  existing login carries over.
- Downloads: `~/Downloads`, named `<title> [<bookId>].mp3`

### Environment Variables

- `JWT_SECRET`: secret for the app's session tokens (defaults to the same value
  the Electron build used, which keeps old logins valid)
- `IS_DEBUG=true`: enable the webview developer tools
- `APP_LOCALE`: fallback UI language when the OS languages are not supported
- `DEVICE_ID`: fixed device id sent with position syncs

## Technologies Used

- **Frontend**: React 18, TypeScript, Tailwind CSS, Vite, i18next, Axios
- **Backend and desktop**: Go, Wails 3
- **Build Tools**: Vite, Go toolchain

## 🌐 Website
Visit our [official documentation](https://debba.github.io/storytel-player/)


## License

MIT License

## Author

Andrea Debernardi (andrea@debbaweb.it)
