import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { getMyPermissions } from '@/api/permissions'

export const usePermissionsStore = defineStore('permissions', {
  state: () => ({
    permissions: {},
    loaded: false,
  }),

  getters: {
    hasPermission: (state) => (key) => {
      const auth = useAuthStore()
      if (auth.isAdmin) return true
      return state.permissions[key] === 'allow'
    },
  },

  actions: {
    async fetchPermissions() {
      try {
        const data = await getMyPermissions()
        this.permissions = {}
        if (Array.isArray(data)) {
          data.forEach(p => { this.permissions[p.key] = p.value })
        }
        this.loaded = true
      } catch (e) {
        // ignore — permissions will stay empty
      }
    },

    clearPermissions() {
      this.permissions = {}
      this.loaded = false
    },
  },
})
