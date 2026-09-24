import { defineConfig } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  baseURL,
  password,
  port,
  publisherPort,
  publisherURL,
} from './e2e/env';

// Seam 3: the browser, driving the real binary — the same one a self-hoster
// runs, serving the embedded SPA — over a throwaway database, against a fake
// publisher on loopback.
const dataDir = join(tmpdir(), `reader-e2e-${process.pid}-${Date.now()}`);

export default defineConfig({
  testDir: 'e2e',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? 'list' : [['list']],
  use: { baseURL },
  webServer: [
    {
      command: 'node e2e/publisher.mjs',
      url: `${publisherURL}/feed.xml`,
      reuseExistingServer: false,
      env: { PUBLISHER_PORT: String(publisherPort) },
    },
    {
      command: '../bin/yogurt',
      url: baseURL,
      reuseExistingServer: false,
      env: {
        YOGURT_ADDR: `127.0.0.1:${port}`,
        YOGURT_DATA_DIR: dataDir,
        YOGURT_PASSWORD: password,
        // The fake publisher is on loopback, which the app otherwise
        // refuses to fetch.
        YOGURT_ALLOW_PRIVATE_FETCH: 'true',
      },
    },
  ],
});
