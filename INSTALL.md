# Installing Storytel Player

Download the latest build for your platform from the
[Releases page](https://github.com/debba/storytel-player/releases), or use one
of the package managers below. To build from source, see
[DEVELOPMENT.md](DEVELOPMENT.md).

## macOS

### Homebrew (recommended)

Add the tap, then install the cask:

```bash
brew tap debba/storytel-player
brew install --cask storytel-player
```

[![Homebrew](https://img.shields.io/badge/Homebrew-Repository-orange?logo=homebrew)](https://github.com/debba/homebrew-storytel-player)

### Direct download

Open the DMG and drag the app to Applications. The app is not notarized yet,
so macOS may refuse to open it. In that case, run this after copying it:

```bash
xattr -c "/Applications/Storytel Player.app"
```

You also need to allow accessibility access to the app (System Settings,
Privacy & Security). If you are upgrading and an older version is already on
the allowed list, remove it there first, or the new version cannot be granted
access.

## Arch Linux (AUR)

```bash
yay -S storytel-player-bin
```

## Updates

There is no auto-updater yet. Download new versions from the
[Releases page](https://github.com/debba/storytel-player/releases).

## Where data is kept

- Settings and login: `config.json` in `storytel-player-wails/` under the user
  config directory (`~/Library/Application Support` on macOS). On first run it
  is copied from the Electron build's `storytel-player/config.json`, so an
  existing login carries over.
- Action log: `app.log`, next to `config.json`.
- Downloads: `~/Downloads`, named `<title> [<bookId>].mp3`.
