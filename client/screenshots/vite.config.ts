// Vite config for the screenshot harness: the app's own config plus the mock
// backend. Used by screenshots/capture.mjs; not part of the app build.

import { defineConfig, mergeConfig } from 'vite';
import base from '../vite.config';
import mockBackend from './mock-backend';

export default mergeConfig(base, defineConfig({
  root: new URL('..', import.meta.url).pathname,
  plugins: [mockBackend()],
  server: { port: 9246, strictPort: false, host: '127.0.0.1' },
  logLevel: 'warn',
}));
