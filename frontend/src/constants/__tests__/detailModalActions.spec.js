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
