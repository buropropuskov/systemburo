import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.js'

export default mergeConfig(viteConfig, defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.spec.js', 'src/**/*.test.js'],
    // Потоки вместо процессов: окружение поднимается заново на каждый файл, а их
    // за прогон больше трёхсот, и поток обходится дешевле форка.
    //
    // Изоляцию файлов при этом оставляем. Без неё прогон вдвое быстрее, но девять
    // десятков тестов начинают падать - они ловят состояние, оставленное соседним
    // файлом. Сначала эти тесты, потом изоляция.
    pool: 'threads',
  },
}))
