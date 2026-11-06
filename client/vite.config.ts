import { defineConfig } from 'vite';
import { mapEditorApi } from './vite-plugins/mapEditorApi';

export default defineConfig({
  root: '.',
  base: '/',
  // Dev-only API for map-editor.html (docs/map-editor.md)
  plugins: [mapEditorApi()],
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
