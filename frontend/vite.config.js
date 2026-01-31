import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  server: {
    // host: '0.0.0.0',
    allowedHosts: ['app.ommprakash.cloud', 'localhost', '127.0.0.1'],
    hmr: {
      // Force the client to connect via standard HTTPS port
      clientPort: 443,
      // Optional: Force the host if auto-detection fails
      // host: "app.example.com"
    }
  }
})
