import { apiRequest, apiRequestRaw } from './client';

/**
 * Получить сводку метрик за период.
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<{
 *   total_applications: number,
 *   by_attachment_type: Array<{name: string, count: number}>,
 *   by_status: Array<{status: string, count: number}>,
 *   processed: number,
 *   in_work: number,
 *   cars_entered: number,
 *   avg_cars_per_day: number,
 *   people_entered: number,
 *   items_sum: number,
 *   cars_on_territory: number,
 *   people_on_territory: number,
 *   users_online: number,
 *   active_users: number,
 *   banned_users: number,
 *   open_feedback: number,
 *   active_unload_places: number,
 *   blacklist_cars: number,
 *   blacklist_people: number,
 *   unique_cars: number,
 *   unique_people: number,
 * }>}
 */
export async function getSummary(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/summary?${params}`);
  return res.json();
}

/**
 * Получить сводку обработки заявок за период — бандл curated-вкладки «Обработка
 * заявок» одним запросом (#1240).
 *
 * Формы значений, которые нельзя путать (контракт движка отчётов):
 * - длительности (`avg`/`p90`/`prev_avg`/`avg_response_time`/`avg_processing_time`)
 *   приходят в СЕКУНДАХ и показываются через formatReportCell(value, 'duration');
 * - доли (`quality[].value`) приходят готовой дробью в единицах `unit` ('%', 'раз/заявку').
 *
 * `null` в любом значении — «нет данных» (этап не прошла ни одна заявка периода,
 * согласующий не отдал голоса), а НЕ ноль: рисовать прочерк, `?? 0` неверен.
 * Признак наличия данных у этапа — `samples` (0 -> avg/p90 всегда null), у долей —
 * `total_applications`.
 *
 * `trend.direction` — куда двинулось значение (up/down/flat), `trend.sentiment` —
 * как это читать (good/bad/neutral): для времени и отказов рост это ухудшение,
 * поэтому цвет берётся из sentiment, а не из direction. trend отсутствует, если
 * сравнивать не с чем.
 *
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<{
 *   from: string,
 *   to: string,
 *   total_applications: number,
 *   stages: Array<{key: string, label: string, samples: number, avg: number|null, p90: number|null, prev_avg: number|null, trend?: {delta_pct: number, direction: 'up'|'down'|'flat', sentiment: 'good'|'bad'|'neutral'}}>,
 *   quality: Array<{key: string, label: string, unit?: string, value: number|null, prev_value: number|null, trend?: {delta_pct: number, direction: 'up'|'down'|'flat', sentiment: 'good'|'bad'|'neutral'}}>,
 *   slow_approvers: Array<{name: string, avg_response_time: number|null, votes_count: number}>,
 *   approvers: Array<{name: string, avg_response_time: number|null, votes_count: number}>,
 *   acceptors: Array<{name: string, avg_acceptance_time: number|null, accepts_count: number}>,
 *   by_organization: Array<{label: string, avg_processing_time: number|null, applications_count: number}>,
 * }>}
 */
export async function getProcessingSummary(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/processing-summary?${params}`);
  if (!res.ok) {
    // На 5xx baseRequest редиректит на /500 и тела JSON не отдаёт — парсим
    // защищённо, чтобы не словить SyntaxError на HTML-странице ошибки.
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось загрузить сводку обработки заявок');
  }
  return res.json();
}

/**
 * Получить список зависших согласований (#1315 S4): живые заявки, ждущие решения
 * согласующего дольше настроенного порога молчания. Снимок текущего состояния, а
 * не агрегат за период — от дат вкладки не зависит (эндпоинт их не принимает).
 * @returns {Promise<Array<{application_id: number, application_number: string, approver_name: string, waiting_days: number, reminder_count: number, last_reminder_at: string|null}>>}
 */
export async function getStuckApprovals() {
  const res = await apiRequest('/statistics/stuck-approvals');
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось загрузить зависшие согласования');
  }
  return res.json();
}

/**
 * Получить журнал обработки — сквозную ленту согласований и принятий за период по
 * времени убыванием (#1251 S4/S7).
 * @param {string} from YYYY-MM-DD
 * @param {string} to YYYY-MM-DD
 * @param {number} [limit] размер страницы (бэк клампит: по умолчанию 50, максимум 200)
 * @param {number} [offset] смещение от начала ленты (постраничная навигация, #1251 P5b)
 * @param {{role?: 'approval'|'acceptance'|'', q?: string}} [filter] отбор ленты (#1251 P5c):
 *   роль события и подстрока номера заявки или ФИО актора. Фильтры входят и в meta.total.
 * @returns {Promise<{items: Array<{application_id: number, application_number: string, actor_name: string, role: 'approval'|'acceptance', occurred_at: string, working_seconds: number|null}>, meta: {total: number, page: number, per_page: number}}>}
 */
export async function getProcessingJournal(from, to, limit, offset, filter = {}) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (limit) params.set('limit', String(limit));
  if (offset) params.set('offset', String(offset));
  // role: approval | acceptance (иное бэк отбивает 400), q — подстрока номера
  // заявки или ФИО актора. Фильтры учитываются и в meta.total.
  if (filter.role) params.set('role', filter.role);
  if (filter.q) params.set('q', filter.q);
  // Сырой ответ: общее число событий лежит в envelope.meta рядом с data, а apiRequest
  // снимает только data и meta теряется (см. getApplicationsPaginated).
  const res = await apiRequestRaw(`/statistics/processing-journal?${params}`);
  const body = await res.json().catch(() => null);
  if (!res.ok || !body || !body.success) {
    throw new Error(body?.error || 'Не удалось загрузить журнал обработки');
  }
  return {
    items: body.data || [],
    meta: body.meta || { total: 0, page: 1, per_page: limit || 50 },
  };
}

/**
 * Получить временной ряд метрики.
 * @param {{from: string, to: string, metric: string, granularity: string}} opts
 *   metric: 'applications' | 'people_entries' | 'car_entries'
 *   granularity: 'day' | 'week' | 'month'
 * @returns {Promise<Array<{date: string, count: number}>>}
 */
export async function getTimeline({ from, to, metric, granularity }) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (metric) params.set('metric', metric);
  if (granularity) params.set('granularity', granularity);
  const res = await apiRequest(`/statistics/timeline?${params}`);
  return res.json();
}

/**
 * Получить серию дневных пиков онлайна пользователей за период.
 * Потребитель — карточка динамики онлайна на дашборде (срез G3b); там форма
 * {date, peak} мапится в {timestamp, count} под AnalyticsAreaChart, как chartData.
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<Array<{date: string, peak: number}>>} по возрастанию даты
 */
export async function getOnlinePeaks(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/online-peaks?${params}`);
  return res.json();
}

/**
 * Получить список пользователей онлайн (активность за окно онлайна), по убыванию
 * свежести last_seen. Потребитель — модалка «кто онлайн» по клику на плитку дашборда.
 * @returns {Promise<Array<{
 *   id: number,
 *   login: string,
 *   full_name: string,
 *   role: string,
 *   user_type: string,
 *   last_seen: string,
 * }>>}
 */
export async function getOnlineUsers() {
  const res = await apiRequest('/statistics/online-users');
  return res.json();
}

/**
 * Получить последние проходы/проезды.
 * @param {number} [limit=15] - максимальное количество записей на тип
 * @returns {Promise<{
 *   people: Array<{subject: string, organization: string, place: string, created_at: string, action_type: 'entry'|'exit'}>,
 *   cars:   Array<{subject: string, mark: string, organization: string, place: string, created_at: string, action_type: 'entry'|'exit'}>,
 * }>}
 */
export async function getRecentPassages(limit = 15) {
  const res = await apiRequest(`/statistics/recent-passages?limit=${limit}`);
  return res.json();
}

/**
 * Получить готовые инсайты за период: пик нагрузки по часам, сравнение с
 * предыдущим периодом равной длины, топ мест и организаций, тренды по дням.
 * @param {string} from - дата начала в формате YYYY-MM-DD
 * @param {string} to   - дата конца в формате YYYY-MM-DD
 * @returns {Promise<{
 *   peak_hours: Array<{metric: string, label: string, unit?: string, peak_hour: number, peak_value: number, hourly: Array<{hour: number, value: number}>}>,
 *   comparisons: Array<{metric: string, label: string, unit?: string, current: number, previous: number, delta_pct: number, direction: 'up'|'down'|'flat'}>,
 *   top_places: Array<{metric: string, label: string, value: number}>,
 *   top_orgs: Array<{metric: string, label: string, value: number}>,
 *   trends: Array<{metric: string, label: string, direction: 'up'|'down'|'flat', series: number[]}>,
 * }>}
 */
export async function getInsights(from, to) {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const res = await apiRequest(`/statistics/insights?${params}`);
  if (!res.ok) {
    // На 5xx baseRequest уже редиректит на /500 и не отдаёт JSON-тело —
    // парсим защищённо, чтобы не словить SyntaxError на HTML-странице ошибки.
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось загрузить инсайты');
  }
  return res.json();
}

/**
 * Получить каталог конструктора отчётов: whitelist метрик, разрезов, фильтров
 * и list-сущностей с подставленными значениями динамических справочников.
 * @returns {Promise<{
 *   metrics: Array<{key: string, label: string, unit?: string, dimensions: string[]}>,
 *   dimensions: Array<{key: string, label: string}>,
 *   filters: Array<{key: string, label: string, type: 'date'|'enum'|'dict', options?: Array<{value: string, label: string}>}>,
 *   list_entities: Array<{key: string, label: string, columns: Array<{key: string, label: string}>, filters: string[]}>,
 *   granularities: Array<{value: string, label: string}>,
 * }>}
 */
export async function getReportCatalog() {
  const res = await apiRequest('/statistics/metrics');
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось загрузить каталог отчётов');
  return data;
}

/**
 * Исполнить отчёт конструктора.
 * @param {{
 *   mode: 'aggregate'|'list', metric?: string, dimension?: string, granularity?: string,
 *   entity?: string, filters?: Array<{key: string, values?: string[], from?: string, to?: string}>,
 *   sort?: string, limit?: number
 * }} request
 * @returns {Promise<object>} ReportResponse (aggregate) или ReportListResponse (list)
 */
export async function runReport(request) {
  const res = await apiRequest('/statistics/report', {
    method: 'POST',
    body: JSON.stringify(request),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось построить отчёт');
  return data;
}

/**
 * Границы дат, доступные отчёту: самая ранняя и самая поздняя дата по его оси
 * времени - той же, по которой сужает фильтр периода. Пустые строки - данных нет
 * или у отчёта нет оси (например, список машин).
 *
 * @param {{mode?: 'aggregate'|'list', metric?: string, entity?: string}} params
 * @returns {Promise<{from: string, to: string}>}
 */
export async function getReportDataPeriod(params = {}) {
  const query = new URLSearchParams();
  for (const key of ['mode', 'metric', 'entity']) {
    if (params[key]) query.set(key, params[key]);
  }
  const res = await apiRequest(`/statistics/report/period?${query.toString()}`);
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось определить границы дат');
  return data;
}

/**
 * @typedef {object} ReportTemplate
 * @property {number} id
 * @property {string} name
 * @property {string} [description]
 * @property {object} config   снимок состояния конструктора
 * @property {boolean} is_system
 * @property {boolean} is_shared
 * @property {number} [owner_user_id]
 */

/**
 * Список доступных шаблонов отчётов: системные пресеты + личные + расшаренные.
 * @returns {Promise<ReportTemplate[]>}
 */
export async function getReportTemplates() {
  const res = await apiRequest('/statistics/templates');
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось загрузить шаблоны');
  return data;
}

/**
 * Сохранить личный шаблон отчёта.
 * @param {{ name: string, description?: string, config: object, is_shared?: boolean }} payload
 * @returns {Promise<ReportTemplate>}
 */
export async function saveReportTemplate(payload) {
  const res = await apiRequest('/statistics/templates', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data?.message || 'Не удалось сохранить шаблон');
  return data;
}

/**
 * Удалить личный шаблон отчёта.
 * @param {number} id
 * @returns {Promise<void>}
 */
export async function deleteReportTemplate(id) {
  const res = await apiRequest(`/statistics/templates/${id}`, { method: 'DELETE' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data?.message || 'Не удалось удалить шаблон');
  }
}
