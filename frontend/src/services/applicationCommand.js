import { useAuthStore } from '@/stores/auth'
import { serverNow } from '@/utils/serverTime'
import { apiRequest } from '@/api/client'

const PREFIX = 'application-command:'
const volatileDrafts = new Map()
let volatileSession = null
const WINDOW_MS = 24 * 60 * 60 * 1000
const UNKNOWN = 'Предыдущая отправка ещё не подтверждена. Проверь результат в списке заявок перед изменением и новой отправкой.'

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical)
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonical(value[key])]))
  return value
}
async function hash(value) {
  const bytes = new TextEncoder().encode(JSON.stringify(canonical(value)))
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
}
function identity() {
  const auth = useAuthStore()
  const user = auth.userPayload
  if (!user?.user_id || !auth.token) throw new Error('Сеанс недоступен. Войди снова перед отправкой.')
  const session = JSON.stringify([user.sid || null, user.session_user_id || user.user_id, user.user_id, user.impersonated_by || null, auth.impersonation?.id || null])
  if (volatileSession !== session) { volatileDrafts.clear(); volatileSession = session }
  return session
}
async function storageKey(scope, session) { return PREFIX + await hash([session, scope]) }
function read(key) {
  const value = sessionStorage.getItem(key)
  if (!value) return null
  try {
    const state = JSON.parse(value)
    if (!/^v1\.\d{13}\.[0-9a-f]{32}$/.test(state.key) || !/^[0-9a-f]{64}$/.test(state.hash)
      || !['pending', 'unknown', 'failed'].includes(state.status)) throw new Error('invalid metadata')
    return state
  } catch { throw new Error(UNKNOWN) }
}

/** Persist only key/hash/status. Neither form bodies nor personal data are stored. */
export async function prepareApplicationCommand(scope, payload) {
  const session = identity()
  const storeKey = await storageKey(scope, session)
  const bodyHash = await hash(payload)
  if (identity() !== session) throw new Error('Сеанс изменился. Повтори отправку из текущего сеанса.')
  let state = read(storeKey)
  if (state) {
    if (state.hash !== bodyHash && state.status === 'failed') {
      state = null // an explicitly edited command following a confirmed rejection
    } else {
      const age = serverNow().getTime() - Number(state.key.split('.')[1])
      if (state.hash !== bodyHash || age > WINDOW_MS || age < -300000) {
        if (state.status !== 'failed') {
          const recovered = await recoverApplicationCommand(scope)
          if (recovered) return { key: state.key, storeKey, session, result: recovered }
        }
        throw new Error(UNKNOWN)
      }
    }
  }
  if (!state) {
    const bytes = crypto.getRandomValues(new Uint8Array(16))
    const random = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('')
    state = { key: `v1.${serverNow().getTime()}.${random}`, hash: bodyHash, status: 'pending' }
  }
  state.status = 'pending'
  sessionStorage.setItem(storeKey, JSON.stringify(state))
  return { key: state.key, storeKey, session }
}

export function finishApplicationCommand(command, status) {
  if (!command) return
  try { if (identity() !== command.session) return } catch { return }
  const state = read(command.storeKey)
  if (!state || state.key !== command.key) return
  if (status === 'success') { sessionStorage.removeItem(command.storeKey); volatileDrafts.delete(command.storeKey) }
  else {
    // A 409 may mean an expired operation or an unresolved prior command.
    state.status = status >= 400 && status < 500 && status !== 409 ? 'failed' : 'unknown'
    if (state.status === 'failed') volatileDrafts.delete(command.storeKey)
    sessionStorage.setItem(command.storeKey, JSON.stringify(state))
  }
}

/** Call only for an explicit new-form action; close/reload do not reset commands. */
export async function resetApplicationCommand(scope) {
  const session = identity()
  const key = await storageKey(scope, session)
  if (identity() === session) { sessionStorage.removeItem(key); volatileDrafts.delete(key) }
}

// Supplement drafts live only in this tab's memory. Reload never writes their PII.
export function rememberApplicationCommandDraft(command, draft) {
  if (identity() === command.session) volatileDrafts.set(command.storeKey, { key: command.key, draft: JSON.parse(JSON.stringify(draft)) })
}
export async function restoreApplicationCommandDraft(scope) {
  const session = identity()
  const key = await storageKey(scope, session)
  if (identity() !== session) return null
  const state = read(key)
  const cached = volatileDrafts.get(key)
  return state && state.status !== 'failed' && cached?.key === state.key ? JSON.parse(JSON.stringify(cached.draft)) : null
}

// Absence can mean an in-flight or lost operation; it never authorizes a new key.
export async function recoverApplicationCommand(scope, { allowUnknown = false } = {}) {
  const session = identity()
  const storeKey = await storageKey(scope, session)
  const state = read(storeKey)
  if (!state || state.status === 'failed') return null
  const response = await apiRequest('/applications/operations/status', {
    method: 'POST', silent: true, body: JSON.stringify({ key: state.key, scope }),
  })
  if (identity() !== session) throw new Error('Сеанс изменился. Проверь результат из текущего сеанса.')
  if (!response.ok) throw new Error('Не удалось проверить прежнюю отправку. Новая отправка не выполнена.')
  const data = await response.json()
  if (identity() !== session) throw new Error('Сеанс изменился. Проверь результат из текущего сеанса.')
  if (data?.state === 'unknown' && allowUnknown) return null
  if (data?.state !== 'committed' || !data.result) throw new Error('Прежняя отправка пока не подтверждена. Проверь список заявок или дополнений. Новая отправка не выполнена.')
  const result = data.result
  const valid = scope === 'application.submit'
    ? Number.isInteger(result.application_id) && result.application_id > 0 && typeof result.application_number === 'string' && result.application_number.length > 0
    : Number.isInteger(result.supplement_id) && result.supplement_id > 0 && Number.isInteger(result.number) && result.number > 0 && ['pending', 'merged'].includes(result.status)
  if (!valid) throw new Error('Ответ о прежней отправке не подтверждён. Новая отправка не выполнена.')
  const current = read(storeKey)
  if (!current || current.key !== state.key) throw new Error(UNKNOWN)
  finishApplicationCommand({ key: state.key, storeKey, session }, 'success')
  return data.result
}
