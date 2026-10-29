// Shared setup and navigation helpers for the scene files.

/** Logged in, welcome modal already dismissed. */
export const LOGGED_IN = { token: 'screenshot-token', hasSeenWelcome: 'true' };

/** Logged in with a book loaded in the player (see ACTIVE_BOOK_ID in fixtures.ts). */
export const PLAYING = { store: LOGGED_IN, session: '9100001' };

export const ACTIVE_TITLE = 'The Lighthouse Keeper';

/** Escapes text for use inside a RegExp. */
export const re = (text) => new RegExp('^' + text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));

/** Opens the full player view from the player bar. */
export async function openPlayer(page, t) {
  await page.getByTitle(t('player.expand')).click();
  await page.locator('#bookmark-btn').waitFor();
}

/** Opens the settings modal from the top bar. */
export async function openSettings(page, t) {
  await page.getByTitle(t('settings.title')).first().click();
  await page.getByText(t('settings.appearance')).waitFor();
}
