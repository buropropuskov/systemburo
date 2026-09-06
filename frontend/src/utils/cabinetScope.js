/**
 * Вкладка кабинета одним набором параметров - для списка заявок и для чипа
 * «Обновления» разом (#2339).
 *
 * Пока их собирали в двух местах, они разъехались: список сузился до вкладки, а чип
 * продолжал считать по всему скоупу ЛК (свои заявки плюс заявки организации). Чип
 * обещал 11 обновлений, клик по нему открывал 3, и это читалось как потерянные заявки.
 *
 * @param {string} filter выбранная вкладка: 'my' | 'organization'
 * @param {number|null} ownerUserId id владельца кабинета
 * @param {number|null} organizationId id его организации
 * @returns {{sender_user_id?: number, organization_id?: number}}
 */
export function cabinetScopeParams(filter, ownerUserId, organizationId) {
  if (filter === 'my' && ownerUserId) return { sender_user_id: ownerUserId };
  if (filter === 'organization' && organizationId) return { organization_id: organizationId };
  return {};
}
