import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		proxy: {
			// Proxy API and WebSocket traffic to the Go backend so the browser
			// treats everything as same-origin (cookies + no CORS in dev).
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true,
				ws: true
			}
		}
	}
});
