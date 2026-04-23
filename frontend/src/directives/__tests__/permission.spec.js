import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { vPermission } from '../permission'
import { usePermissionsStore } from '@/stores/permissions'
import { useAuthStore } from '@/stores/auth'

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }))
  return `${header}.${body}.fake-signature`
}

function mountWithPermission(permissionKey, pinia) {
  return mount({
    template: `<div v-permission="'${permissionKey}'">Content</div>`,
    directives: { permission: vPermission },
  }, {
    global: { plugins: [pinia] },
  })
}

describe('v-permission directive', () => {
  let pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    localStorage.clear()
  })

  it('hides element when permission is denied', () => {
    const store = usePermissionsStore()
    store.permissions = { 'passes.create': 'deny' }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).toBe('none')
  })

  it('hides element when permission is missing', () => {
    const store = usePermissionsStore()
    store.permissions = {}

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).toBe('none')
  })

  it('shows element when permission is allowed', () => {
    const store = usePermissionsStore()
    store.permissions = { 'passes.create': 'allow' }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })

  it('shows element for admin regardless of permission', () => {
    const authStore = useAuthStore()
    authStore.setTokens(createMockJWT({ type_id: 6 }))

    const store = usePermissionsStore()
    store.permissions = { 'passes.create': 'deny' }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })

  it('shows element for admin even when permission is missing', () => {
    const authStore = useAuthStore()
    authStore.setTokens(createMockJWT({ type_id: 6 }))

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })
})
