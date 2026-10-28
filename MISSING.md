# Missing after the Electron to Wails migration

What the Electron build did that the Wails build does not do yet, plus
leftovers that still assume Electron. Roughly in order of user impact.

## Features

### Google / Apple sign-in (SSO)

The Electron build opened Storytel's web login in a separate window, then read
the `storytel_session` cookie and the Firebase refresh token from that
window's IndexedDB (`src/modules/sso.ts` in the old tree). Wails 3 can open
the window, but cannot return values from `ExecJS` or read HttpOnly cookies.

- Current state: `window.electronAuth` is not defined, so `LoginForm` hides
  the SSO buttons. Email and password login works. The backend side
  (`POST /api/sso-login` and the Firebase token refresh in `storytel.go`) is
  already ported.
- Possible approach: inject a script into the login window that posts the
  IndexedDB entry to a `/native/sso` route, and get the session cookie
  through a platform-specific cookie store API (WKHTTPCookieStore on macOS,
  ICoreWebView2CookieManager on Windows).

### Auto-updater

`electron-updater` checked GitHub releases on startup. Nothing replaces it.
Options: Wails 3's updater support, or a GitHub releases check that only
notifies.

## Packaging and release

### CI workflows

`.github/workflows/build.yml` and `prerelease.yml` still call the removed
`electron:dist:*` scripts and rely on electron-builder for tagging and GitHub
release uploads. They fail as they are and need rewriting for Go builds on
each platform runner.

### Signing and notarization (macOS)

`wails3 package` only ad-hoc signs, which is enough for local use. A release
build needs a Developer ID signature, hardened runtime and notarization; the
Taskfile has `darwin:sign` and `darwin:sign:notarize` tasks for that (set up
with `wails3 setup`), but they have not been tried.
`server/assets/entitlements.mac.plist` is kept for that, but nothing uses
it yet.

### Windows and Linux builds

The Wails template's tasks are in place (`build/windows`, `build/linux`: NSIS
and MSIX installers, AppImage, deb, rpm, Arch) and point at `server/`, but
nothing has been built or tested on those platforms. Docker cross-compilation
from the template (`build:docker`) assumes the Go module at the repo root and
will not work as-is. The
code has no macOS-only paths except language detection in `i18n.go`, which
falls back to `LANG` on other platforms.

### AUR package

`aur/PKGBUILD` installs the `.deb` from GitHub releases, so it keeps working
only for as long as Linux releases ship a `.deb` with the same name. Its
dependencies will change (`webkit2gtk-4.1` and `gtk3` instead of Electron's
bundled runtime).

### Homebrew cask

INSTALL.md points at `debba/homebrew-storytel-player`, which installs the
Electron DMG. It needs updating once Wails releases exist.

## Cleanup

- **Bridge names**: the frontend still calls `window.electronStore`,
  `window.electronLocale` and the like, recreated by
  `client/src/platform/wails.ts`. Renaming them (for example to
  `window.desktop.*`) is optional.
- **Code comments**: some comments in `client/src/i18n.ts` still mention the
  Electron store or IPC.
- **CHANGELOG**: it has no entry for the migration yet.
- **Version bumps**: `npm version` runs `scripts/sync-version.js`, which still
  writes the deleted `src/version.ts`. The version now lives in `package.json`,
  `server/main.go` (`appVersion`), `build/config.yml` and the generated files
  under `build/` (plists, Windows manifests and installer files, nfpm.yaml);
  `wails3 task common:update:build-assets` regenerates the latter from
  `build/config.yml`, but Wails warns it overwrites changes made under `build/`,
  so check the diff afterwards.
- **DMG artwork**: the DMG has no background image or file icon (both are
  optional); see "DMG appearance" in [DEVELOPMENT.md](DEVELOPMENT.md).
- **Separate data folder**: the app uses `storytel-player-wails/` and copies the
  Electron `config.json` once. If Wails replaces Electron for good, it could
  use `storytel-player/` directly, but it must not run alongside an Electron
  install that writes the same file.
