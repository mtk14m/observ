import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    // 127.0.0.1, not localhost: Node may resolve localhost to IPv6 (::1),
    // which Docker runtimes such as Colima do not forward.
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test/setup.ts'],
  },
})
