import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'


// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
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
