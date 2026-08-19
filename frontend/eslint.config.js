// ESLint 10 flat config.
// В eslint-plugin-vue 10 конфиги экспортируются как массивы "flat/<preset>",
// которые уже содержат parser, globals и рекомендованные правила для Vue SFC.
// Для .js-файлов в проекте просто накидываем eslint:recommended + browser/node globals.

import js from '@eslint/js'
import vue from 'eslint-plugin-vue'
import globals from 'globals'

export default [
  {
    ignores: ['dist/', 'node_modules/', 'coverage/', 'playwright-report/', 'test-results/', 'e2e/.auth/']
  },
  js.configs.recommended,
  ...vue.configs['flat/recommended'],
  {
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: {
        ...globals.browser,
        ...globals.node
      }
    },
    rules: {
      // v-html используется только для admin-editable инструкций, sanitize
      // через DOMPurify в utils/sanitize.js. Правило не актуально для нас.
      'vue/no-v-html': 'off',
      // Отладочная печать в бандл не уезжает. error и warn пока разрешены:
      // их в коде больше трёхсот, это сообщения о реальных сбоях, и убирать их
      // надо вместе с введением логгера, а не запретом в линтере.
      'no-console': ['error', { allow: ['error', 'warn'] }]
    }
  },
  {
    // Тесты и служебные скрипты печатают в консоль по делу.
    files: ['**/__tests__/**', 'e2e/**', 'build/**', '*.config.js', '*.config.cjs'],
    rules: {
      'no-console': 'off'
    }
  }
]
