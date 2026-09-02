import { copyFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig, type Plugin } from 'vite';

function copyUnrarWasm(): Plugin {
  return {
    name: 'copy-unrar-wasm',
    buildStart() {
      copyFileSync(
        fileURLToPath(new URL('./node_modules/node-unrar-js/esm/js/unrar.wasm', import.meta.url)),
        fileURLToPath(new URL('./public/unrar.wasm', import.meta.url)),
      );
    },
  };
}

export default defineConfig({
  plugins: [copyUnrarWasm(), react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('.', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    manifest: true,
  },
  server: {
    host: '127.0.0.1',
    port: 3000,
    strictPort: true,
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/healthz': 'http://127.0.0.1:8080',
    },
  },
});
