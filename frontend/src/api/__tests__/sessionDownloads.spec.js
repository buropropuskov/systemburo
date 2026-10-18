import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { request } = vi.hoisted(() => ({ request: vi.fn() }))
vi.mock('@/api/client', () => ({ apiRequest: request }))
import { downloadApplicationFile } from '../applicationFiles'
import { downloadApplicationsRegistry } from '../applications'
import { downloadDocument } from '../documents'
import { downloadGuideFile } from '../guide'

describe('downloads share session handling', () => {
  beforeEach(() => {
    request.mockReset()
    request.mockResolvedValue(new Response('synthetic', { headers: { 'Content-Disposition': 'attachment; filename="registry.xlsx"' } }))
    URL.createObjectURL = vi.fn(() => 'blob:download')
    URL.revokeObjectURL = vi.fn()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })
  afterEach(() => vi.restoreAllMocks())

  it.each([
    [() => downloadApplicationFile(1, 2, 'scan.pdf'), '/applications/1/files/2'],
    [() => downloadApplicationsRegistry({ status: 'active' }), '/applications/export?status=active'],
    [() => downloadDocument(3, 'document.pdf'), '/documents/3/download'],
    [() => downloadGuideFile('/api/guide/sections/user/download', 'guide.pdf'), '/guide/sections/user/download'],
  ])('uses the common client: %s', async (download, path) => {
    await download()
    expect(request).toHaveBeenCalledWith(path)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:download')
  })

  it.each(['https://foreign.invalid/api/guide/sections/user/download', '//foreign.invalid/guide', '/api/../users/me'])('rejects an unexpected guide address %s', async (path) => {
    await expect(downloadGuideFile(path)).rejects.toThrow('Некорректный адрес')
    expect(request).not.toHaveBeenCalled()
  })
})
