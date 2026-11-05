import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import { prepareApplicationCommand, finishApplicationCommand, resetApplicationCommand, recoverApplicationCommand, rememberApplicationCommandDraft, restoreApplicationCommandDraft } from '../applicationCommand'

const { auth, clock, request } = vi.hoisted(() => ({
  auth: { token: 'token', userPayload: { user_id: 7, sid: 'session-a' } },
  clock: { now: 1790000000000 },
  request: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiRequest: request }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/utils/serverTime', () => ({ serverNow: () => new Date(clock.now) }))

beforeEach(() => {
  vi.stubGlobal('crypto', webcrypto)
  sessionStorage.clear()
  auth.token = 'token'
  auth.userPayload = { user_id: 7, sid: 'session-a' }
  clock.now = 1790000000000
  request.mockReset().mockResolvedValue({ ok: true, json: async () => ({ state: 'unknown' }) })
})

afterEach(() => vi.unstubAllGlobals())

describe('application command metadata', () => {
  it('keeps one key across unknown result, retries and module reload', async () => {
    const one = await prepareApplicationCommand('application.submit', { message: 'private', file_ids: [8] })
    finishApplicationCommand(one, undefined)
    vi.resetModules()
    const reloaded = await import('../applicationCommand')
    const two = await reloaded.prepareApplicationCommand('application.submit', { file_ids: [8], message: 'private' })
    expect(two.key).toBe(one.key)
    expect(one.key).toMatch(/^v1\.1790000000000\.[0-9a-f]{32}$/)
    const stored = sessionStorage.getItem(one.storeKey)
    expect(Object.keys(JSON.parse(stored)).sort()).toEqual(['hash', 'key', 'status'])
    expect(stored).not.toContain('private')
    expect(stored).not.toContain('file_ids')
  })

  it('blocks modified unknown body and server-derived required users', async () => {
    const one = await prepareApplicationCommand('application.submit', { required_users: [{ user_id: 1 }] })
    finishApplicationCommand(one, 503)
    await expect(prepareApplicationCommand('application.submit', { required_users: [{ user_id: 2 }] })).rejects.toThrow('не подтверждена')
    expect(JSON.parse(sessionStorage.getItem(one.storeKey)).key).toBe(one.key)
  })

  it('known rejection keeps unchanged key; explicit editing creates a new command', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    finishApplicationCommand(one, 400)
    expect((await prepareApplicationCommand('application.submit', { a: 1 })).key).toBe(one.key)
    finishApplicationCommand(one, 400)
    expect((await prepareApplicationCommand('application.submit', { a: 2 })).key).not.toBe(one.key)
  })

  it('409 never silently converts an unknown command into a new key', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    finishApplicationCommand(one, 409)
    await expect(prepareApplicationCommand('application.submit', { a: 2 })).rejects.toThrow('не подтверждена')
  })

  it('success and explicit new-form reset permit another intentional command', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    finishApplicationCommand(one, 'success')
    const two = await prepareApplicationCommand('application.submit', { a: 1 })
    expect(two.key).not.toBe(one.key)
    await resetApplicationCommand('application.submit')
    expect((await prepareApplicationCommand('application.submit', { a: 1 })).key).not.toBe(two.key)
  })

  it('expires after 24h and uses existing synchronized server clock', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    clock.now += 86400001
    await expect(prepareApplicationCommand('application.submit', { a: 1 })).rejects.toThrow('не подтверждена')
    expect(JSON.parse(sessionStorage.getItem(one.storeKey)).key).toBe(one.key)
  })

  it('different owner/effective session and operation scopes do not share keys', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    const supplement = await prepareApplicationCommand('application.supplement:5', { a: 1 })
    expect(supplement.key).not.toBe(one.key)
    auth.userPayload = { user_id: 8, sid: 'session-b' }
    const other = await prepareApplicationCommand('application.submit', { a: 1 })
    expect(other.key).not.toBe(one.key)
    finishApplicationCommand(one, 'success')
    expect(sessionStorage.getItem(one.storeKey)).not.toBeNull()
  })

  it('refuses to send if metadata cannot be persisted', async () => {
    const fail = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('quota') })
    await expect(prepareApplicationCommand('application.submit', { a: 1 })).rejects.toThrow('quota')
    fail.mockRestore()
  })

  it('known rejected command may be explicitly edited after the old key expires', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    finishApplicationCommand(one, 400)
    clock.now += 86400001
    const two = await prepareApplicationCommand('application.submit', { a: 2 })
    expect(two.key).not.toBe(one.key)
    expect(request).not.toHaveBeenCalled()
  })

  it('recovers a committed result after reload without sending the lost body', async () => {
    const scope = 'application.supplement:42'
    const one = await prepareApplicationCommand(scope, { additions: ['private rows'] })
    finishApplicationCommand(one, undefined)
    request.mockResolvedValue({ ok: true, json: async () => ({ state: 'committed', result: { supplement_id: 8, number: 2, status: 'pending', counts: {} } }) })
    const result = await recoverApplicationCommand(scope)
    expect(result.supplement_id).toBe(8)
    expect(request).toHaveBeenCalledWith('/applications/operations/status', { method: 'POST', silent: true, body: JSON.stringify({ key: one.key, scope }) })
    expect(sessionStorage.getItem(one.storeKey)).toBeNull()
  })

  it('expired unknown is only looked up, never renewed when lookup is absent', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    clock.now += 86400001
    await expect(prepareApplicationCommand('application.submit', { a: 1 })).rejects.toThrow('не подтверждена')
    expect(request).toHaveBeenCalledTimes(1)
    expect(JSON.parse(sessionStorage.getItem(one.storeKey)).key).toBe(one.key)
  })

  it('does not clear metadata for malformed committed results or session changes', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    request.mockResolvedValueOnce({ ok: true, json: async () => ({ state: 'committed', result: {} }) })
    await expect(recoverApplicationCommand('application.submit')).rejects.toThrow('не подтверждён')
    expect(sessionStorage.getItem(one.storeKey)).not.toBeNull()
    request.mockImplementationOnce(async () => {
      auth.userPayload = { user_id: 8, sid: 'changed' }
      return { ok: true, json: async () => ({ state: 'committed', result: { application_id: 1, application_number: '1' } }) }
    })
    await expect(recoverApplicationCommand('application.submit')).rejects.toThrow('Сеанс изменился')
    expect(sessionStorage.getItem(one.storeKey)).not.toBeNull()
  })

  it('preserves unknown supplement draft in RAM, not in storage, and releases it on success', async () => {
    const scope = 'application.supplement:42'
    const one = await prepareApplicationCommand(scope, { additions: [1] })
    const draft = { rowsByAttachment: { 9: [{ lastName: 'private draft' }] }, comment: 'private comment' }
    rememberApplicationCommandDraft(one, draft)
    finishApplicationCommand(one, undefined)
    expect(await restoreApplicationCommandDraft(scope)).toEqual(draft)
    expect(sessionStorage.getItem(one.storeKey)).not.toContain('private')
    finishApplicationCommand(one, 'success')
    expect(await restoreApplicationCommandDraft(scope)).toBeNull()
  })

  it('an absent lookup may precede a preserved-body retry, but never renews metadata', async () => {
    const one = await prepareApplicationCommand('application.submit', { a: 1 })
    finishApplicationCommand(one, undefined)
    expect(await recoverApplicationCommand('application.submit', { allowUnknown: true })).toBeNull()
    expect((await prepareApplicationCommand('application.submit', { a: 1 })).key).toBe(one.key)
  })
})
