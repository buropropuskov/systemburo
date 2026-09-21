import { apiRequest } from './client';

/**
 * Справочник списком: отдаёт массив, а на отказе - пустой, чтобы экран строился без него.
 *
 * Такие загрузки писались в компонентах руками, с try/catch и console.error на каждую;
 * в таблице машин их было три подряд, и каждая занимала по семь строк.
 */
export async function fetchJson(path, fallback = []) {
  try {
    const res = await apiRequest(path);
    return res.ok ? await res.json() : fallback;
  } catch {
    return fallback;
  }
}
