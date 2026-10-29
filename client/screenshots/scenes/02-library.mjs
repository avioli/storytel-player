// The library (dashboard): first-run welcome, the book grid with its filters,
// the player bar, removing a book, and the empty and error states.

import { LOGGED_IN, PLAYING, ACTIVE_TITLE } from './common.mjs';

export default [
  { name: 'welcome', store: { token: 'screenshot-token' }, steps: async () => {} },
  { name: 'library', store: LOGGED_IN, steps: async () => {} },
  { name: 'library-player-bar', ...PLAYING, steps: async () => {} },
  {
    name: 'library-no-match',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await page.getByPlaceholder(t('dashboard.search')).fill('zzz');
    },
  },
  { name: 'library-empty', store: LOGGED_IN, emptyLibrary: true, steps: async () => {} },
  {
    name: 'remove-book',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      const card = page.getByText(ACTIVE_TITLE, { exact: true }).first();
      await card.hover();
      await page.getByLabel(t('bookshelf.remove')).first().click({ force: true });
      await page.getByText(t('bookshelf.removeTitle')).waitFor();
    },
  },
  { name: 'library-error', store: LOGGED_IN, fail: ['/api/bookshelf'], steps: async () => {} },
];
