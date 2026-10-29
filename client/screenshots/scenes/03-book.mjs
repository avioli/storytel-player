// The book details page, opened from the library.

import { LOGGED_IN, ACTIVE_TITLE } from './common.mjs';

export default [
  {
    name: 'book',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await page.getByText(ACTIVE_TITLE, { exact: true }).first().click();
      await page.getByText(t('bookView.details')).waitFor();
    },
  },
];
