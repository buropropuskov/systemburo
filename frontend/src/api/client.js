import { useAuthStore } from '@/stores/auth'

const API_BASE_URL = process.env.VUE_APP_API_BASE_URL || ''

export async function apiRequest(path, options = {}) {
  const authStore = useAuthStore()
  const token = authStore.token
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)

  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      signal: options.signal || controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...(token && { Authorization: `Bearer ${token}` }),
        ...options.headers,
      },
    })
    clearTimeout(timeout)

    // Monkey-patch .json() to auto-unwrap the {success, data, error} envelope
    const originalJson = response.json.bind(response)
    response.json = async () => {
      const body = await originalJson()
      if (body && typeof body === 'object' && 'success' in body) {
        if (!body.success) {
          // Return {message: error} for backward compatibility with error parsing
          // Components that do: const err = await response.json(); showNotification(err.message)
          return { message: body.error }
        }
        return body.data
      }
      return body
    }

    return response
  } catch (err) {
    clearTimeout(timeout)
    throw err
  }
}

// apiRequestRaw returns full envelope without unwrapping (for pagination meta, etc.)
export async function apiRequestRaw(path, options = {}) {
  const authStore = useAuthStore()
  const token = authStore.token
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)

  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      signal: options.signal || controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...(token && { Authorization: `Bearer ${token}` }),
        ...options.headers,
      },
    })
    clearTimeout(timeout)
    return response
  } catch (err) {
    clearTimeout(timeout)
    throw err
  }
}
