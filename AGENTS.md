# Agent Development Guidelines for Storytel Player

## Project Overview

**Storytel Player** is an unofficial cross-platform desktop application for playing Storytel audiobooks, built with a React + TypeScript frontend and a Go backend on Wails 3.

### Tech Stack
- **Frontend**: React 18 + TypeScript + Tailwind CSS + Vite
- **Backend**: Go (stdlib `net/http`), served in-process
- **Desktop**: Wails 3 (system webview: WKWebView / WebView2 / WebKitGTK)
- **Internationalization**: i18next (multi-language support)
- **State Management**: JSON store in the user config dir (`server/store.go`)
- **Build Tools**: Vite, Go toolchain, Wails CLI (`Taskfile.yml` + `build/`)

### Architecture
```
storytel-player/
|-- client/          # React frontend application
|   |-- src/
|   |   |-- components/  # React components
|   |   |-- hooks/       # Custom React hooks
|   |   |-- services/    # API services
|   |   |-- utils/       # Utility functions
|   |   `-- i18n.ts      # i18next configuration
|   `-- package.json
|-- server/          # Go module: Wails desktop app + backend
|   |-- main.go          # Wails entry: window, tray, /native/* bridge
|   |-- api.go           # /api/* routes (auth, bookshelf, stream, downloads)
|   |-- storytel.go      # Storytel API client
|   |-- store.go         # Persistent JSON settings store
|   |-- i18n.go          # Language negotiation, tray translations
|   |-- logger.go        # app.log action/HTTP log
|   |-- locales/         # Translation files (embedded)
|   |-- assets/          # App and tray icons (embedded)
|   `-- frontend/        # Vite build output (generated, gitignored)
|-- build/           # Wails build config (config.yml) and per-OS packaging
|-- Taskfile.yml     # Wails tasks; FRONTEND_DIR/GO_DIR point at client/server
|-- scripts/         # release helpers
`-- package.json     # Root package.json (npm script shortcuts)
```

## Development Guidelines

### Language Requirements

#### Code and Documentation
- **ALL documentation files** (README, CONTRIBUTING, AGENTS.md, etc.) **MUST be written in English**
- **ALL code comments** MUST be written in English
- **ALL commit messages** MUST be written in English
- **ALL variable names, function names, and identifiers** MUST be in English

#### User-Facing Content
- **ALL user-facing text** (UI labels, messages, errors) MUST use the i18n translation system
- Never hardcode user-facing strings in components
- Always add new strings to translation files in `server/locales/`

#### Translation System
- **Location**: Translation files are in `server/locales/`
  - `server/locales/en.json` - English translations (default)
  - `server/locales/it.json` - Italian translations
  - Add more languages as needed

- **Usage in Frontend**:
  ```typescript
  import { useTranslation } from 'react-i18next';

  function MyComponent() {
    const { t } = useTranslation();
    return <div>{t('key.name')}</div>;
  }
  ```

- **Usage in Backend**:
  ```typescript
  // Server automatically provides translations via API
  // Use i18next instance configured in server
  ```

- **Adding New Translations**:
  1. Add the key to `server/locales/en.json`
  2. Add translations to all other language files (`it.json`, etc.)
  3. Use the key in your component with `t('your.new.key')`

- **Adding a New Language**: the supported-language list exists once per build
  target, because the renderer (TypeScript) and the backend (Go) do not share
  code. Update all three in lockstep:
  1. `server/locales/<code>.json` - full key parity with `en.json`
  2. `server/i18n.go` - add the code to `supportedLanguages`
  3. `client/src/config/languages.ts` - add code + endonym to `APP_LANGUAGES`

- **UI Language vs. Catalog Language**: they are unrelated and must not be
  conflated. The UI language is the user's own choice (or their OS locale);
  which books exist, and in which languages, is decided by the country of the
  logged-in Storytel account. Never derive one from the other - a `sv-FI`
  system locale means "Swedish speaker in Finland", not "Finnish catalog".
  Language *names* shown in the UI are produced locally with
  `Intl.DisplayNames` (see `localizedLanguageName` in `client/src/utils/helpers.ts`);
  the `languageName` the Storytel API returns is localized for the account's
  market, not for the app UI, so it is only a last-resort fallback.

### Code Style

- **TypeScript**: Strict mode enabled, use proper typing
- **React**: Functional components with hooks
- **CSS**: Tailwind CSS utility classes
- **Formatting**: Follow existing code style in the project

### Component Patterns

- Use existing modal components as reference (see `client/src/components/*Modal.tsx`)
- Follow existing hook patterns (see `client/src/hooks/`)
- Maintain consistent UI/UX with Tailwind classes already in use
- Reuse common components when possible

### State Management

- **Local State**: React hooks (useState, useEffect, etc.)
- **Persistent Storage**: Use `client/src/utils/storage.ts` (backed by `/native/store/*` in the desktop app)
- **API State**: Direct API calls via Axios (see `client/src/utils/api.ts`)

### Development Workflow

1. **Install dependencies**: `npm run install-all`, plus Go 1.26+ and the
   `wails3` CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`)
2. **Run with hot reload**: `wails3 dev` (Vite for `client/`, rebuild on `.go` changes)
3. **Build**: `wails3 build` (binary in `bin/`)
4. **Package**: `wails3 package` (outputs `bin/Storytel Player.app` on macOS)
5. **Tests**: `npm test` (Go unit tests), `npm run test:smoke` (live Storytel, needs a login)

### Testing Strategy

- Go unit tests (`npm test`) and a live smoke test (`npm run test:smoke`)
- Manual testing of the app
- Test across platforms (Windows, macOS, Linux)
- Verify i18n translations for all supported languages
- Test both online and offline modes

## Project-Specific Patterns

### Modal Components
- Follow the pattern in existing modal components
- Use proper state management with hooks
- Include proper TypeScript interfaces
- Ensure proper i18n integration

### Storage Keys
- Use descriptive names for storage keys
- Document storage schema in code comments
- Use TypeScript interfaces for stored data structures

### API Integration
- Backend API calls go through `client/src/utils/api.ts`
- Storytel API integration is in `server/storytel.go`; routes are in `server/api.go`
- Handle errors gracefully with user feedback

### Native Bridge
- Desktop-only calls (store, locale, window, tray, logs) are `/native/*` routes in `server/main.go`
- `client/src/platform/wails.ts` exposes them as the `window.electron*` / `window.trayControls`
  objects the components use (names kept from the Electron era; typed in `client/src/types/window.d.ts`)
- Go -> renderer events are DOM `CustomEvent`s named `native:<name>`
- Always handle both success and error cases

## Important Notes

- **Security**: This is a desktop app with persistent session storage
- **Authentication**: HS256 token (`server/api.go`) kept in the JSON store
- **Single Instance**: App uses single instance lock (one app instance at a time)
- **Updates**: no auto-updater yet (see MISSING.md)
- **Offline Mode**: Support for downloading audiobooks for offline playback

## Git Repository

- **Repository**: https://github.com/debba/storytel-player
- **Issues**: https://github.com/debba/storytel-player/issues
- **Releases**: https://github.com/debba/storytel-player/releases

## Discord Community

- Discord server for community discussions, feature requests, and support
- Link will be added once the welcome modal implementation is complete

## License

MIT License - see LICENSE file for details

---

**Remember**:
1. Documentation and code in English
2. User-facing content via i18n system
3. Follow existing patterns in the codebase
4. Test thoroughly before committing
