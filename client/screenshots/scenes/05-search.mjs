// Catalog search: the empty page, results with filters and a preview, and
// no results.

import { LOGGED_IN } from './common.mjs';

const search = async (page, t, query) => {
  const box = page.getByPlaceholder(t('search.placeholder')).first();
  await box.click();
  if (query) {
    await box.fill(query);
    await box.press('Enter');
  }
};

export default [
  {
    name: 'search',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await search(page, t);
      await page.getByText(t('search.initialPrompt')).waitFor();
    },
  },
  {
    name: 'search-results',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await search(page, t, 'the');
      await page.getByText('Small Fires').first().waitFor();
    },
  },
  {
    name: 'search-preview',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await search(page, t, 'the');
      await page.getByText('Small Fires').first().click();
    },
  },
  {
    name: 'search-no-results',
    store: LOGGED_IN,
    steps: async ({ page, t }) => {
      await search(page, t, 'zzz');
      await page.getByText(t('search.noResultsHint')).waitFor();
    },
  },
];
