import { API_BASE_URL, apiRequest } from '@/api/client'

const states = new WeakMap()

function protectedPhotoPath(source) {
  try {
    const base = new URL(`${API_BASE_URL}/`, window.location.href)
    const url = new URL(source, window.location.href)
    if (url.origin !== base.origin || !url.pathname.startsWith(base.pathname)) return null
    const path = '/' + url.pathname.slice(base.pathname.length)
    if (!/^\/uploads\/(unload_places|system_tables)\//.test(path)) return null
    return path + url.search
  } catch {
    return null
  }
}

function release(state) {
  state.controller?.abort()
  state.controller = null
  if (state.blob) URL.revokeObjectURL(state.blob)
  state.blob = null
}

function update(el, vnode) {
  const state = states.get(el)
  const source = String(vnode.props?.src || '')
  if (state.source === source) return
  release(state)
  state.source = source
  state.attempted = false
  state.generation++
}

// A direct image may race refresh-cookie rotation. Recover once through the
// shared authenticated client; never send a token to an arbitrary image URL.
export const vAuthImage = {
  mounted(el, _binding, vnode) {
    const state = { source: null, attempted: false, generation: 0, blob: null, controller: null }
    state.onError = async () => {
      if (state.attempted) return
      state.attempted = true
      const path = protectedPhotoPath(state.source)
      if (!path) return
      const generation = state.generation
      const controller = new AbortController()
      state.controller = controller
      try {
        const response = await apiRequest(path, { silent: true, silent403: true, signal: controller.signal })
        if (!response.ok || !response.headers.get('Content-Type')?.startsWith('image/')) return
        const blob = await response.blob()
        if (controller.signal.aborted || states.get(el) !== state || state.generation !== generation) return
        state.blob = URL.createObjectURL(blob)
        el.src = state.blob
      } catch {
        // Keep the existing image/error UI. No further automatic retry.
      } finally {
        if (state.controller === controller) state.controller = null
      }
    }
    states.set(el, state)
    update(el, vnode)
    el.addEventListener('error', state.onError)
    if (el.complete && el.naturalWidth === 0 && state.source) state.onError()
  },
  updated(el, _binding, vnode) { update(el, vnode) },
  beforeUnmount(el) {
    const state = states.get(el)
    if (!state) return
    el.removeEventListener('error', state.onError)
    states.delete(el)
    release(state)
  },
}
