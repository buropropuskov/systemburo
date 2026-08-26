import { apiRequest } from './client';
import { usePermissionsStore } from '@/stores/permissions';
import { resolveFactTableRoute } from '@/components/onboarding/securityFactSteps';

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

/**
 * Резолв route фактовой таблицы для тура охраны: тянет `/system-tables` (тот же
 * список, что NavMenu показывает в дропдауне «Таблицы») и выбирает первую активную
 * фактовую таблицу - машин, а если таких нет, то людей. Сетевая ошибка/отсутствие
 * подходящей таблицы -> null (сегмент отметки в туре просто не добавляется).
 *
 * Список приходит целиком, поэтому доступность отсеиваем сами тем же правом, что
 * стережёт роут таблицы (`table.<name>.view`) и по которому NavMenu строит свой
 * дропдаун - иначе тур повёл бы на таблицу, с которой роут-гард сразу уводит.
 * Пока права не загружены, hasPermission отвечает «нет» на всё: в этом случае
 * предикат не передаём, чтобы не потерять сегмент на пустом кэше прав.
 *
 * @returns {Promise<string|null>} `/table/<name>` или null
 */
export async function getSecurityFactRoute() {
  try {
    const res = await apiRequest('/system-tables');
    if (!res.ok) return null;
    const tables = await res.json();
    const permissions = usePermissionsStore();
    const canView = permissions.loaded
      ? (name) => permissions.hasPermission(`table.${name}.view`)
      : undefined;
    return resolveFactTableRoute(tables, canView);
  } catch {
    return null;
  }
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
