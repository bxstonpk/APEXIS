import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// unit / component test (Vitest + Testing Library) — แยกจาก build ของ Next
// ค่าเริ่มต้น jsdom; เทสต์ฝั่ง server (*.server.test.ts) ระบุ `// @vitest-environment node` ที่หัวไฟล์
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})
