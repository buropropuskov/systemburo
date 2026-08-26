import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePermissionsStore } from '@/stores/permissions'
import { usePermission } from '../usePermission'

describe('usePermission', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('can()', () => {
    it('возвращает true для mode super', () => {
      const store = usePermissionsStore()
      store.mode = 'super'

      const { can } = usePermission()
      expect(can('page.admin')).toBe(true)
      expect(can('action.export.applications')).toBe(true)
    })

    it('возвращает false для mode banned', () => {
      const store = usePermissionsStore()
      store.mode = 'banned'
      store.effective = { 'page.center': { value: 'allow', source: 'role' } }

      const { can } = usePermission()
      expect(can('page.center')).toBe(false)
    })

    it('возвращает true для mode admin когда ключ не в denied', () => {
      const store = usePermissionsStore()
      store.mode = 'admin'
      store.denied = new Set()

      const { can } = usePermission()
      expect(can('entity.cars.read')).toBe(true)
    })

    it('возвращает false для mode admin когда ключ в denied', () => {
      const store = usePermissionsStore()
      store.mode = 'admin'
      store.denied = new Set(['permission.audit.manage'])

      const { can } = usePermission()
      expect(can('permission.audit.manage')).toBe(false)
    })

    it('работает для mode normal с allow', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = { 'page.cars': { value: 'allow', source: 'group' } }

      const { can } = usePermission()
      expect(can('page.cars')).toBe(true)
    })

    it('работает для mode normal с deny', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = { 'page.cars': { value: 'deny', source: 'base' } }

      const { can } = usePermission()
      expect(can('page.cars')).toBe(false)
    })

    it('работает с динамическими ключами вида table.<slug>.view', () => {
      const store = usePermissionsStore()
      store.mode = 'normal'
      store.effective = {
        'table.visitors.view': { value: 'allow', source: 'role' },
        'table.access.view':   { value: 'deny', source: 'base' },
      }

      const { can } = usePermission()
      expect(can('table.visitors.view')).toBe(true)
      expect(can('table.access.view')).toBe(false)
      expect(can('table.unknown.view')).toBe(false)
    })
  })

  describe('sourceOf()', () => {
    it('возвращает источник из effective', () => {
      const store = usePermissionsStore()
      store.effective = {
        'entity.cars.read': { value: 'allow', source: 'group' },
        'action.ban.user':  { value: 'allow', source: 'override' },
      }

      const { sourceOf } = usePermission()
      expect(sourceOf('entity.cars.read')).toBe('group')
      expect(sourceOf('action.ban.user')).toBe('override')
    })

    it('возвращает null для отсутствующего ключа', () => {
      const store = usePermissionsStore()
      store.effective = {}

      const { sourceOf } = usePermission()
      expect(sourceOf('entity.cars.read')).toBe(null)
    })
  })
})
