import { apiRequest } from './client';

/**
 * API подсказок по справочникам организаций и компаний (#1437).
 * unwrap бросает на !res.ok с сообщением бэка (fallback - если тела нет),
 * чтобы 4xx не прошёл как молчаливый успех.
 */

async function unwrap(res, fallback) {
  const body = await res.json();
  if (!res.ok) throw new Error(body?.message || fallback);
  return body;
}

/**
 * Близкие к query проверенные организации (максимум пять). Эндпоинт закрыт правом
 * application.organization.override - вызывать только тем, кому разрешён ручной ввод.
 */
export async function suggestOrganizations(query) {
  const res = await apiRequest(`/organizations/suggest?q=${encodeURIComponent(query)}`);
  return unwrap(res, 'Не удалось загрузить подсказки организаций');
}

/** Близкие к query проверенные компании, зеркало suggestOrganizations. */
export async function suggestCompanies(query) {
  const res = await apiRequest(`/companies/suggest?q=${encodeURIComponent(query)}`);
  return unwrap(res, 'Не удалось загрузить подсказки компаний');
}
