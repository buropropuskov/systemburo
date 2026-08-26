import { describe, it, expect, vi, afterEach } from 'vitest'
import { startTicketDownload, formatBytes } from '../download'

describe('formatBytes', () => {
  it('форматирует байты по границам единиц как cmd/server/cleanup.go humanBytes', () => {
    expect(formatBytes(0)).toBe('0 Б')
    expect(formatBytes(500)).toBe('500 Б')
    expect(formatBytes(1023)).toBe('1023 Б')
    expect(formatBytes(1024)).toBe('1 КБ')
    expect(formatBytes(1536)).toBe('2 КБ')
    expect(formatBytes(1024 * 1024)).toBe('1.0 МБ')
    expect(formatBytes(1.5 * 1024 * 1024)).toBe('1.5 МБ')
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1.0 ГБ')
    expect(formatBytes(2.5 * 1024 * 1024 * 1024)).toBe('2.5 ГБ')
  })

  it('отклоняет отрицательные и не-числа', () => {
    expect(formatBytes(-1)).toBe('—')
    expect(formatBytes(NaN)).toBe('—')
    expect(formatBytes(undefined)).toBe('—')
  })
})

describe('startTicketDownload', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('создаёт временную ссылку на /api<path>?ticket=... и кликает её', () => {
    const originalCreateElement = document.createElement.bind(document)
    const clickSpy = vi.fn()
    const appendSpy = vi.spyOn(document.body, 'appendChild')
    const removeSpy = vi.spyOn(document.body, 'removeChild')
    const createSpy = vi.spyOn(document, 'createElement').mockImplementation((tag) => {
      const el = originalCreateElement(tag)
      if (tag === 'a') el.click = clickSpy
      return el
    })

    startTicketDownload('/file-archive/download', 'tck 123')

    expect(createSpy).toHaveBeenCalledWith('a')
    const anchor = appendSpy.mock.calls[0][0]
    expect(anchor.href).toContain('/api/file-archive/download?ticket=tck%20123')
    expect(clickSpy).toHaveBeenCalled()
    expect(removeSpy).toHaveBeenCalledWith(anchor)
  })
})
