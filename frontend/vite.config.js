import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import permissionsKeysPlugin from './build/vite-plugin-permissions.js'

/** Иконки, темизуемые по имени файла: см. src/assets/icon-theme.css. */
const ICON_ASSET_RE = /[\\/]src[\\/]assets[\\/]icons[\\/][^\\/]+$/

export default defineConfig({
  plugins: [vue(), permissionsKeysPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    /*
     * Иконки из src/assets/icons отдаём отдельными файлами. Тёмные темы
     * осветляют однотонные PNG правилами вида img[src*="/edit-"]
     * (src/assets/icon-theme.css), а инлайн в data:-URI стирает имя файла из
     * src - селектор промахивается, и иконка остаётся чёрной на чёрном фоне.
     * Все они мельче стандартного лимита 4 КБ, поэтому в сборке инлайнились
     * все до единой, и правила темы не действовали (в dev действовали - там
     * файл отдаётся по исходному имени; отсюда «на моей машине работает»).
     * undefined для остальных путей = штатное правило по размеру.
     */
    assetsInlineLimit: (filePath) => (ICON_ASSET_RE.test(filePath) ? false : undefined),
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
