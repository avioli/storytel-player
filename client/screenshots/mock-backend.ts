// Vite plugin that stands in for the Go backend while taking screenshots:
// it answers the /api/* and /native/* routes the app calls (see server/api.go
// and server/main.go) from the fixtures, and keeps a little in-memory state.
// Translations are read from server/locales, so they are always the real ones.
//
// Control routes for the capture script:
//   POST /__mock/reset  start a scenario; body (all optional):
//     language    UI language mode ('auto' or a code)
//     store       native store contents (token, hasSeenWelcome, ...)
//     fail        API route prefixes that answer 500, e.g. ["/api/bookshelf"]
//     session     book id restored as the last played book (shows the player bar)
//     emptyLibrary   true for a bookshelf with no books
//     downloaded  book ids that count as downloaded
//     hangDownloads  true to keep POST /api/download open (download in progress)
//   GET  /__mock/cover/<id>.svg, /__mock/silence.wav

import { readFileSync, readdirSync } from 'fs';
import { resolve } from 'path';
import type { IncomingMessage, ServerResponse } from 'http';
import type { Plugin } from 'vite';
import {
  BOOKMARKS, BOOKS, bookDetails, bookMetadata, bookshelf, coverSvg, logs, positional,
  searchResults, silenceWav,
} from './fixtures';

const LOCALES = resolve(__dirname, '../../server/locales');
const languages = () => readdirSync(LOCALES).filter((f) => f.endsWith('.json')).map((f) => f.slice(0, -5));

interface State {
  language: string; // 'auto' or a language code
  store: Record<string, unknown>;
  fail: string[]; // route prefixes that answer with an error, e.g. "/api/bookshelf"
  removed: Set<string>;
  downloaded: string[];
  hangDownloads: boolean;
  bookmarks: typeof BOOKMARKS;
  logs: ReturnType<typeof logs>;
}

const fresh = (): State => ({
  language: 'en', store: {}, fail: [], removed: new Set(), downloaded: [], hangDownloads: false,
  bookmarks: BOOKMARKS.map((b) => ({ ...b })), logs: logs(),
});

let state = fresh();

const send = (res: ServerResponse, status: number, body: unknown, type = 'application/json') => {
  res.statusCode = status;
  res.setHeader('Content-Type', type);
  res.end(typeof body === 'string' || Buffer.isBuffer(body) ? body : JSON.stringify(body));
};

const readJSON = (req: IncomingMessage): Promise<any> =>
  new Promise((ok) => {
    let raw = '';
    req.on('data', (c) => (raw += c));
    req.on('end', () => {
      try { ok(raw ? JSON.parse(raw) : {}); } catch { ok({}); }
    });
  });

const localeState = () => {
  const resolved = state.language === 'auto' ? 'en' : state.language;
  return { mode: state.language, resolved, systemLanguages: ['en-GB'], systemResolved: 'en', region: 'GB' };
};

async function handle(req: IncomingMessage, res: ServerResponse): Promise<boolean> {
  const url = new URL(req.url || '/', 'http://mock');
  const path = url.pathname;
  const method = req.method || 'GET';
  const origin = `http://${req.headers.host}`;
  const seg = path.split('/').filter(Boolean);

  if (path.startsWith('/__mock/')) {
    if (path === '/__mock/reset' && method === 'POST') {
      const b = await readJSON(req);
      state = fresh();
      state.language = b.language || 'en';
      state.store = b.store || {};
      state.fail = b.fail || [];
      state.downloaded = b.downloaded || [];
      state.hangDownloads = !!b.hangDownloads;
      if (b.emptyLibrary) BOOKS.forEach((book) => state.removed.add(book.id));
      if (b.session) {
        const book = bookshelf(origin, new Set()).books.find((e) => e.id === b.session);
        state.store.last_played_session = JSON.stringify({ book, bookId: b.session, consumableId: b.session });
      }
      return send(res, 200, { ok: true }), true;
    }
    if (seg[1] === 'cover') {
      const svg = coverSvg(seg[2].replace(/\.svg$/, ''));
      return svg ? send(res, 200, svg, 'image/svg+xml') : send(res, 404, ''), true;
    }
    if (path === '/__mock/silence.wav') return send(res, 200, silenceWav(), 'audio/wav'), true;
    return false;
  }

  if (path.startsWith('/native/')) {
    const b = method === 'POST' ? await readJSON(req) : {};
    switch (`${method} ${path}`) {
      case 'POST /native/store/get': return send(res, 200, { value: state.store[b.key] ?? null }), true;
      case 'POST /native/store/set': state.store[b.key] = b.value; return send(res, 200, {}), true;
      case 'POST /native/store/remove': delete state.store[b.key]; return send(res, 200, {}), true;
      case 'GET /native/locale': return send(res, 200, localeState().resolved), true;
      case 'GET /native/locale/state': return send(res, 200, localeState()), true;
      case 'POST /native/locale': state.language = b.locale; return send(res, 200, localeState()), true;
      case 'GET /native/window/always-on-top': return send(res, 200, false), true;
      default: return send(res, 200, {}), true; // tray, window and logs-folder calls
    }
  }

  if (!path.startsWith('/api/')) return false;

  if (state.fail.some((p) => path.startsWith(p))) {
    return send(res, 500, { error: 'Simulated failure' }), true;
  }

  const id = seg[2];
  const b = method === 'GET' || method === 'DELETE' ? {} : await readJSON(req);
  const route = `${method} /${seg.slice(0, 2).join('/')}`;

  switch (route) {
    case 'GET /api/translations': {
      const lang = url.searchParams.get('lang') || 'en';
      const code = languages().includes(lang) ? lang : 'en';
      res.setHeader('Content-Language', code);
      return send(res, 200, readFileSync(resolve(LOCALES, `${code}.json`), 'utf-8')), true;
    }
    case 'GET /api/auth': // /api/auth/status
      return send(res, 200, { authenticated: !!req.headers.authorization?.replace('Bearer ', '') }), true;
    case 'POST /api/login':
      if (!b.email || !b.password) return send(res, 400, { error: 'Email and password required' }), true;
      return send(res, 200, { success: true, token: 'screenshot-token' }), true;
    case 'POST /api/logout': return send(res, 200, { success: true }), true;
    case 'GET /api/account': return send(res, 200, { email: 'reader@example.com' }), true;
    case 'GET /api/bookshelf': return send(res, 200, bookshelf(origin, state.removed)), true;
    case 'GET /api/search': return send(res, 200, searchResults(origin, url.searchParams.get('q') || '')), true;
    case 'POST /api/bookshelf': {
      if (seg[2] === 'remove') state.removed.add(String(b.consumableId));
      return send(res, 200, { success: true, result: { added: true, removed: true } }), true;
    }
    case 'POST /api/stream': return send(res, 200, { streamUrl: `${origin}/__mock/silence.wav`, remote: true }), true;
    case 'GET /api/book-details': {
      const d = bookDetails(origin, id);
      return d ? send(res, 200, d) : send(res, 404, { error: 'Not found' }), true;
    }
    case 'GET /api/bookmetadata': {
      const d = bookMetadata(origin, id);
      return d ? send(res, 200, d) : send(res, 404, { error: 'Not found' }), true;
    }
    case 'GET /api/bookmarks': return send(res, 200, { bookmarks: state.bookmarks }), true;
    case 'POST /api/bookmarks':
      state.bookmarks.push({ id: Date.now(), position: Number(b.position) || 0, note: b.note || '', insertTime: new Date().toISOString() });
      return send(res, 200, { success: true }), true;
    case 'PUT /api/bookmarks':
    case 'DELETE /api/bookmarks': return send(res, 200, { success: true }), true;
    case 'GET /api/bookmark-positional': return send(res, 200, id ? positional(id) : []), true;
    case 'PUT /api/bookmark-positional': return send(res, 200, positional(id)), true;
    case 'GET /api/download-status': return send(res, 200, { downloaded: state.downloaded.includes(id) }), true;
    case 'POST /api/download':
      if (state.hangDownloads) return true; // never answers: stays "in progress"
      return send(res, 200, { success: true }), true;
    case 'GET /api/logs': return send(res, 200, state.logs), true;
    case 'DELETE /api/logs': state.logs = []; return send(res, 200, { success: true }), true;
    case 'POST /api/logs': return send(res, 200, { success: true }), true;
    default: return send(res, 404, { error: `No mock for ${method} ${path}` }), true;
  }
}

export default function mockBackend(): Plugin {
  return {
    name: 'storytel-mock-backend',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        handle(req, res).then((handled) => (handled ? undefined : next())).catch(next);
      });
    },
  };
}
