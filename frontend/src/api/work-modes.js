import { apiRequest } from './client';

/**
 * API-клиент модалки «Режимы работы» (C3).
 * Read-only агрегатор расписаний Бюро, мест разгрузки и мест прохода (C2).
 */

/**
 * Режимы работы всех объектов в единой форме слота.
 * GET /api/work-modes -> envelope -> { bureau, unload_places[], checkpoints[] }.
 * Каждый объект: { id, kind, name, status, current_status, time_slots[] },
 * слот: { day_of_week (0=Пн..6=Вс), open_time, close_time, is_next_day, is_active }.
 * @returns {Promise<{bureau: object, unload_places: object[], checkpoints: object[]}>}
 */
export async function getWorkModes() {
  const res = await apiRequest('/work-modes');
  return res.json();
}
