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

  describe('hasPermission — mode: normal', () => {
    it('возвращает true если effective[key].value === allow', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = { 'passes.create': { value: 'allow', source: 'role' } }

      expect(store.hasPermission('passes.create')).toBe(true)
    })

    it('возвращает false если effective[key].value === deny', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = { 'passes.create': { value: 'deny', source: 'base' } }

      expect(store.hasPermission('passes.create')).toBe(false)
    })

    it('возвращает false если ключ отсутствует в effective', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = {}

      expect(store.hasPermission('passes.create')).toBe(false)
    })
  })

  describe('hasPermission — mode: super', () => {
    it('возвращает true для любого ключа', () => {
      const store = usePermissionsStore()
      store.mode = 'super'
      store.effective = {}

      expect(store.hasPermission('passes.create')).toBe(true)
      expect(store.hasPermission('page.admin.blacklist')).toBe(true)
    })

    it('возвращает true даже если ключ в effective deny', () => {
      const store = usePermissionsStore()
      store.mode = 'super'
      store.effective = { 'passes.create': { value: 'deny', source: 'base' } }

      expect(store.hasPermission('passes.create')).toBe(true)
    })
  })

  describe('hasPermission — mode: admin', () => {
    it('возвращает true если ключ не в denied', () => {
      const store = usePermissionsStore()
      store.mode = 'admin'
      store.denied = new Set()

      expect(store.hasPermission('passes.create')).toBe(true)
    })

    it('возвращает false если ключ в denied', () => {
      const store = usePermissionsStore()
      store.mode = 'admin'
      store.denied = new Set(['passes.create'])

      expect(store.hasPermission('passes.create')).toBe(false)
    })

    it('не зависит от effective при mode admin', () => {
      const store = usePermissionsStore()
      store.mode = 'admin'
      store.effective = { 'passes.create': { value: 'deny', source: 'base' } }
      store.denied = new Set()

      expect(store.hasPermission('passes.create')).toBe(true)
    })
  })

  describe('hasPermission — mode: admin, super-only ключи (#1997)', () => {
    // Стор сам по себе давно верно читает denied ("выдано, если ключа нет в
    // denied") -- баг #1997 был в бэкенде: GetMyPermissions клал в Denied только
    // личные deny-override, а super-only ключи (выдача админки, режим техработ)
    // резолвер режет для admin неявно (PermissionSet.Has), в ответ они не
    // попадали. Обычный admin видел тумблер доступным, сервер отказывал на
    // сохранении. Тесты фиксируют актуальный контракт бэка (Denied включает
    // super-only для admin) с реальными именами ключей каталога.
    it('не выдаёт super-only ключ обычному admin, если бэк включил его в denied', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'admin',
        permissions: [],
        denied: ['action.grant.admin', 'page.admin.system_control'],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.hasPermission('action.grant.admin')).toBe(false)
      expect(store.hasPermission('page.admin.system_control')).toBe(false)
      // Обычное admin-право (не super-only, не в denied) остаётся доступным.
      expect(store.hasPermission('page.admin.users')).toBe(true)
    })

    it('супер-админу (mode: super) super-only ключ доступен', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'super',
        permissions: [],
        denied: [],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.hasPermission('action.grant.admin')).toBe(true)
      expect(store.hasPermission('page.admin.system_control')).toBe(true)
    })
  })

  describe('hasPermission — mode: banned', () => {
    it('возвращает false для любого ключа', () => {
      const store = usePermissionsStore()
      store.mode = 'banned'
      store.effective = { 'passes.create': { value: 'allow', source: 'role' } }

      expect(store.hasPermission('passes.create')).toBe(false)
      expect(store.hasPermission('page.admin')).toBe(false)
    })
  })

  describe('permissionSource', () => {
    it('возвращает источник из effective', () => {
      const store = usePermissionsStore()
      store.effective = { 'entity.cars.read': { value: 'allow', source: 'group' } }

      expect(store.permissionSource('entity.cars.read')).toBe('group')
    })

    it('возвращает null для отсутствующего ключа', () => {
      const store = usePermissionsStore()
      store.effective = {}

      expect(store.permissionSource('entity.cars.read')).toBe(null)
    })
  })

  describe('clearPermissions', () => {
    it('сбрасывает все поля в начальное состояние', () => {
      const store = usePermissionsStore()
      store.effective = { 'passes.create': { value: 'allow', source: 'role' } }
      store.permissions = { 'passes.create': 'allow' }
      store.mode = 'admin'
      store.denied = new Set(['x.key'])
      store.banned = true
      store.loaded = true

      store.clearPermissions()

      expect(store.effective).toEqual({})
      expect(store.permissions).toEqual({})
      expect(store.mode).toBe('normal')
      expect(store.denied.size).toBe(0)
      expect(store.banned).toBe(false)
      expect(store.loaded).toBe(false)
    })
  })

  describe('fetchPermissions — новый формат', () => {
    it('разбирает новый объект и заполняет mode/effective/denied', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'normal',
        permissions: [
          { key: 'passes.create', value: 'allow', source: 'role' },
          { key: 'passes.delete', value: 'deny', source: 'base' },
        ],
        denied: [],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.mode).toBe('normal')
      expect(store.effective['passes.create']).toEqual({ value: 'allow', source: 'role' })
      expect(store.effective['passes.delete']).toEqual({ value: 'deny', source: 'base' })
      // Плоская карта для совместимости
      expect(store.permissions['passes.create']).toBe('allow')
      expect(store.loaded).toBe(true)
    })

    it('разбирает mode: admin с denied', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'admin',
        permissions: [],
        denied: ['permission.audit.manage'],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.mode).toBe('admin')
      expect(store.denied.has('permission.audit.manage')).toBe(true)
      expect(store.hasPermission('passes.create')).toBe(true)
      expect(store.hasPermission('permission.audit.manage')).toBe(false)
    })

    it('разбирает mode: banned', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'banned',
        permissions: [],
        denied: [],
        banned: true,
        banReason: 'Нарушение правил',
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.mode).toBe('banned')
      expect(store.banned).toBe(true)
      expect(store.banReason).toBe('Нарушение правил')
      expect(store.hasPermission('passes.create')).toBe(false)
    })

    it('разбирает mode: super', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'super',
        permissions: [],
        denied: [],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.mode).toBe('super')
      expect(store.hasPermission('any.key')).toBe(true)
    })

    it('sets loaded to true при пустом permissions', async () => {
      getMyPermissions.mockResolvedValue({
        mode: 'normal',
        permissions: [],
        denied: [],
        banned: false,
        banReason: null,
      })

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.effective).toEqual({})
      expect(store.loaded).toBe(true)
    })
  })

  describe('fetchPermissions — defensive-parse старого формата (массив)', () => {
    it('разбирает старый массив [{key,value}] как normal/super', async () => {
      getMyPermissions.mockResolvedValue([
        { key: 'passes.create', value: 'allow' },
        { key: 'passes.delete', value: 'deny' },
      ])

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.effective['passes.create']).toEqual({ value: 'allow', source: 'base' })
      expect(store.effective['passes.delete']).toEqual({ value: 'deny', source: 'base' })
      expect(store.loaded).toBe(true)
    })

    it('не падает при пустом массиве', async () => {
      getMyPermissions.mockResolvedValue([])

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.effective).toEqual({})
      expect(store.loaded).toBe(true)
    })

    it('сохраняет старые permissions при ошибке API', async () => {
      getMyPermissions.mockRejectedValue(new Error('Network error'))

      const store = usePermissionsStore()
      store.effective = { 'old.key': { value: 'allow', source: 'role' } }
      await store.fetchPermissions()

      expect(store.effective['old.key']).toEqual({ value: 'allow', source: 'role' })
      expect(store.loaded).toBe(false)
    })

    it('перезаписывает permissions при повторном fetch', async () => {
      const store = usePermissionsStore()
      store.effective = { 'old.key': { value: 'allow', source: 'role' } }

      getMyPermissions.mockResolvedValue([
        { key: 'new.key', value: 'allow' },
      ])

      await store.fetchPermissions()

      expect(store.effective['new.key']).toBeDefined()
      expect(store.effective['old.key']).toBeUndefined()
    })

    it('для super_admin из auth определяет mode super при старом формате', async () => {
      const authStore = useAuthStore()
      authStore.setTokens(createMockJWT({ is_super_admin: true }))

      getMyPermissions.mockResolvedValue([
        { key: 'passes.create', value: 'deny' },
      ])

      const store = usePermissionsStore()
      await store.fetchPermissions()

      expect(store.mode).toBe('super')
      expect(store.hasPermission('passes.create')).toBe(true)
    })
  })
})
