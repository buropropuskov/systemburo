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
 * Подсказки по наименованию организации: близкие проверенные записи (максимум пять),
 * каноничное оформление введённого текста и признак того, что такое наименование в
 * справочнике уже есть. Эндпоинт закрыт правом application.organization.override -
 * вызывать только тем, кому разрешён ручной ввод.
 *
 * Ответ - { items, canonical, matched }. Канон считает бэк: правила оформления (ОПФ
 * заглавными, заглавная буква названия, парные кавычки) живут в Go, и фронт не должен
 * держать их вторую копию. matched отвечает на «есть ли запись с таким же ключом
 * дедупликации» - по нему форма предупреждает, что наименование уйдёт на проверку.
 */
export async function suggestOrganizations(query) {
  const res = await apiRequest(`/organizations/suggest?q=${encodeURIComponent(query)}`);
  return unwrap(res, 'Не удалось загрузить подсказки организаций');
}

/** Подсказки по наименованию компании, зеркало suggestOrganizations. */
export async function suggestCompanies(query) {
  const res = await apiRequest(`/companies/suggest?q=${encodeURIComponent(query)}`);
  return unwrap(res, 'Не удалось загрузить подсказки компаний');
}

/**
 * Разбор записей «на проверке» (#1437). kind - 'organization' либо 'company';
 * дальше различие только в сегменте пути, тела и ответы у справочников общие.
 * Эндпоинты закрыты правом application.organization.moderate.
 */
const SEGMENT = { organization: 'organizations', company: 'companies' };

function segment(kind) {
  const value = SEGMENT[kind];
  if (!value) throw new Error(`Неизвестный справочник: ${kind}`);
  return value;
}

/**
 * Подтверждает запись: наименование верное, запись остаётся в справочнике.
 * Ответ - { status, entry?, existing?, message? }; status = 'conflict' означает, что
 * такое наименование уже есть в справочнике и запись предлагается привязать к нему.
 */
export async function approveDirectoryEntry(kind, id) {
  const res = await apiRequest(`/${segment(kind)}/${id}/moderation/approve`, { method: 'POST' });
  return unwrap(res, 'Не удалось подтвердить запись справочника');
}

/** Исправляет наименование записи «на проверке»; ответ как у approveDirectoryEntry. */
export async function renameDirectoryEntry(kind, id, name) {
  const res = await apiRequest(`/${segment(kind)}/${id}/moderation/rename`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  });
  return unwrap(res, 'Не удалось исправить наименование');
}

/**
 * Привязывает запись «на проверке» к существующей: ссылки переезжают на неё, черновик
 * удаляется. Ответ - { target, reassigned, dropped_duplicates }.
 */
export async function mergeDirectoryEntry(kind, id, targetId) {
  const res = await apiRequest(`/${segment(kind)}/${id}/moderation/merge`, {
    method: 'POST',
    body: JSON.stringify({ target_id: targetId }),
  });
  return unwrap(res, 'Не удалось привязать запись к существующей');
}

/**
 * Справочник целиком для выбора цели привязки. Отдаёт только проверенные записи:
 * привязка к другому черновику запрещена бэком, показывать такие варианты незачем.
 */
export async function fetchApprovedDirectory(kind) {
  const res = await apiRequest(`/${segment(kind)}`);
  const items = await unwrap(res, 'Не удалось загрузить справочник');
  return (Array.isArray(items) ? items : []).filter((item) => item?.moderation_status !== 'pending');
}
