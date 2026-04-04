const API_BASE = 'http://localhost:8080';

/**
 * @returns {Promise<Array<{id: number, key: string, value: string, type: string}>>}
 */
export async function getSettings() {
  const token = localStorage.getItem('token');
  const response = await fetch(`${API_BASE}/settings`, {
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Ошибка загрузки настроек: ${response.status}`);
  }

  return response.json();
}

/**
 * @param {string} key
 * @param {string} value
 * @returns {Promise<Object>}
 */
export async function updateSetting(key, value) {
  const token = localStorage.getItem('token');
  const response = await fetch(`${API_BASE}/settings/${key}`, {
    method: 'PUT',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ value: String(value) }),
  });

  if (!response.ok) {
    throw new Error(`Ошибка сохранения настройки: ${response.status}`);
  }

  return response.json();
}

/**
 * @returns {Promise<{max_file_size: number, allowed_image_types: string, allowed_doc_types: string}>}
 */
export async function getUploadSettings() {
  const token = localStorage.getItem('token');
  const response = await fetch(`${API_BASE}/settings/upload`, {
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Ошибка загрузки настроек загрузки: ${response.status}`);
  }

  return response.json();
}
