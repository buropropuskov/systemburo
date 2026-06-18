import { apiRequest } from './client';

/**
 * API статуса онбординг-тура (per-user, #657). Источник правды - бэкенд:
 * хранит версию тура, которую прошёл юзер, чтобы статус переживал смену
 * браузера/устройства, а подъём ONBOARDING_VERSION показывал тур заново.
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
 * @returns {Promise<{ completed_version: number | null }>}
 */
export async function getOnboardingStatus() {
  const res = await apiRequest('/onboarding');
  return unwrap(res, 'Не удалось загрузить статус обучения');
}

/**
 * @param {number} version версия пройденного тура (>= 1)
 * @returns {Promise<{ message: string }>}
 */
export async function markOnboardingComplete(version) {
  const res = await apiRequest('/onboarding/complete', {
    method: 'POST',
    body: JSON.stringify({ version }),
  });
  return unwrap(res, 'Не удалось сохранить статус обучения');
}
