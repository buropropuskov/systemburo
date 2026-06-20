/**
 * Сборка тела запроса POST /statistics/report из состояния конструктора отчётов.
 *
 * Период передаётся фильтром date_range{from,to} — движок (и aggregate, и list)
 * сам спец-кейсит его по своей tsColumn. date_range добавляется только если он
 * применим к выбранному срезу: для list — входит ли в entity.filters (у машин и
 * людей его нет, движок вернёт 400), для aggregate — всегда (у каждой метрики есть
 * временная колонка). dict/enum-значения попадают только по применимым ключам;
 * пустые значения отбрасываются — пустой фильтр движок и так пропускает.
 *
 * @param {{
 *   mode: 'aggregate'|'list',
 *   metric?: string, metrics?: string[], dimension?: string, granularity?: string,
 *   pivot?: string, entity?: string, sort?: string, limit?: number|string|null,
 *   filters?: Record<string, string[]>
 * }} state
 * @param {{from?: string, to?: string}} [period]
 * @param {string[]} [applicableFilters] — ключи фильтров, валидные для текущего среза
 * @returns {object} тело запроса ReportRequest
 */
export function buildReportRequest(state, period = {}, applicableFilters = []) {
  const allowed = new Set(applicableFilters);
  const filters = [];

  const from = period.from || '';
  const to = period.to || '';
  if (allowed.has('date_range') && (from || to)) {
    filters.push({ key: 'date_range', from, to });
  }

  for (const [key, values] of Object.entries(state.filters || {})) {
    if (key === 'date_range' || !allowed.has(key)) continue;
    const clean = (values || []).filter((v) => v != null && String(v).trim() !== '');
    if (clean.length) filters.push({ key, values: clean });
  }

  const limit = normalizeLimit(state.limit);

  if (state.mode === 'list') {
    return { mode: 'list', entity: state.entity || '', filters, limit };
  }

  // Мультивыбор метрик: каждая метрика -> своя колонка результата (движок отдаёт
  // metrics-приоритет над metric). Одиночный metric оставлен для пресетов и
  // обратной совместимости — шлётся, только когда metrics пуст.
  const metrics = (Array.isArray(state.metrics) ? state.metrics : [])
    .filter((m) => m != null && String(m).trim() !== '');

  const req = {
    mode: 'aggregate',
    dimension: state.dimension || '',
    filters,
    limit,
  };
  if (metrics.length) req.metrics = metrics;
  else req.metric = state.metric || '';
  // Гранулярность и cross-tab-ось осмысленны только для временного разреза. Pivot
  // разворачивает значения оси в колонки; бэк валидирует ось против метрик.
  if (state.dimension === 'period') {
    if (state.granularity) req.granularity = state.granularity;
    if (state.pivot) req.pivot = state.pivot;
  }
  if (state.sort) req.sort = state.sort;
  return req;
}

/**
 * Нормализует лимит строк: пусто/невалидно -> 0 (бэк применит дефолт 100),
 * иначе целое в диапазоне [1, 1000].
 * @param {number|string|null|undefined} limit
 * @returns {number}
 */
function normalizeLimit(limit) {
  const n = Number(limit);
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.min(Math.floor(n), 1000);
}
