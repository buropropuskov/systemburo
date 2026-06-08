import { apiRequest } from './client';

export async function createEmployee(data) {
  const res = await apiRequest('/employees', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return res.json();
}

export async function getActiveEmployeesForTable(tableId) {
  const res = await apiRequest(`/employees/active-for-table/${tableId}`);
  return res.json();
}

/**
 * Поиск сотрудника в реестре по ФИО (для открытия карточки со страницы ЧС).
 * Возвращает запись unique_employee или null, если совпадения нет (404).
 */
export async function lookupUniqueEmployee({ last_name, first_name, middle_name = '' }) {
  const qs = new URLSearchParams({ last_name, first_name, middle_name: middle_name || '' });
  const res = await apiRequest(`/unique-employees/lookup?${qs.toString()}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Ошибка поиска сотрудника (${res.status})`);
  return res.json();
}
