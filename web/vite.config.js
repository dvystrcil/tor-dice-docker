import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Static SPA build. Output to web/dist/ which the Go binary embeds via
// //go:embed. No SSR — the Go server only serves the static files plus
// the JSON REST endpoints at /api/*.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    target: 'es2020',
  },
  server: {
    // For local dev only. Production runs from embedded files.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
