import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// SPA mode: no server-side rendering, fall back to index.html for client routing.
		adapter: adapter({
			fallback: 'index.html',
			pages: 'build',
			assets: 'build'
		})
	}
};

export default config;
