import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// The build output is embedded into the Go binary, so it lands inside the Go
// tree rather than in web/build. Per ADR-0006 this is a static SPA: no
// server-side rendering, no framework server.
const embedTarget = '../internal/webui/dist/spa';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({
				pages: embedTarget,
				assets: embedTarget,
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		})
	],
	server: {
		// `pnpm dev` serves the SPA; the API still comes from the Go binary.
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: false
			}
		}
	}
});
