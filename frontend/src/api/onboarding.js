import { apiRequest } from './client';
import { resolveFactTableRoute } from '@/components/onboarding/securityOnboardingSteps';

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

/**
 * Резолв route фактовой таблицы для security-тура: тянет `/system-tables` (тот же
 * список, что NavMenu в дропдауне «Таблицы») и выбирает первую активную фактовую
 * таблицу машин. Сетевая ошибка/отсутствие подходящей таблицы -> null (сегмент
 * отметки в туре просто не добавляется).
 *
 * @returns {Promise<string|null>} `/table/<name>` или null
 */
export async function getSecurityFactRoute() {
  try {
    const res = await apiRequest('/system-tables');
    if (!res.ok) return null;
    const tables = await res.json();
    return resolveFactTableRoute(tables);
  } catch {
    return null;
  }
}

/**
 * Сброс прохождения тура пользователю (админ): после сброса у него снова
 * сработает автозапуск при входе.
 *
 * @param {string} username
 * @returns {Promise<{ message: string }>}
 */
export async function resetOnboardingForUser(username) {
  const res = await apiRequest(`/users/${encodeURIComponent(username)}/onboarding/reset`, {
    method: 'POST',
  });
  return unwrap(res, 'Не удалось сбросить обучение');
}
