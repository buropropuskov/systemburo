/**
 * Масштабирование интерфейса под эталонную ширину на больших экранах.
 *
 * На мониторах шире REFERENCE_WIDTH весь UI увеличивается через CSS `zoom` на
 * корне так, что вёрстка раскладывается «как на 1440px» и просто рендерится
 * крупнее: 2560@100% выглядит как 1440@100%. Это программный аналог ручного
 * браузерного зума, который пользователи ставили сами на больших мониторах.
 * Ниже эталона zoom=1 - работает обычная адаптивная вёрстка (брейкпоинты).
 * Выше MAX_ZOOM масштаб не растёт (ультраширокие/мультимонитор), там контент
 * дополнительно ограничен по ширине через --content-max.
 *
 * Почему zoom, а не transform:scale - zoom корректно ведёт себя с
 * position:fixed и не требует ручной компенсации ширины/скролла. Почему не
 * rem-масштаб - в проекте размеры в px, переопределение root font-size на них
 * не влияет. Поддержка: Chrome/Edge, Firefox 126+, Safari; на древних
 * браузерах деградирует в no-op (сохраняется текущее поведение).
 */

// Держать синхронным с --bp-wide в assets/tokens.css (там документная граница
// брейкпоинта, здесь - эталон, под который масштабируем UI).
const REFERENCE_WIDTH = 1440
// Эталон-look держим до ~2880px (2 x 1440), дальше не раздуваем - иначе на
// мультимониторе/ультраширокой панели UI станет неоправданно крупным.
const MAX_ZOOM = 2

let appliedZoom = 1
let scheduled = false

/**
 * window.innerWidth - истинная ширина вьюпорта в устройственных CSS-px и НЕ
 * зависит от CSS `zoom` на корне (проверено: zoom меняет documentElement.
 * clientWidth = layout-viewport, но innerWidth остаётся физическим). Поэтому
 * берём его напрямую - никакой компенсации по applied zoom не нужно, петли нет.
 */
export function computeZoom(width) {
  if (width <= REFERENCE_WIDTH) return 1
  return Math.min(width / REFERENCE_WIDTH, MAX_ZOOM)
}

/**
 * Пересчитывает и применяет масштаб по текущей ширине окна. Экспортирована для
 * теста DOM-эффектов (установка/сброс zoom, порог дребезга).
 */
export function updateViewportZoom() {
  scheduled = false
  const next = computeZoom(window.innerWidth)
  // Порог против дребезга на субпиксельных ресайзах.
  if (Math.abs(next - appliedZoom) < 0.005) return
  appliedZoom = next
  // zoom=1 -> снимаем свойство, чтобы не оставлять артефакт в вычисленных стилях
  // (и чтобы на ширинах <=1440 корень был чист).
  document.documentElement.style.zoom = next === 1 ? '' : String(Number(next.toFixed(4)))
}

function schedule() {
  if (scheduled) return
  scheduled = true
  requestAnimationFrame(updateViewportZoom)
}

/**
 * Инициализирует контроллер масштаба: применяет масштаб сразу и подписывается
 * на изменение размера окна (в т.ч. перенос между мониторами разной ширины).
 * Зовётся один раз из main.js после mount.
 */
export function initViewportScale() {
  if (typeof window === 'undefined') return
  updateViewportZoom()
  window.addEventListener('resize', schedule, { passive: true })
}
