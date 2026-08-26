import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../client', () => ({ apiRequest: vi.fn() }))
import { apiRequest } from '../client'
import { getOrganizationMembers, getCompanyMembers } from '../organizations'

function okJson(payload) {
  return { ok: true, json: vi.fn().mockResolvedValue(payload) }
}

describe('api участников организаций/компаний (#1046)', () => {
  beforeEach(() => vi.clearAllMocks())

  it('getOrganizationMembers дёргает /organizations/:id/members и возвращает данные', async () => {
    const members = [{ id: 1, username: 'ivan', last_name: 'Иванов' }]
    apiRequest.mockResolvedValue(okJson(members))

    const res = await getOrganizationMembers(7)

    expect(apiRequest).toHaveBeenCalledWith('/organizations/7/members')
    expect(res).toEqual(members)
  })

  it('getCompanyMembers дёргает /companies/:id/members и возвращает данные', async () => {
    const members = [{ id: 2, username: 'petr', last_name: 'Петров' }]
    apiRequest.mockResolvedValue(okJson(members))

    const res = await getCompanyMembers(3)

    expect(apiRequest).toHaveBeenCalledWith('/companies/3/members')
    expect(res).toEqual(members)
  })
})
