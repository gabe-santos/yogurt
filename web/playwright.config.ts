import { defineConfig } from '@playwright/test';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { baseURL, password, port } from './e2e/env';

// Seam 3: the browser, driving the real binary — the same one a self-hoster
// runs, serving the embedded SPA — over a throwaway database.
const dataDir = join(tmpdir(), `reader-e2e-${process.pid}-${Date.now()}`);

export default defineConfig({
	testDir: 'e2e',
	fullyParallel: false,
	workers: 1,
	forbidOnly: !!process.env.CI,
	reporter: process.env.CI ? 'list' : [['list']],
	use: { baseURL },
	webServer: {
		command: '../bin/reader',
		url: baseURL,
		reuseExistingServer: false,
		env: {
			READER_ADDR: `127.0.0.1:${port}`,
			READER_DATA_DIR: dataDir,
			READER_PASSWORD: password
		}
	}
});
