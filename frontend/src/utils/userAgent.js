/**
 * Грубый разбор User-Agent на браузер и ОС для колонки «Устройство» в истории
 * входов. Не претендует на точность полноценных UA-парсеров - покрывает частые
 * случаи, остальное отдаёт пустыми строками (в UI тогда показывается сырой UA).
 * @param {string} ua
 * @returns {{ browser: string, os: string }}
 */
export function parseUserAgent(ua) {
  if (!ua || typeof ua !== 'string') return { browser: '', os: '' }
  return { browser: detectBrowser(ua), os: detectOs(ua) }
}

// Порядок важен: Edge/Opera/Яндекс содержат в UA подстроку Chrome, Chrome содержит
// Safari - более специфичные проверки идут первыми.
function detectBrowser(ua) {
  if (/Edg\//.test(ua)) return 'Edge'
  if (/OPR\/|Opera/.test(ua)) return 'Opera'
  if (/YaBrowser/.test(ua)) return 'Яндекс.Браузер'
  if (/Chrome\//.test(ua)) return 'Chrome'
  if (/Firefox\//.test(ua)) return 'Firefox'
  if (/Version\/[\d.]+ Safari/.test(ua)) return 'Safari'
  return ''
}

function detectOs(ua) {
  // Windows NT 10 покрывает и Windows 10, и 11 - UA их не различает.
  if (/Windows NT 10/.test(ua)) return 'Windows 10/11'
  if (/Windows NT/.test(ua)) return 'Windows'
  if (/iPhone|iPad|iPod/.test(ua)) return 'iOS'
  if (/Android/.test(ua)) return 'Android'
  if (/Mac OS X/.test(ua)) return 'macOS'
  if (/Linux/.test(ua)) return 'Linux'
  return ''
}

/**
 * Короткая человекочитаемая метка устройства из User-Agent: «Chrome · Windows 10/11».
 * Если распознать не удалось - вернёт исходный UA (обрезанный) или прочерк.
 * @param {string} ua
 * @returns {string}
 */
export function formatDevice(ua) {
  const { browser, os } = parseUserAgent(ua)
  if (browser && os) return `${browser} · ${os}`
  if (browser) return browser
  if (os) return os
  return ua ? ua.slice(0, 40) : '—'
}
