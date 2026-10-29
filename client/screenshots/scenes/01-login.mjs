// Signed-out screens: the login form, and the error after a failed sign-in.

export default [
  { name: 'login', store: {}, steps: async () => {} },
  {
    name: 'login-error',
    store: {},
    fail: ['/api/login'],
    steps: async ({ page, t }) => {
      await page.getByPlaceholder(t('login.email')).fill('reader@example.com');
      await page.getByPlaceholder(t('login.password')).fill('wrong password');
      await page.getByRole('button', { name: t('login.signIn') }).click();
    },
  },
];
