import { defineStore } from 'pinia'
import { getPublicContacts } from '@/api/settings'

/**
 * Контакты Бюро пропусков (телефон, почта) из системных настроек. Единый источник
 * для всех мест, где показываются контакты Бюро: страница логина, плашка блокировки
 * и т.д. Грузится один раз и кэшируется; пустые значения = "не настроено".
 */
export const useContactsStore = defineStore('contacts', {
  state: () => ({
    phone: '',
    email: '',
    loaded: false,
  }),
  getters: {
    hasAny: (state) => Boolean(state.phone || state.email),
  },
  actions: {
    async fetch(force = false) {
      if (this.loaded && !force) return
      try {
        const data = await getPublicContacts()
        this.phone = data?.phone || ''
        this.email = data?.email || ''
        this.loaded = true
      } catch {
        // Контакты опциональны (best-effort справочная информация): при сбое
        // публичного эндпоинта оставляем пустыми, не ломая логин/плашку.
      }
    },
  },
})
