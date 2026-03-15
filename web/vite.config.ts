import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 2019,
    proxy: {
      '/v1': {
        target: 'http://127.0.0.1:8500',
        changeOrigin: true,
      }
    }
  }
})
