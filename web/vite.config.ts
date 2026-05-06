import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: Number(process.env.VITE_PORT) || 2019,
    proxy: {
      '/v1': {
        target: process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:8500',
        changeOrigin: true,
      }
    }
  }
})
