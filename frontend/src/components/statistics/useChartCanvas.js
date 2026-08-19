import {
  ArcElement,
  BarController,
  BarElement,
  CategoryScale,
  Chart,
  DoughnutController,
  Filler,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from 'chart.js';
import { watch, onBeforeUnmount } from 'vue';

// Chart.js собран по частям: незарегистрированный тип падает в разборе с
// «"bar" is not a registered controller». Регистрируем здесь и только то, чем
// пользуется аналитика, - импорт chart.js/auto тянул бы в сборку все типы
// графиков разом, включая те, которых в системе нет.
Chart.register(
  ArcElement,
  BarController,
  BarElement,
  CategoryScale,
  DoughnutController,
  Filler,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
);

/**
 * Жизненный цикл графика Chart.js на теге canvas.
 *
 * Chart.js рисует императивно и сам за собой не убирает: экземпляр держит
 * слушателя изменения размера и попадает в общий реестр по идентификатору
 * canvas. Не разрушив прежний, при повторном построении получаешь «Canvas is
 * already in use» и утечку слушателей на каждую смену фильтра.
 *
 * Построение идёт после отрисовки (flush: 'post') и следит за самим элементом:
 * canvas живёт под `v-if="hasData"`, поэтому появляется и исчезает не вместе с
 * компонентом, и `onMounted` его не застаёт.
 *
 * @param {import('vue').Ref<HTMLCanvasElement|null>} canvas элемент для рисования
 * @param {import('vue').ComputedRef<object>} config конфигурация Chart.js целиком
 */
export function useChartCanvas(canvas, config) {
  let chart = null;

  function destroy() {
    if (!chart) return;
    chart.destroy();
    chart = null;
  }

  function draw() {
    destroy();
    if (!canvas.value) return;
    chart = new Chart(canvas.value, config.value);
  }

  watch([canvas, config], draw, { immediate: true, flush: 'post' });
  onBeforeUnmount(destroy);
}

/**
 * Цвет с заданной прозрачностью для градиентной заливки.
 *
 * Принимает шестнадцатеричную запись, которой пользуются вызывающие компоненты
 * (`color: '#4F5BDF'`). Незнакомую запись возвращает как есть: подмешать к ней
 * прозрачность нельзя, но и ронять график из-за оформления незачем.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @param {number} alpha прозрачность от 0 до 1
 * @returns {string}
 */
export function withAlpha(color, alpha) {
  const parts = parseHex(color);
  if (!parts) return String(color ?? '').trim();
  return `rgba(${parts.join(', ')}, ${alpha})`;
}

/**
 * Цвет, подмешанный к белому, - подсветка сегмента под курсором.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @param {number} amount доля белого от 0 до 1
 * @returns {string}
 */
export function lighten(color, amount) {
  const parts = parseHex(color);
  if (!parts) return String(color ?? '').trim();
  const mixed = parts.map((c) => Math.round(c + (255 - c) * amount));
  return `rgb(${mixed.join(', ')})`;
}

/**
 * Составляющие цвета из шестнадцатеричной записи.
 *
 * @param {string} color цвет в записи #rgb или #rrggbb
 * @returns {number[]|null} null для незнакомой записи
 */
function parseHex(color) {
  const hex = String(color ?? '').trim();
  const short = /^#([\da-f])([\da-f])([\da-f])$/i.exec(hex);
  const full = /^#([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i.exec(hex);
  if (!short && !full) return null;
  return short
    ? short.slice(1).map((c) => parseInt(c + c, 16))
    : full.slice(1).map((c) => parseInt(c, 16));
}

/**
 * Вертикальный градиент от насыщенного верха к прозрачному низу области графика.
 *
 * Отдаётся Chart.js вычисляемым значением: на первом проходе разметки области
 * ещё нет, и градиент строить не из чего - тогда заливки просто нет, а на
 * следующем проходе она появляется.
 *
 * @param {string} color базовый цвет ряда
 * @param {number} [from=0.32] прозрачность у верхней кромки
 * @param {number} [to=0.02] прозрачность у нижней
 * @returns {(ctx: object) => CanvasGradient|string}
 */
export function verticalGradient(color, from = 0.32, to = 0.02) {
  return ({ chart }) => {
    const area = chart?.chartArea;
    if (!area) return 'transparent';
    const gradient = chart.ctx.createLinearGradient(0, area.top, 0, area.bottom);
    gradient.addColorStop(0, withAlpha(color, from));
    gradient.addColorStop(1, withAlpha(color, to));
    return gradient;
  };
}

/** Оформление осей и подсказки, общее для всех графиков аналитики. */
export const AXIS_LABEL = { color: '#a2a2a2', font: { size: 11 } };
export const GRID_COLOR = '#eef0f7';
export const TOOLTIP_STYLE = {
  backgroundColor: 'rgba(30, 30, 40, 0.92)',
  titleColor: '#ffffff',
  bodyColor: '#ffffff',
  padding: 10,
  cornerRadius: 6,
  displayColors: true,
};

/**
 * Точка ряда под курсором: цвет ряда в белом кольце.
 *
 * Без явных цветов Chart.js берёт их у самого ряда - обводку из `borderColor`
 * линии, заливку из `backgroundColor`, то есть из полупрозрачного градиента
 * области. Точка выходит того же цвета, что линия под ней, и на графике её не
 * видно. Белое кольцо отбивает её от линии в любой палитре ряда; так же
 * рисовал маркер прежний движок.
 *
 * @param {string} color цвет ряда
 * @returns {object} часть описания набора данных Chart.js
 */
export function hoverPointStyle(color) {
  return {
    pointHoverRadius: 6,
    pointHoverBorderWidth: 3,
    pointHoverBackgroundColor: color,
    pointHoverBorderColor: '#ffffff',
  };
}
