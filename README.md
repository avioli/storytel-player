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

## Installation

See [INSTALL.md](INSTALL.md) for Homebrew, direct downloads, the AUR package,
updates and where the app keeps its data.

## Development

The app is built with Go and [Wails 3](https://v3.wails.io/) around a React
frontend. See [DEVELOPMENT.md](DEVELOPMENT.md) for the architecture, running,
testing, building and releasing, and [CONTRIBUTING.md](CONTRIBUTING.md) for
how to report bugs and suggest features.

## 🌐 Website
Visit our [official documentation](https://debba.github.io/storytel-player/)


## License

MIT License

## Author

Andrea Debernardi (andrea@debbaweb.it)
