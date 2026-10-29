// Fictional library and catalog for the screenshot harness. The shapes follow
// what the Go backend returns (server/api.go, server/storytel.go); titles,
// people and covers are made up.

export interface FixtureBook {
  id: string;
  title: string;
  author: string;
  narrator: string;
  category: string;
  language: string; // ISO 639-1
  languageName: string; // as the Storytel API sends it (market language)
  hours: number;
  minutes: number;
  status: 1 | 2 | 3; // 1 not started, 2 started, 3 concluded
  progress: number; // 0..1 of the duration
  color: string; // cover background
  inLibrary: boolean;
  description: string;
}

export const BOOKS: FixtureBook[] = [
  {
    id: '9100001', title: 'The Lighthouse Keeper', author: 'Mara Ellison', narrator: 'Tom Hadley',
    category: 'Crime', language: 'en', languageName: 'English', hours: 9, minutes: 41,
    status: 2, progress: 0.38, color: '#1f4e79', inLibrary: true,
    description: 'A storm cuts a small island off from the mainland, and the keeper of its lighthouse finds a stranger on the rocks who remembers nothing but a name.',
  },
  {
    id: '9100002', title: 'Salt and Copper', author: 'Ines Varga', narrator: 'Ruth Calloway',
    category: 'Historical fiction', language: 'en', languageName: 'English', hours: 12, minutes: 5,
    status: 1, progress: 0, color: '#8a4b2a', inLibrary: true,
    description: 'Two families run rival foundries in a mining town at the turn of the century.',
  },
  {
    id: '9100003', title: 'Нощен влак за Варна', author: 'Петя Колева', narrator: 'Иван Стоянов',
    category: 'Romance', language: 'bg', languageName: 'Bulgarian', hours: 6, minutes: 43,
    status: 3, progress: 1, color: '#7a2e4d', inLibrary: true,
    description: 'Двама непознати споделят едно купе и една дълга нощ по пътя към морето.',
  },
  {
    id: '9100004', title: 'Under the Birches', author: 'Lena Holm', narrator: 'Anders Berg',
    category: 'Fantasy', language: 'sv', languageName: 'Swedish', hours: 8, minutes: 12,
    status: 2, progress: 0.72, color: '#2f6b4f', inLibrary: true,
    description: 'A girl follows a fox into the forest behind her grandmother\'s house and finds a village that is only there in winter.',
  },
  {
    id: '9100005', title: 'The Quiet Orbit', author: 'Dev Raman', narrator: 'Priya Nair',
    category: 'Science fiction', language: 'en', languageName: 'English', hours: 14, minutes: 30,
    status: 1, progress: 0, color: '#2b2d5c', inLibrary: true,
    description: 'The last crew of a research station has to decide who goes home on the only shuttle.',
  },
  {
    id: '9100006', title: 'Il giardino d\'inverno', author: 'Chiara Fontana', narrator: 'Marco Ricci',
    category: 'Literary fiction', language: 'it', languageName: 'Italian', hours: 7, minutes: 58,
    status: 3, progress: 1, color: '#5d6b2f', inLibrary: true,
    description: 'Un giardiniere e una vedova si scrivono lettere per un intero inverno.',
  },
  {
    id: '9100007', title: 'Small Fires', author: 'Nora Quinn', narrator: 'Ellie Shaw',
    category: 'Thriller', language: 'en', languageName: 'English', hours: 10, minutes: 2,
    status: 1, progress: 0, color: '#9c2f1f', inLibrary: false,
    description: 'A fire investigator returns to the town where her own house burned down twenty years ago.',
  },
  {
    id: '9100008', title: 'The Lighthouse at Dawn', author: 'Mara Ellison', narrator: 'Tom Hadley',
    category: 'Crime', language: 'en', languageName: 'English', hours: 11, minutes: 17,
    status: 1, progress: 0, color: '#33607a', inLibrary: false,
    description: 'The keeper is back, and this time the island has visitors who do not want to leave.',
  },
];

export const ACTIVE_BOOK_ID = '9100001';

const durationMs = (b: FixtureBook) => (b.hours * 3600 + b.minutes * 60) * 1000;

export const coverUrl = (origin: string, b: FixtureBook) => `${origin}/__mock/cover/${b.id}.svg`;

export function coverSvg(id: string): string | null {
  const b = BOOKS.find((x) => x.id === id);
  if (!b) return null;
  const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');
  const words = b.title.split(' ');
  const lines: string[] = [];
  for (const w of words) {
    const last = lines[lines.length - 1];
    if (last && (last + ' ' + w).length <= 12) lines[lines.length - 1] = last + ' ' + w;
    else lines.push(w);
  }
  const text = lines
    .map((l, i) => `<text x="40" y="${150 + i * 64}" font-size="54" font-weight="700">${esc(l)}</text>`)
    .join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="640" viewBox="0 0 640 640">
<rect width="640" height="640" fill="${b.color}"/>
<circle cx="520" cy="540" r="190" fill="#ffffff" opacity="0.08"/>
<g fill="#ffffff" font-family="Georgia, serif">${text}
<text x="40" y="590" font-size="30" opacity="0.85">${esc(b.author)}</text></g>
</svg>`;
}

export function bookshelf(origin: string, removed: Set<string>) {
  return {
    books: BOOKS.filter((b) => b.inLibrary && !removed.has(b.id)).map((b, i) => ({
      id: b.id,
      status: b.status,
      stateUpdateTime: new Date(Date.UTC(2026, 9, 20 - i)).toISOString(),
      positionUpdatedTime: new Date(Date.UTC(2026, 9, 20 - i)).toISOString(),
      abook: {
        id: String(Number(b.id) + 500000),
        description: b.description,
        narratorAsString: b.narrator,
        time: durationMs(b) * 1000, // microseconds, as Storytel sends it
      },
      abookMark: { pos: Math.round(durationMs(b) * b.progress) * 1000 },
      book: {
        consumableId: b.id,
        name: b.title,
        authorsAsString: b.author,
        category: { title: b.category },
        language: { localizedName: b.languageName, isoValue: b.language },
        largeCover: coverUrl(origin, b),
        largeCoverE: '',
      },
    })),
  };
}

export function searchResults(origin: string, q: string) {
  const needle = q.trim().toLowerCase();
  if (!needle || needle === 'zzz') return { results: [] };
  return {
    results: BOOKS.filter((b) =>
      [b.title, b.author, b.narrator].some((s) => s.toLowerCase().includes(needle)) ||
      needle === 'the').map((b) => ({
      id: b.id,
      title: b.title,
      authors: b.author,
      narrators: b.narrator,
      coverUrl: coverUrl(origin, b),
      category: b.category,
      durationMs: durationMs(b),
      description: b.description,
      hasAbook: true,
      hasEbook: b.id === '9100002',
      language: b.language,
      languageName: b.languageName,
    })),
  };
}

export function bookDetails(origin: string, id: string) {
  const b = BOOKS.find((x) => x.id === id);
  if (!b) return null;
  const ms = durationMs(b);
  return {
    consumableId: b.id,
    bookId: Number(b.id),
    title: b.title,
    originalTitle: '',
    description: b.description,
    language: b.language,
    authors: [{ id: '1' + b.id, name: b.author }],
    narrators: [{ id: '2' + b.id, name: b.narrator }],
    translators: [],
    category: { name: b.category },
    cover: { url: coverUrl(origin, b), width: 640, height: 640 },
    duration: { hours: Math.floor(ms / 3600000), minutes: Math.floor(ms / 60000) % 60, seconds: 0 },
    formats: [{
      id: String(Number(b.id) + 500000), type: 'abook', isReleased: true, isLockedContent: false,
      isbn: '978000' + b.id, publisher: { name: 'Lantern Audio' },
      releaseDate: '2025-03-01T00:00:00Z',
    }],
    ratings: { averageRating: 4.3, numberOfRatings: 1280 },
    seriesInfo: null,
    tags: [],
    kidsBook: false,
    isAbridged: false,
  };
}

const CHAPTER_TITLES = ['Prologue', 'The Storm', 'Low Tide', 'A Name on the Rocks', 'The Logbook', 'Signals', 'Epilogue'];

export function bookMetadata(origin: string, id: string) {
  const b = BOOKS.find((x) => x.id === id);
  if (!b) return null;
  const total = durationMs(b);
  const each = Math.floor(total / CHAPTER_TITLES.length);
  return {
    title: b.title,
    kidsBook: false,
    autoPlay: false,
    formats: [{
      type: 'abook',
      cover: { url: coverUrl(origin, b), width: 640, height: 640 },
      durationInMilliseconds: total,
      takedownDate: null,
      chapters: CHAPTER_TITLES.map((title, i) => ({
        number: i + 1,
        title,
        durationInMilliseconds: each,
        durationInSeconds: Math.round(each / 1000),
      })),
    }],
    sttMapping: { syncedReadingEnabled: false, mappingFileUrl: '' },
  };
}

// Positions are in seconds, as the app reads them.
export const BOOKMARKS = [
  { id: 1, position: 1234, note: 'The stranger says the name for the first time', insertTime: '2026-10-12T19:04:00Z' },
  { id: 2, position: 5050, note: '', insertTime: '2026-10-14T21:30:00Z' },
  { id: 3, position: 9381, note: 'Read this part again', insertTime: '2026-10-18T07:45:00Z' },
];

export function positional(id: string) {
  const b = BOOKS.find((x) => x.id === id);
  if (!b) return [];
  return [{
    consumableId: b.id, type: 'abook', kidsMode: false,
    position: Math.round(durationMs(b) * b.progress), updatedTime: '2026-10-20T08:00:00Z',
  }];
}

export function logs() {
  const at = (m: number) => new Date(Date.UTC(2026, 9, 28, 9, m)).toISOString();
  return [
    { id: 'l5', timestamp: at(12), type: 'action', message: 'Playback started', data: { bookId: ACTIVE_BOOK_ID } },
    { id: 'l4', timestamp: at(11), type: 'http', message: 'GET /api/bookmetadata/9100001', method: 'GET', url: '/api/bookmetadata/9100001', status: 200 },
    { id: 'l3', timestamp: at(10), type: 'error', message: 'Failed to refresh token', data: { status: 401 } },
    { id: 'l2', timestamp: at(9), type: 'http', message: 'GET /api/bookshelf', method: 'GET', url: '/api/bookshelf', status: 200 },
    { id: 'l1', timestamp: at(8), type: 'action', message: 'Login', data: { email: 'reader@example.com' } },
  ];
}

/** 30 seconds of 8 kHz, 8-bit mono silence, so the audio element can load. */
export function silenceWav(): Buffer {
  const rate = 8000;
  const samples = rate * 30;
  const buf = Buffer.alloc(44 + samples, 0x80);
  buf.write('RIFF', 0);
  buf.writeUInt32LE(36 + samples, 4);
  buf.write('WAVEfmt ', 8);
  buf.writeUInt32LE(16, 16);
  buf.writeUInt16LE(1, 20); // PCM
  buf.writeUInt16LE(1, 22); // mono
  buf.writeUInt32LE(rate, 24);
  buf.writeUInt32LE(rate, 28);
  buf.writeUInt16LE(1, 32);
  buf.writeUInt16LE(8, 34);
  buf.write('data', 36);
  buf.writeUInt32LE(samples, 40);
  return buf;
}
