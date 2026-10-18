import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePermissionsStore } from '../permissions'

describe('application action entitlements', () => {
  beforeEach(() => setActivePinia(createPinia()))
  for (const key of ['action.forward.application', 'action.approve.application']) {
    it(`${key}: waits for permissions and preserves the object entitlement without a grant`, () => {
      const store = usePermissionsStore()
      expect(store.allowsApplicationAction(key)).toBe(false)
      store.loaded = true
      expect(store.allowsApplicationAction(key)).toBe(true)
      store.denied.add(key)
      expect(store.allowsApplicationAction(key)).toBe(false)
    })
    it(`${key}: an admin denial applies; super retains full access`, () => {
      const store = usePermissionsStore()
      store.loaded = true
      store.mode = 'admin'
      store.denied.add(key)
      expect(store.allowsApplicationAction(key)).toBe(false)
      store.mode = 'super'
      expect(store.allowsApplicationAction(key)).toBe(true)
      store.banned = true
      expect(store.allowsApplicationAction(key)).toBe(false)
    })
    it(`${key}: normal effective deny applies`, () => {
      const store = usePermissionsStore()
      store.loaded = true
      store.effective[key] = { value: 'deny', source: 'override' }
      expect(store.allowsApplicationAction(key)).toBe(false)
    })
  }
  it('does not grant unrelated operations', () => {
    const store = usePermissionsStore()
    store.loaded = true
    store.mode = 'super'
    expect(store.allowsApplicationAction('page.admin')).toBe(false)
  })
})
