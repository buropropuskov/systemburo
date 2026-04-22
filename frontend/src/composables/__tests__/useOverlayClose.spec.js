import { describe, it, expect, vi } from 'vitest'
import { useOverlayClose } from '../useOverlayClose'

function makeEvent(target, currentTarget) {
  return { target, currentTarget }
}

describe('useOverlayClose', () => {
  it('закрывает при mousedown+mouseup строго на оверлее', () => {
    const close = vi.fn()
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(close)
    const overlay = {}

    onOverlayMousedown(makeEvent(overlay, overlay))
    onOverlayMouseup(makeEvent(overlay, overlay))

    expect(close).toHaveBeenCalledOnce()
  })

  it('не закрывает, если mousedown начался внутри модалки (drag-out)', () => {
    const close = vi.fn()
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(close)
    const overlay = {}
    const modalContent = {}

    // mousedown на content (не всплывает до оверлея благодаря @mousedown.stop в шаблоне,
    // но даже если всплыл — e.target !== e.currentTarget).
    onOverlayMousedown(makeEvent(modalContent, overlay))
    onOverlayMouseup(makeEvent(overlay, overlay))

    expect(close).not.toHaveBeenCalled()
  })

  it('не закрывает, если mouseup на дочернем элементе оверлея', () => {
    const close = vi.fn()
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(close)
    const overlay = {}
    const child = {}

    onOverlayMousedown(makeEvent(overlay, overlay))
    onOverlayMouseup(makeEvent(child, overlay))

    expect(close).not.toHaveBeenCalled()
  })

  it('сбрасывает состояние между кликами', () => {
    const close = vi.fn()
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(close)
    const overlay = {}
    const modalContent = {}

    // Первый drag-out — не закрывает
    onOverlayMousedown(makeEvent(modalContent, overlay))
    onOverlayMouseup(makeEvent(overlay, overlay))
    expect(close).not.toHaveBeenCalled()

    // Второй нормальный клик — закрывает
    onOverlayMousedown(makeEvent(overlay, overlay))
    onOverlayMouseup(makeEvent(overlay, overlay))
    expect(close).toHaveBeenCalledOnce()
  })
})
