/// <reference types="vitest/config" />
import path from 'node:path';

import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
      routesDirectory: './src/app/routes',
      generatedRouteTree: './src/routeTree.gen.ts',
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, 'src') },
  },
  server: {
    port: 5173,
    // The Host header is preserved so that the API resolves the tenant from
    // the subdomain (kazutb.localhost:5173).
    proxy: {
      '/api': {
        target: process.env.QADAM_API_PROXY ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
  build: {
    target: 'es2023',
    // No source maps in the published bundle (DECISIONS: source maps); a
    // debugging build gets them without a reference in the code.
    sourcemap: process.env.QADAM_SOURCEMAP === 'hidden' ? 'hidden' : false,
    // Fonts stay files: the CSP allows fonts only from our origin (font-src
    // 'self'), so a small subset inlined as a data: URL would be blocked.
    assetsInlineLimit: (file) => (file.endsWith('.woff2') ? false : undefined),
    rolldownOptions: {
      output: {
        codeSplitting: {
          // Every icon was its own chunk (45 requests on the landing): one file instead.
          groups: [{ name: 'icons', test: /[\\/]node_modules[\\/]lucide-react[\\/]/ }],
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: false,
    restoreMocks: true,
  },
});
