import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';

const target = process.env.ATTESTARA_WEB_TARGET ?? 'local';
if (!['local', 'github-pages'].includes(target)) throw new Error('Unsupported web build target');

export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  base: target === 'github-pages' ? '/Attestara/' : '/',
  define: { __ATTESTARA_STATIC_PAGES__: JSON.stringify(target === 'github-pages') },
  server: { host: '127.0.0.1', port: 5173, strictPort: true },
  build: {
    outDir: fileURLToPath(new URL('../../dist/web', import.meta.url)),
    emptyOutDir: true,
    sourcemap: false,
    target: 'es2022',
  },
});
