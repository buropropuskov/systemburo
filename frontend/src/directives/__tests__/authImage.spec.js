import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { request } = vi.hoisted(() => ({ request: vi.fn() }))
vi.mock('@/api/client', () => ({ API_BASE_URL: '/api', apiRequest: request }))
import { vAuthImage } from '../authImage'

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('authenticated image recovery', () => {
  let image
  beforeEach(() => {
    request.mockReset()
    image = document.createElement('img')
    Object.defineProperty(image, 'complete', { value: false })
    URL.createObjectURL = vi.fn(() => 'blob:recovered')
    URL.revokeObjectURL = vi.fn()
  })
  afterEach(() => vAuthImage.beforeUnmount(image))

  it('recovers a protected image once and releases the object URL', async () => {
    request.mockResolvedValue(new Response('photo', { headers: { 'Content-Type': 'image/png' } }))
    vAuthImage.mounted(image, {}, { props: { src: '/api/uploads/unload_places/photo.png' } })
    image.dispatchEvent(new Event('error'))
    await flush()
    expect(image.src).toBe('blob:recovered')
    image.dispatchEvent(new Event('error'))
    await flush()
    expect(request).toHaveBeenCalledTimes(1)
    expect(request.mock.calls[0][0]).toBe('/uploads/unload_places/photo.png')
    vAuthImage.beforeUnmount(image)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:recovered')
  })

  it.each(['https://foreign.invalid/api/uploads/unload_places/p.png', 'data:image/png;base64,AA', 'blob:any', '/api/uploads/application_files/p.png'])('does not authorize %s', async (src) => {
    vAuthImage.mounted(image, {}, { props: { src } })
    image.dispatchEvent(new Event('error'))
    await flush()
    expect(request).not.toHaveBeenCalled()
  })

  it('does not make a blob or loop after an access denial', async () => {
    request.mockResolvedValue(new Response('{}', { status: 403 }))
    vAuthImage.mounted(image, {}, { props: { src: '/api/uploads/system_tables/photo.png' } })
    image.dispatchEvent(new Event('error'))
    await flush()
    image.dispatchEvent(new Event('error'))
    await flush()
    expect(request).toHaveBeenCalledTimes(1)
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })

  it('aborts on unmount and ignores the late image', async () => {
    let finish
    request.mockReturnValue(new Promise((resolve) => { finish = resolve }))
    vAuthImage.mounted(image, {}, { props: { src: '/api/uploads/system_tables/photo.png' } })
    image.dispatchEvent(new Event('error'))
    const signal = request.mock.calls[0][1].signal
    vAuthImage.beforeUnmount(image)
    finish(new Response('late', { headers: { 'Content-Type': 'image/png' } }))
    await flush()
    expect(signal.aborted).toBe(true)
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })

  it('ignores a late response after source change', async () => {
    let finish
    request.mockReturnValue(new Promise((resolve) => { finish = resolve }))
    vAuthImage.mounted(image, {}, { props: { src: '/api/uploads/system_tables/old.png' } })
    image.dispatchEvent(new Event('error'))
    const signal = request.mock.calls[0][1].signal
    vAuthImage.updated(image, {}, { props: { src: '/api/uploads/system_tables/new.png' } })
    finish(new Response('old', { headers: { 'Content-Type': 'image/png' } }))
    await flush()
    expect(signal.aborted).toBe(true)
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })
})
