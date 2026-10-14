import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import permissionsKeysPlugin from './build/vite-plugin-permissions.js'

export default defineConfig({
  plugins: [vue(), permissionsKeysPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    rolldownOptions: {
      output: {
        // Только статические зависимости входа: страницы и тяжёлые функции
        // остаются отложенными. Не создаём десятки стартовых микрофайлов.
        codeSplitting: {
          groups: [{
            name: 'app-shell',
            tags: ['$initial'],
            // main содержит top-level await router.isReady(). Если включить его
            // в общий файл, lazy-страница начнёт ждать собственный bootstrap.
            test: (id) => {
              const file = id.replaceAll('\\', '/').split('?')[0];
              return !file.endsWith('/src/main.js') && !file.endsWith('/index.html');
            },
          }],
        },
      },
    },
  },
  server: {
    port: 8081,
    /*
     * Dev-proxy: в dev нет nginx, фронт и бэкенд - отдельные контейнеры/процессы.
     * Фронт видит бэкенд по имени сервиса docker (go-backend) либо на localhost
     * при `npm run dev` без docker. VITE_API_TARGET позволяет подменить target
     * из окружения (docker-compose ставит go-backend:8090, локально - default).
     * В prod nginx проксирует /api сам - этот блок не активируется.
     */
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8090',
        changeOrigin: true,
      },
      '/uploads': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8090',
        changeOrigin: true,
      },
    },
  },
})
