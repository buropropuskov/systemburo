import { apiRequest } from './client';
import { usePermissionsStore } from '@/stores/permissions';
import { resolveSecurityPostContext } from '@/components/onboarding/securityFactSteps';

/**
 * API статуса онбординг-туров (per-user, per-tour). Источник правды - бэкенд:
 * хранит по каждому туру версию, которую прошёл юзер, чтобы статус переживал смену
 * браузера/устройства, а подъём версии КОНКРЕТНОГО тура показывал заново только его.
 *
 * apiRequest разворачивает envelope: на успехе res.json() отдаёт data,
 * на ошибке - { message }. res.ok проверяем явно и бросаем Error.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Пройденные версии по всем турам. Ключ отсутствует или null = тур не проходили.
 *
 * @returns {Promise<{ completed: Record<string, number|null>, finished: string[] }>}
 */
export async function getOnboardingStatus() {
  const res = await apiRequest('/onboarding');
  return unwrap(res, 'Не удалось загрузить статус обучения');
}

/**
 * @param {string} tour ключ тура из реестра (tours.js)
 * @param {number} version версия пройденного тура (>= 1)
 * @param {boolean} [finished] тур доведён до финала (а не закрыт на середине)
 * @returns {Promise<{ message: string }>}
 */
export async function markOnboardingComplete(tour, version, finished = false) {
  const res = await apiRequest('/onboarding/complete', {
    method: 'POST',
    body: JSON.stringify({ tour, version, finished }),
  });
  return unwrap(res, 'Не удалось сохранить статус обучения');
}

/** Compatibility route wrapper: known empty posts are null; failures reject. */
export async function getSecurityFactRoute() {
  return (await getSecurityPostContext())?.route || null;
}

/** Known empty posts are null; transport/contract/permission readiness errors reject. */
export async function getSecurityPostContext() {
  const permissions = usePermissionsStore();
  if (!permissions.loaded) throw new Error('Learning access is not ready');
  const res = await apiRequest('/system-tables');
  const tables = await unwrap(res, 'Не удалось загрузить таблицы обучения');
  if (!permissions.loaded) throw new Error('Learning access is not ready');
  if (!Array.isArray(tables)) throw new Error('Unknown learning tables');
  return resolveSecurityPostContext(tables, name => permissions.hasPermission(`table.${name}.view`));
}

/**
 * Сброс прохождения тура пользователю (админ): после сброса у него снова
 * сработает автозапуск при входе.
 *
 * @param {string} username
 * @param {string} [tour] ключ тура; без него сбрасываются все туры пользователя
 * @returns {Promise<{ message: string }>}
 */
export async function resetOnboardingForUser(username, tour) {
  const res = await apiRequest(`/users/${encodeURIComponent(username)}/onboarding/reset`, {
    method: 'POST',
    body: JSON.stringify(tour ? { tour } : {}),
  });
  return unwrap(res, 'Не удалось сбросить обучение');
}
