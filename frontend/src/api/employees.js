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
