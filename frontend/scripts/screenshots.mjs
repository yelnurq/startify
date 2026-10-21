// Captures the key screens in light and dark themes on desktop and mobile for
// visual review. Usage:
//   node scripts/screenshots.mjs [outDir] [baseURL] [filter]
// Defaults: test-results/screenshots, http://localhost:5173, every target.
import { mkdirSync } from 'node:fs';
import path from 'node:path';

import { chromium } from '@playwright/test';

const outDir = path.resolve(process.argv[2] ?? 'test-results/screenshots');
const baseURL = process.argv[3] ?? 'http://localhost:5173';
const filter = process.argv[4] ?? '';

const viewports = {
  desktop: { viewport: { width: 1440, height: 900 } },
  mobile: {
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 2,
    isMobile: true,
    hasTouch: true,
  },
};

/** Each target: a page, and optionally sections captured one by one. */
const targets = [
  { name: 'home', url: '/' },
  {
    name: 'design',
    url: '/design',
    sections: [
      'colors',
      'typography',
      'shape',
      'buttons',
      'forms',
      'identity',
      'cards',
      'overlays',
      'feedback',
      'navigation',
      'data',
      'empty',
      'layouts',
    ],
  },
  { name: 'shell', url: '/design/shell' },
  { name: 'workspace', url: '/design/workspace' },
  { name: 'auth', url: '/design/auth' },
];

mkdirSync(outDir, { recursive: true });
const browser = await chromium.launch();
let count = 0;
for (const theme of ['light', 'dark']) {
  for (const [device, options] of Object.entries(viewports)) {
    const context = await browser.newContext({
      ...options,
      colorScheme: theme,
      locale: 'ru-RU',
      timezoneId: 'Asia/Almaty',
      reducedMotion: 'reduce',
    });
    await context.addInitScript((t) => {
      localStorage.setItem('qadam.theme', t);
      localStorage.setItem('qadam.locale', 'ru');
    }, theme);
    const page = await context.newPage();
    for (const target of targets) {
      const id = `${target.name}-${device}-${theme}`;
      if (filter && !id.includes(filter)) continue;
      await page.goto(baseURL + target.url, { waitUntil: 'networkidle' });
      await page.evaluate(() => document.fonts.ready);
      if (target.sections) {
        await page.screenshot({ path: path.join(outDir, `${id}-hero.png`) });
        for (const section of target.sections) {
          const el = page.locator(`#${section}`);
          await el.scrollIntoViewIfNeeded();
          await page.waitForTimeout(150);
          await el.screenshot({ path: path.join(outDir, `${id}-${section}.png`) });
          count++;
        }
      } else {
        await page.screenshot({
          path: path.join(outDir, `${id}.png`),
          fullPage: device === 'mobile',
        });
        count++;
      }
    }
    await context.close();
  }
}
await browser.close();
console.log(`${count} screenshots in ${outDir}`);
