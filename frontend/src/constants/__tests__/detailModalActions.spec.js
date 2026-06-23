import { describe, it, expect } from 'vitest'
import { getModalActionPermission } from '@/constants/detailModalActions'

describe('detailModalActions — openApplication гейтится detail.open_application', () => {
  it('vehicle: контексты с переходом в заявку требуют detail.open_application', () => {
    for (const src of ['carstable', 'facttable', 'general']) {
      expect(getModalActionPermission('vehicle', src, 'openApplication')).toBe('detail.open_application')
    }
  })

  it('employee: контексты с переходом в заявку требуют detail.open_application', () => {
    for (const src of ['employeesview', 'peopletable', 'general']) {
      expect(getModalActionPermission('employee', src, 'openApplication')).toBe('detail.open_application')
    }
  })

  it('контекстно-скрытые остаются false (уже в заявке / корзина / ЧС / страница авто)', () => {
    expect(getModalActionPermission('vehicle', 'application', 'openApplication')).toBe(false)
    expect(getModalActionPermission('vehicle', 'carsview', 'openApplication')).toBe(false)
    expect(getModalActionPermission('employee', 'application', 'openApplication')).toBe(false)
    expect(getModalActionPermission('employee', 'trash', 'openApplication')).toBe(false)
  })

  it('не задел смежные действия: documents/blacklist прежние', () => {
    expect(getModalActionPermission('vehicle', 'carsview', 'documents')).toBe('detail.documents')
    expect(getModalActionPermission('employee', 'peopletable', 'blacklist')).toBe('page.admin.blacklist')
  })
})

describe('detailModalActions — гранулярные detail-ключи карточки', () => {
  it('history гейтится detail.full_history (vehicle и employee)', () => {
    for (const src of ['application', 'carstable', 'facttable', 'carsview', 'trash', 'general']) {
      expect(getModalActionPermission('vehicle', src, 'history')).toBe('detail.full_history')
    }
    for (const src of ['application', 'employeesview', 'peopletable', 'trash', 'general']) {
      expect(getModalActionPermission('employee', src, 'history')).toBe('detail.full_history')
    }
  })

  it('entryExit гейтится detail.entry_exit_history там, где раздел доступен', () => {
    for (const src of ['application', 'carstable', 'facttable', 'carsview', 'general']) {
      expect(getModalActionPermission('vehicle', src, 'entryExit')).toBe('detail.entry_exit_history')
    }
    for (const src of ['application', 'employeesview', 'peopletable', 'general']) {
      expect(getModalActionPermission('employee', src, 'entryExit')).toBe('detail.entry_exit_history')
    }
  })

  it('passHistory выкинут (нет UI «История пропусков») — всегда false', () => {
    for (const src of ['carsview', 'general', 'application', 'trash']) {
      expect(getModalActionPermission('vehicle', src, 'passHistory')).toBe(false)
    }
    for (const src of ['employeesview', 'general', 'peopletable', 'trash']) {
      expect(getModalActionPermission('employee', src, 'passHistory')).toBe(false)
    }
  })

  it('контекст ЧС не задет: history там остаётся page.admin.blacklist', () => {
    expect(getModalActionPermission('vehicle', 'blacklist', 'history')).toBe('page.admin.blacklist')
    expect(getModalActionPermission('employee', 'blacklist', 'history')).toBe('page.admin.blacklist')
  })

  it('контекстно-скрытые detail-действия остаются false', () => {
    expect(getModalActionPermission('vehicle', 'trash', 'entryExit')).toBe(false)
    expect(getModalActionPermission('vehicle', 'application', 'passHistory')).toBe(false)
    expect(getModalActionPermission('employee', 'employeeslist', 'history')).toBe(false)
    expect(getModalActionPermission('employee', 'peopletable', 'passHistory')).toBe(false)
  })
})
