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

  it('скрывает элемент когда право deny', () => {
    const store = usePermissionsStore()
    store.mode = 'normal'
    store.effective = { 'passes.create': { value: 'deny', source: 'base' } }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).toBe('none')
  })

  it('скрывает элемент когда ключ отсутствует', () => {
    const store = usePermissionsStore()
    store.mode = 'normal'
    store.effective = {}

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).toBe('none')
  })

  it('показывает элемент когда право allow', () => {
    const store = usePermissionsStore()
    store.mode = 'normal'
    store.effective = { 'passes.create': { value: 'allow', source: 'role' } }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })

  it('показывает элемент для super независимо от effective', () => {
    const authStore = useAuthStore()
    authStore.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }))

    const store = usePermissionsStore()
    store.mode = 'super'
    store.effective = { 'passes.create': { value: 'deny', source: 'base' } }

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })

  it('показывает элемент для super при пустом effective', () => {
    const authStore = useAuthStore()
    authStore.setTokens(createMockJWT({ type_id: 6, is_super_admin: true }))

    const store = usePermissionsStore()
    store.mode = 'super'

    const wrapper = mountWithPermission('passes.create', pinia)

    expect(wrapper.find('div').element.style.display).not.toBe('none')
  })
})
