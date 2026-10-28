# Developing Storytel Player

For bug reports, feature requests and the contribution process, see
[CONTRIBUTING.md](CONTRIBUTING.md). For coding guidelines (i18n, patterns),
see [AGENTS.md](AGENTS.md). Known gaps in the Wails port are listed in
[MISSING.md](MISSING.md).

## Architecture

The app is a single native binary built with Wails 3. It uses the operating
system's webview (WKWebView on macOS, WebView2 on Windows, WebKitGTK on Linux)
instead of bundling Chromium and Node, so the macOS app is about 11 MB.

- **Frontend** (`client/`): React 18, TypeScript, Tailwind CSS, Vite, i18next,
  Axios
- **Backend** (`server/`): a Go module that is both the desktop shell and the API
  - `main.go`: window, system tray, single instance lock, `/native/*` bridge
  - `api.go`: the `/api/*` routes (auth, bookshelf, streaming, downloads, logs)
  - `storytel.go`: the Storytel API client
  - `store.go`, `i18n.go`, `logger.go`: settings, tray translations, action log
- **No network port**: the React app calls `/api/*` and `/native/*` on its own
  origin; Wails routes those requests straight to the Go handlers in-process
- **Audio streaming**: Storytel's signed CDN URLs, or local downloads served
  with HTTP Range support
- **Logging**: `app.log` in the config directory, with HTTP traffic and user
  actions

## Prerequisites

- Go 1.26+
- The Wails CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`
  (installs to `~/go/bin`, which needs to be on your `PATH`)
- Node.js 22+ and npm, to build the frontend. `.nvmrc` pins the exact version;
  with nodenv, a missing version breaks the `npm` shim and the Wails tasks
  report "Looks like npm isn't installed". Install that version, or override
  it with `NODENV_VERSION`.
- Platform webview dependencies, see the
  [Wails installation guide](https://v3.wails.io/getting-started/installation/)

Then install the frontend dependencies:

```bash
npm run install-all
```

## Running in development

```bash
wails3 dev            # or: npm run dev
```

This builds the app and opens it with the UI served by the Vite dev server:

- Changes in `client/` show up immediately (hot module replacement)
- Changes to `.go` files or `server/locales/` rebuild and restart the app

The dev server runs on port 9245 (`WAILS_VITE_PORT` changes it).

### Environment variables

- `JWT_SECRET`: secret for the app's session tokens (defaults to the same value
  the Electron build used, which keeps old logins valid)
- `IS_DEBUG=true`: enable the webview developer tools
- `APP_LOCALE`: fallback UI language when the OS languages are not supported
- `DEVICE_ID`: fixed device id sent with position syncs

## Testing

```bash
npm test              # Go unit tests
npm run test:smoke    # live Storytel requests with an existing login token
```

## Building

```bash
wails3 build          # binary in bin/
wails3 package        # macOS: bin/Storytel Player.app (ad-hoc signed)
```

The build is driven by `Taskfile.yml` and `build/`, adapted from the Wails
template for this repo's layout (`FRONTEND_DIR: client`, `GO_DIR: server` in
`Taskfile.yml`). The frontend is built into `server/frontend/` and embedded
into the Go binary, together with `server/locales/` and `server/assets/`.
App metadata lives in `build/config.yml` and the per-platform files under
`build/`. Windows and Linux tasks come from the template but are untested.

## Release

### macOS

```bash
wails3 task darwin:package:dmg       # bin/Storytel Player.dmg
wails3 task darwin:package:universal # arm64 + amd64 app
```

These builds are only ad-hoc signed. Signing with a Developer ID and
notarization (`darwin:sign`, `darwin:sign:notarize`) are not set up yet; see
[MISSING.md](MISSING.md), which also covers the CI workflows, Windows, Linux,
AUR and Homebrew.

### Version

`package.json` holds the version. Bump it with npm, which also updates every
other file that repeats it and the CHANGELOG, then commits and tags:

```bash
npm version 1.5.0     # or: patch, minor, major
```

`scripts/sync-version.mjs` does the copying: `server/main.go` (`appVersion`),
`build/config.yml`, the macOS plists, the Windows manifests and installer
files, `nfpm.yaml` and the README download links. `npm run version:check`
fails if any of them differs from `package.json`, so CI can catch a manual
edit. Avoid `wails3 task common:update:build-assets` for this: it regenerates
the files under `build/` from `build/config.yml` and drops local changes.

### App icon

The macOS icon comes from an Icon Composer document,
`build/darwin/AppIcon.icon`: the Storytel glyph (`Assets/glyph.png`) over a
solid fill, with the glass settings in `icon.json`. Open it in Icon Composer
(bundled with Xcode 26) to change it, then run:

```bash
scripts/make-macos-icon.sh
```

It needs Xcode 26 or later and ImageMagick 7, and writes two committed files,
so the build itself does not need either tool:

- `build/darwin/Assets.car`, compiled by `actool`. macOS 26 and later draw
  the icon from it (`CFBundleIconName` in the plists).
- `build/darwin/icons.icns` for older macOS. `ictool` renders the icon in
  Apple's shape, and the script adds the margin and drop shadow that macOS
  icons carry in the image itself, on Apple's 1024 px grid.

`build/appicon.png` is still the source of the Windows `.ico`, and
`server/assets/icon.png` the window icon on Windows and Linux. The glyph was
cut out of the 512 px `appicon.png`; a vector or larger original would be
sharper.

### DMG appearance

A DMG can carry three images, all optional and all separate from the app's own
icon (`build/darwin/icons.icns`, inside the `.app`):

- **Volume icon**: the mounted disk in Finder and on the desktop. This uses
  the app icon (`DMG_VOLUME_ICON`).
- **File icon**: the icon of the `.dmg` file itself, before it is opened
  (`DMG_FILE_ICON`, an `.icns`). Without it, Finder shows the generic disk
  image icon.
- **Background**: the image behind the app and Applications icons in the
  window that opens on mount (`DMG_BACKGROUND`, a PNG sized to the window,
  540x380 by default). Without it, the window is plain.

The repo has no file icon or background yet. Either can be added later as a
file under `build/darwin/` and set as the default in the `create:dmg` task in
`build/darwin/Taskfile.yml`, or passed for a single build:

```bash
wails3 task darwin:package:dmg DMG_BACKGROUND=path/to/bg.png DMG_FILE_ICON=path/to/icon.icns
```

For a starting point, the Wails template ships `dmg-background.png` and
`dmg-file-icon.icns`: run `wails3 init -n scratch` in a temporary directory
and copy them from its `build/darwin/`, or take them from
`v3/internal/commands/build_assets/darwin/` in the
[Wails repository](https://github.com/wailsapp/wails).

## Scripts

| Command | Description |
|---------|-------------|
| `npm run install-all` | Install frontend dependencies |
| `npm run dev` | `wails3 dev`: run with hot reload |
| `npm run build` | `wails3 build`: build the binary into `bin/` |
| `npm run package` | `wails3 package`: build the app bundle or installer |
| `npm test` | Run the Go unit tests |
| `npm run test:smoke` | Run the live smoke test |
