import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePermissionsStore } from '../permissions'
import { useAuthStore } from '../auth'

vi.mock('@/api/permissions', () => ({
  getMyPermissions: vi.fn(),
}))
import { getMyPermissions } from '@/api/permissions'

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }))
  return `${header}.${body}.fake-signature`
}

describe('permissions store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  describe('hasPermission', () => {
    it('returns true when permission value is allow', () => {
      const store = usePermissionsStore()
      store.permissions = { 'passes.create': 'allow' }

      expect(store.hasPermission('passes.create')).toBe(true)
    })

    it('returns false when permission value is deny', () => {
      const store = usePermissionsStore()
      store.permissions = { 'passes.create': 'deny' }

      expect(store.hasPermission('passes.create')).toBe(false)
    })

    it('returns false when permission key is missing', () => {
      const store = usePermissionsStore()
      store.permissions = {}

      expect(store.hasPermission('passes.create')).toBe(false)
    })

    it('returns true for admin regardless of permission value', () => {
      const authStore = useAuthStore()
      authStore.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }))

      const store = usePermissionsStore()
      store.permissions = { 'passes.create': 'deny' }

      expect(store.hasPermission('passes.create')).toBe(true)
    })

    it('returns true for admin even when permission is missing', () => {
      const authStore = useAuthStore()
      authStore.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }))

      const store = usePermissionsStore()
      store.permissions = {}

      expect(store.hasPermission('passes.create')).toBe(true)
    })
  })

  describe('clearPermissions', () => {
    it('resets permissions to empty object and loaded to false', () => {
      const store = usePermissionsStore()
      store.permissions = { 'passes.create': 'allow' }
      store.loaded = true

      store.clearPermissions()

      expect(store.permissions).toEqual({})
      expect(store.loaded).toBe(false)
    })
  })

  describe('fetchPermissions', () => {
    it('populates permissions from API response', async () => {
      getMyPermissions.mockResolvedValue([
        { key: 'passes.create', value: 'allow' },
        { key: 'passes.delete', value: 'deny' },
      ])

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.permissions).toEqual({
        'passes.create': 'allow',
        'passes.delete': 'deny',
      })
      expect(store.loaded).toBe(true)
    })

    it('sets loaded to true even with empty response', async () => {
      getMyPermissions.mockResolvedValue([])

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.permissions).toEqual({})
      expect(store.loaded).toBe(true)
    })

    it('keeps permissions empty on API error', async () => {
      getMyPermissions.mockRejectedValue(new Error('Network error'))

      const store = usePermissionsStore()
      store.permissions = { 'old.key': 'allow' }
      await store.fetchPermissions()

      expect(store.permissions).toEqual({ 'old.key': 'allow' })
      expect(store.loaded).toBe(false)
    })

    it('overwrites previous permissions on re-fetch', async () => {
      const store = usePermissionsStore()
      store.permissions = { 'old.key': 'allow' }

      getMyPermissions.mockResolvedValue([
        { key: 'new.key', value: 'allow' },
      ])

      await store.fetchPermissions()

      expect(store.permissions).toEqual({ 'new.key': 'allow' })
      expect(store.permissions).not.toHaveProperty('old.key')
    })
  })
})
