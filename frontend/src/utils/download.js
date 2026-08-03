import { API_BASE_URL } from '@/api/client';

/**
 * Извлекает имя файла из заголовка Content-Disposition: сначала RFC 5987
 * filename* (UTF-8 percent-encoded, для кириллицы), затем базовый filename,
 * иначе fallback. Порядок важен - filename* приоритетнее ASCII-фолбэка.
 * @param {string} cd  значение заголовка Content-Disposition (может быть пустым)
 * @param {string} fallback  имя по умолчанию, если заголовок пуст/без имени
 * @returns {string}
 */
export function parseContentDispositionFilename(cd, fallback) {
  const header = cd || '';
  const utf8Match = header.match(/filename\*=UTF-8''(.+)/i);
  if (utf8Match) return decodeURIComponent(utf8Match[1]);
  const basicMatch = header.match(/filename="?([^";]+)"?/);
  return basicMatch ? basicMatch[1] : fallback;
}

/**
 * Запускает скачивание файла по одноразовому билету (образец -
 * internal/realtime/tickets.go): билет сам несёт авторизацию, поэтому эндпоинт
 * публичный и браузер может скачать его прямой навигацией - без fetch+blob,
 * что важно для потокового ZIP файлового архива (гигабайты не буферизуются
 * в память вкладки, Content-Disposition с именем отдаёт сервер).
 * @param {string} path  путь эндпоинта без /api и без query, например '/file-archive/download'
 * @param {string} ticket  одноразовый билет, полученный отдельным защищённым запросом
 */
export function startTicketDownload(path, ticket) {
  const url = `${API_BASE_URL}${path}?ticket=${encodeURIComponent(ticket)}`;
  const a = document.createElement('a');
  a.href = url;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

/**
 * Переводит байты в привычные единицы (Б/КБ/МБ/ГБ) - те же ступени и округление,
 * что у humanBytes в cmd/server/cleanup.go, чтобы CLI-отчёт и вкладка «Файловый
 * архив» показывали одинаковые числа. Расходится только край: ноль здесь «0 Б»
 * (пустой архив - законное состояние, а не отсутствие данных), а мусорный ввод
 * даёт прочерк. Единственная реализация на фронте: размеры файлов в разделах
 * документов и руководства считаются здесь же.
 * @param {number} bytes
 * @returns {string}
 */
export function formatBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes < 0) return '—';
  if (bytes === 0) return '0 Б';
  if (bytes < 1024) return `${bytes} Б`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} КБ`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} ГБ`;
}
