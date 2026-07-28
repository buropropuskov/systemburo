import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { isErrorPageReachable, closeIncidentOnLeave } from '@/utils/errorPageAccess'
import { saveBugContext, buildBugContext, clearBugContext, loadBugContext } from '@/composables/useBugReport'
import { useMaintenanceStore } from '@/stores/maintenance'

const route = (name, query = {}) => ({ name, query })

describe('utils/errorPageAccess', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
  })

  it('пускает на обычные страницы без условий', () => {
    expect(isErrorPageReachable(route('news'))).toBe(true)
    expect(isErrorPageReachable(route('center'))).toBe(true)
  })

  it('не пускает на /500 без сохранённого инцидента', () => {
    expect(isErrorPageReachable(route('Error500'))).toBe(false)
  })

  it('пускает на /500 после реальной 5xx-ошибки', () => {
    saveBugContext(buildBugContext({ route: 'GET /applications', httpStatus: 502 }))
    expect(isErrorPageReachable(route('Error500'))).toBe(true)
  })

  it('перестаёт пускать на /500 после закрытия инцидента', () => {
    saveBugContext(buildBugContext({ route: 'GET /applications', httpStatus: 500 }))
    clearBugContext()
    expect(isErrorPageReachable(route('Error500'))).toBe(false)
  })

  it('уход с /500 закрывает инцидент и закрывает повторный вход', () => {
    saveBugContext(buildBugContext({ route: 'GET /notifications', httpStatus: 500 }))

    closeIncidentOnLeave(route('news'), route('Error500'))
    expect(loadBugContext()).toBeNull()
    expect(isErrorPageReachable(route('Error500'))).toBe(false)
  })

  it('не трогает инцидент, пока юзер остаётся на /500', () => {
    saveBugContext(buildBugContext({ route: 'GET /notifications', httpStatus: 500 }))

    closeIncidentOnLeave(route('Error500'), route('Error500'))
    expect(loadBugContext()).not.toBeNull()

    // и первый вход на страницу (from - обычный роут) инцидент не гасит
    closeIncidentOnLeave(route('Error500'), route('news'))
    expect(loadBugContext()).not.toBeNull()
  })

  it('пускает на /maintenance только при включённых техработах', () => {
    const store = useMaintenanceStore()
    expect(isErrorPageReachable(route('Maintenance'))).toBe(false)

    store.setFromPayload({ enabled: true, message: 'Обновляем систему' })
    expect(isErrorPageReachable(route('Maintenance'))).toBe(true)
  })

  it('пускает на /403 только с permission из permission-guard', () => {
    expect(isErrorPageReachable(route('Forbidden'))).toBe(false)
    expect(isErrorPageReachable(route('Forbidden', { permission: 'page.admin.users' }))).toBe(true)
  })
})
