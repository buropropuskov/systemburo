// Листинг 3.1 — Конфигурация Vitest с псевдонимами путей
// Файл: vite.config.js (фронтенд-репозиторий dokkee-frontend)
// Обеспечивает:
//   • алиасы путей @/components/* → src/components/*
//   • среду jsdom для DOM-API в тестах Vue
//   • отчёты покрытия text/html/lcov
//   • стабильный setup-файл для global hooks

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './tests/setup.js',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov'],
      include: ['src/**/*.{js,vue}'],
      exclude: ['src/main.js', 'src/router/index.js', 'src/assets/**'],
    },
    reporters: ['default', 'junit'],
    outputFile: { junit: './test-results/junit.xml' },
  },
})
