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
 * Масштаб ограничен И по ширине (1440), И по высоте (REFERENCE_HEIGHT): zoom
 * равномерный, поэтому увеличение по ширине уменьшает layout-высоту (2560x1440
 * при zoom по ширине 1.78 даёт layout-высоту 810px - меню и контент перестают
 * помещаться вертикально). Берём меньший из двух коэффициентов, чтобы эталонный
 * кадр 1440x900 помещался целиком (на 2560x1440 zoom = 1.6, layout 1600x900).
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
// Эталонная высота кадра: ограничивает масштаб по вертикали, чтобы меню/контент
// помещались на широких, но невысоких мониторах (2560x1440 -> zoom 1.6, а не 1.78).
const REFERENCE_HEIGHT = 900
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
export function computeZoom(width, height) {
  if (width <= REFERENCE_WIDTH) return 1 // узкие - обычная адаптивная вёрстка
  // Масштаб = меньший из коэффициентов по ширине и высоте (чтобы кадр 1440x900
  // помещался целиком), но не меньше 1: на низких широких экранах не сжимаем UI,
  // оставляем ширину как есть.
  const z = Math.min(width / REFERENCE_WIDTH, height / REFERENCE_HEIGHT, MAX_ZOOM)
  return z < 1 ? 1 : z
}

/**
 * Пересчитывает и применяет масштаб по текущей ширине окна. Экспортирована для
 * теста DOM-эффектов (установка/сброс zoom, порог дребезга).
 */
export function updateViewportZoom() {
  scheduled = false
  const next = computeZoom(window.innerWidth, window.innerHeight)
  // --app-vh: 1% ЗУМЛЕННОЙ высоты вьюпорта в CSS-px (innerHeight/zoom/100).
  // Замена для vh под zoom: сам vh считается от НЕзумленной высоты и завышает
  // высоты в zoom раз (fixed height:100vh уезжает под экран, min-height:Nvh
  // раздувается). В стилях использовать calc(var(--app-vh, 1vh) * N) вместо Nvh.
  document.documentElement.style.setProperty('--app-vh', (window.innerHeight / next / 100) + 'px')
  // Порог против дребезга на субпиксельных ресайзах.
  if (Math.abs(next - appliedZoom) < 0.005) return
  appliedZoom = next
  // zoom=1 -> снимаем свойство, чтобы не оставлять артефакт в вычисленных стилях
  // (и чтобы на ширинах <=1440 корень был чист).
  document.documentElement.style.zoom = next === 1 ? '' : String(Number(next.toFixed(4)))
}

/**
 * Текущий применённый масштаб. Нужен JS-расчётам высоты «до низа экрана»:
 * getBoundingClientRect() под zoom возвращает device-px, а window.innerHeight -
 * НЕзумленную высоту; чтобы получить CSS-высоту, (innerHeight - rect.top) надо
 * делить на этот zoom (иначе элемент выходит в zoom раз выше экрана).
 */
export function getViewportZoom() {
  return appliedZoom
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
