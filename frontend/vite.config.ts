import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import legacy from '@vitejs/plugin-legacy';

export default defineConfig({
  plugins: [
    react(),
    // Kiosk Raspberry Pis run a stale image stuck on Chromium
    // 78.0.3904.108, which predates optional chaining/nullish coalescing
    // (Chrome 80). renderModernChunks is off because plugin-legacy's
    // browser detection only checks for ESM/dynamic import/import.meta/
    // async generators, all supported since Chrome 63-64 -- it would
    // misdetect Chrome 78 as modern and serve the untranspiled bundle.
    legacy({
      targets: ['chrome >= 78'],
      renderModernChunks: false,
    }),
  ],
  build: {
    outDir: '../internal/http/frontend/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/admin': 'http://localhost:8080',
      '/static': 'http://localhost:8080',
      '/media': 'http://localhost:8080',
    },
  },
});
