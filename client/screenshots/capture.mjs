// Screenshot harness: runs the app in Chromium against the mock backend
// (screenshots/mock-backend.ts) and captures each screen in each language.
//
//   npm run screenshots                       all languages, all scene files
//   npm run screenshots -- bg en              only these languages
//   npm run screenshots -- --scenes=player    only scene files matching a name
//
// Scenes live in screenshots/scenes/<nn>-<area>.mjs, one file per area of the
// app. Each exports an array of scenes:
//
//   { name, steps, ...options }
//
// `steps({ page, t })` drives the UI from a freshly loaded app. Buttons are
// found by their translated text via t(), so a scene works in every language.
// The other options set up the mock backend first; see the reset route in
// mock-backend.ts (store, fail, session, emptyLibrary, downloaded, ...).
//
// Output: screenshots/out/<lang>/<area>/<nn>-<scene>.png (ignored by git).

import { mkdirSync, readFileSync, readdirSync, rmSync } from 'fs';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { createServer } from 'vite';
import { chromium } from 'playwright';

const here = dirname(fileURLToPath(import.meta.url));
const LOCALES = resolve(here, '../../server/locales');
const OUT = resolve(here, 'out');
const VIEWPORT = { width: 1280, height: 800 }; // the app's default window size

const args = process.argv.slice(2);
const only = args.find((a) => a.startsWith('--scenes='))?.slice(9);
const allLanguages = readdirSync(LOCALES).filter((f) => f.endsWith('.json')).map((f) => f.slice(0, -5)).sort();
const languages = args.filter((a) => !a.startsWith('--'));
const targets = languages.length ? languages : allLanguages;

const sceneFiles = readdirSync(resolve(here, 'scenes'))
  .filter((f) => /^\d+-.+\.mjs$/.test(f) && (!only || f.includes(only)))
  .sort();
const areas = [];
for (const f of sceneFiles) {
  areas.push({ area: f.replace(/\.mjs$/, ''), scenes: (await import(resolve(here, 'scenes', f))).default });
}

function translator(lang) {
  const dict = JSON.parse(readFileSync(resolve(LOCALES, `${lang}.json`), 'utf-8'));
  return (key, vars = {}) => {
    const value = key.split('.').reduce((o, k) => o?.[k], dict);
    if (typeof value !== 'string') throw new Error(`${lang}: missing key ${key}`);
    return value.replace(/{{(\w+)}}/g, (_, k) => String(vars[k] ?? ''));
  };
}

// Waits for requests to finish, but not forever: a scene may hold one open
// on purpose (a download in progress).
const settle = (page) => page.waitForLoadState('networkidle', { timeout: 3000 }).catch(() => {});

async function capture(base, browser, lang, t, area, index, scene) {
  const dir = resolve(OUT, lang, area);
  const file = resolve(dir, `${String(index + 1).padStart(2, '0')}-${scene.name}.png`);
  const { name, steps, ...options } = scene;
  await fetch(new URL('/__mock/reset', base), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...options, language: lang }),
  });
  const context = await browser.newContext({ viewport: VIEWPORT, locale: 'en-GB', reducedMotion: 'reduce' });
  const page = await context.newPage();
  page.setDefaultTimeout(5000);
  try {
    await page.goto(base);
    await settle(page);
    await steps({ page, t });
    await settle(page);
    await page.waitForTimeout(300); // let transitions finish
    await page.screenshot({ path: file });
    console.log(`${lang} ${area}/${name}`);
    return true;
  } catch (err) {
    await page.screenshot({ path: file.replace(/\.png$/, '.FAILED.png') }).catch(() => {});
    console.error(`${lang} ${area}/${name}: ${err.message.split('\n')[0]}`);
    return false;
  } finally {
    await context.close();
  }
}

// Vite 7 exits when stdin closes unless CI is set (it takes that as the parent
// process going away), which kills runs from scripts and editors.
process.env.CI ??= 'true';
const server = await createServer({ configFile: resolve(here, 'vite.config.ts') });
await server.listen();
const base = server.resolvedUrls.local[0];
const browser = await chromium.launch();
let failures = 0;

try {
  for (const lang of targets) {
    const t = translator(lang);
    for (const { area, scenes } of areas) {
      rmSync(resolve(OUT, lang, area), { recursive: true, force: true });
      mkdirSync(resolve(OUT, lang, area), { recursive: true });
      for (const [i, scene] of scenes.entries()) {
        if (!(await capture(base, browser, lang, t, area, i, scene))) failures++;
      }
    }
  }
} finally {
  await browser.close();
  await server.close();
}

console.log(`screenshots in ${OUT}${failures ? `, ${failures} failed` : ''}`);
process.exit(failures ? 1 : 0);
