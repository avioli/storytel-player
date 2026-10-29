// The player: the bar's popovers, the full player view and its modals.

import { PLAYING, openPlayer } from './common.mjs';

export default [
  {
    name: 'bar-chapters',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await page.getByTitle(t('player.chapters')).click();
    },
  },
  {
    name: 'bar-sleep-timer',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await page.getByTitle(t('player.sleepTimer')).click();
    },
  },
  {
    name: 'bar-speed',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await page.getByTitle(t('player.speed')).click();
    },
  },
  { name: 'player', ...PLAYING, steps: async ({ page, t }) => openPlayer(page, t) },
  {
    name: 'chapters',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#bookmark-btn').locator('xpath=preceding-sibling::button[1]').click();
      await page.getByText(t('chapters.title')).waitFor();
    },
  },
  {
    name: 'bookmarks',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#bookmark-btn').click();
      await page.getByText(t('bookmarks.title')).waitFor();
    },
  },
  {
    name: 'bookmark-create',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#bookmark-btn').click();
      await page.getByRole('button', { name: t('bookmarks.createNew') }).click();
      await page.getByPlaceholder(t('bookmarks.notePlaceholderCreate')).waitFor();
    },
  },
  {
    name: 'bookmark-edit',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#bookmark-btn').click();
      await page.getByTitle(t('bookmarks.tooltipEdit')).first().click();
      await page.getByPlaceholder(t('bookmarks.notePlaceholderEdit')).waitFor();
    },
  },
  {
    name: 'bookmark-delete',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#bookmark-btn').click();
      await page.getByTitle(t('bookmarks.tooltipDelete')).first().click();
      await page.getByText(t('bookmarks.deleteConfirm')).waitFor();
    },
  },
  {
    name: 'goto',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.getByRole('button', { name: t('gotoModal.go'), exact: true }).click();
      await page.getByText(t('gotoModal.title')).first().waitFor();
    },
  },
  {
    name: 'playback-speed',
    ...PLAYING,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.getByRole('button', { name: /^\d+(\.\d+)?x$/ }).click();
      await page.getByText(t('playbackSpeed.presets')).waitFor();
    },
  },
  {
    name: 'download-delete',
    ...PLAYING,
    downloaded: ['9100001'],
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#download-btn').click();
      await page.getByText(t('download.deleteMessage')).waitFor();
    },
  },
  {
    name: 'download-cancel',
    ...PLAYING,
    hangDownloads: true,
    steps: async ({ page, t }) => {
      await openPlayer(page, t);
      await page.locator('#download-btn').click(); // starts the download
      await page.locator('#download-btn.bg-red-600').waitFor();
      await page.locator('#download-btn').click(); // asks to cancel it
      await page.getByText(t('download.cancelMessage')).waitFor();
    },
  },
];
