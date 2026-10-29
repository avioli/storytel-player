// Settings and what opens from it: language picker, logs, logout.

import { LOGGED_IN, openSettings, re } from './common.mjs';

export default [
  { name: 'settings', store: LOGGED_IN, steps: async ({ page, t }) => openSettings(page, t) },
  {
    name: 'language',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await page.getByTitle(re(t('language.change').split('(')[0].trim())).first().click();
      await page.getByText(t('language.subtitle')).waitFor();
    },
  },
  {
    name: 'logs',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await openSettings(page, t);
      await page.getByText(t('settings.viewLogs')).click();
      await page.getByPlaceholder(t('logs.search')).waitFor();
    },
  },
  {
    name: 'logout',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await openSettings(page, t);
      await page.getByRole('button', { name: t('settings.logout') }).click();
      await page.getByText(t('logout.confirmMessage')).waitFor();
    },
  },
];
