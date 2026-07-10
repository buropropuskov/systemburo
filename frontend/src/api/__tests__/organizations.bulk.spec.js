import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
}))
import { apiRequest } from '@/api/client'
import {
  bulkUpdateOrganizationType,
  bulkAssignOrganizationUnloadPlaces,
  bulkAssignOrganizationTables,
  bulkAssignOrganizationUsers,
  bulkArchiveOrganizations,
  bulkRestoreOrganizations,
  bulkUpdateCompanyType,
  bulkAssignCompanyUnloadPlaces,
  bulkAssignCompanyTables,
  bulkAssignCompanyUsers,
  bulkArchiveCompanies,
  bulkRestoreCompanies,
} from '../organizations'

const RESULT = { success_count: 2, error_count: 0, errors: [] }

function okJson(payload) {
  return { ok: true, status: 200, json: vi.fn().mockResolvedValue(payload) }
}

// Разбирает JSON-body второго аргумента apiRequest (реальная форма запроса к BE).
function bodyOf(call) {
  return JSON.parse(call[1].body)
}

describe('api/organizations — bulk-обёртки', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiRequest.mockResolvedValue(okJson(RESULT))
  })

  it('bulkUpdateOrganizationType: POST /organizations/bulk/type c ids+type', async () => {
    const res = await bulkUpdateOrganizationType([1, 2], 'Отдел')
    const call = apiRequest.mock.calls[0]
    expect(call[0]).toBe('/organizations/bulk/type')
    expect(call[1].method).toBe('POST')
    expect(bodyOf(call)).toEqual({ ids: [1, 2], type: 'Отдел' })
    expect(res).toEqual(RESULT)
  })

  it('bulkUpdateOrganizationType: type=null проходит как null (снять тип)', async () => {
    await bulkUpdateOrganizationType([3], null)
    expect(bodyOf(apiRequest.mock.calls[0])).toEqual({ ids: [3], type: null })
  })

  it('bulkAssignOrganizationUnloadPlaces: snake_case unload_place_ids + mode', async () => {
    await bulkAssignOrganizationUnloadPlaces([1], [5, 6], 'add')
    const call = apiRequest.mock.calls[0]
    expect(call[0]).toBe('/organizations/bulk/unload-places')
    expect(bodyOf(call)).toEqual({ ids: [1], unload_place_ids: [5, 6], mode: 'add' })
  })

  it('bulkAssignOrganizationTables: snake_case table_ids + mode', async () => {
    await bulkAssignOrganizationTables([1, 2], [9], 'replace')
    const call = apiRequest.mock.calls[0]
    expect(call[0]).toBe('/organizations/bulk/tables')
    expect(bodyOf(call)).toEqual({ ids: [1, 2], table_ids: [9], mode: 'replace' })
  })

  it('bulkAssignOrganizationUsers: users с индивидуальным required_approval + mode', async () => {
    await bulkAssignOrganizationUsers([1], [
      { username: 'ivanov', required_approval: true },
      { username: 'petrov', required_approval: false },
    ], 'add')
    const call = apiRequest.mock.calls[0]
    expect(call[0]).toBe('/organizations/bulk/users')
    expect(bodyOf(call)).toEqual({
      ids: [1],
      users: [
        { username: 'ivanov', required_approval: true },
        { username: 'petrov', required_approval: false },
      ],
      mode: 'add',
    })
  })

  it('bulkArchiveOrganizations / bulkRestoreOrganizations: только ids', async () => {
    await bulkArchiveOrganizations([1, 2])
    expect(apiRequest.mock.calls[0][0]).toBe('/organizations/bulk/archive')
    expect(bodyOf(apiRequest.mock.calls[0])).toEqual({ ids: [1, 2] })

    await bulkRestoreOrganizations([3])
    expect(apiRequest.mock.calls[1][0]).toBe('/organizations/bulk/restore')
    expect(bodyOf(apiRequest.mock.calls[1])).toEqual({ ids: [3] })
  })

  it('company-обёртки бьют в /companies/bulk/* с той же формой тела', async () => {
    await bulkUpdateCompanyType([1], 'Компания')
    expect(apiRequest.mock.calls[0][0]).toBe('/companies/bulk/type')
    expect(bodyOf(apiRequest.mock.calls[0])).toEqual({ ids: [1], type: 'Компания' })

    await bulkAssignCompanyUnloadPlaces([1], [2], 'replace')
    expect(apiRequest.mock.calls[1][0]).toBe('/companies/bulk/unload-places')
    expect(bodyOf(apiRequest.mock.calls[1])).toEqual({ ids: [1], unload_place_ids: [2], mode: 'replace' })

    await bulkAssignCompanyTables([1], [2], 'add')
    expect(apiRequest.mock.calls[2][0]).toBe('/companies/bulk/tables')
    expect(bodyOf(apiRequest.mock.calls[2])).toEqual({ ids: [1], table_ids: [2], mode: 'add' })

    await bulkAssignCompanyUsers([1], [{ username: 'u', required_approval: false }], 'replace')
    expect(apiRequest.mock.calls[3][0]).toBe('/companies/bulk/users')
    expect(bodyOf(apiRequest.mock.calls[3])).toEqual({ ids: [1], users: [{ username: 'u', required_approval: false }], mode: 'replace' })

    await bulkArchiveCompanies([1])
    expect(apiRequest.mock.calls[4][0]).toBe('/companies/bulk/archive')

    await bulkRestoreCompanies([1])
    expect(apiRequest.mock.calls[5][0]).toBe('/companies/bulk/restore')
  })
})
