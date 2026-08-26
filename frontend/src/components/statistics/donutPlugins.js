/**
 * Плагины кольцевого графика: доля внутри сегмента и подпись в центре.
 *
 * Chart.js из коробки не подписывает ни сегменты, ни середину кольца, а
 * прежний движок рисовал и то, и другое. Плагины держим отдельным модулем,
 * потому что их логика - расчёт, а не оформление: она проверяется юнитами на
 * поддельном холсте, чего с кодом внутри компонента не сделать.
 */

import { cssVariable } from './useChartCanvas';

/**
 * Минимальный угол сегмента, при котором подпись доли ещё помещается в дугу.
 * Значение перенесено из прежнего движка (minAngleToShowLabel), иначе тонкие
 * доли подписываются друг поверх друга.
 */
const MIN_LABEL_ANGLE = (10 * Math.PI) / 180;

/**
 * Цвет оформления из темы страницы.
 *
 * @param {object} chart экземпляр Chart.js
 * @param {string} name имя переменной темы
 * @param {string} fallback цвет для окружения без темы
 * @returns {string}
 */
function themed(chart, name, fallback) {
  return cssVariable(chart?.canvas, name, fallback);
}

const FALLBACK_FONT = "'Montserrat', sans-serif";

/**
 * Семейство шрифта, которым подписан холст.
 *
 * Прежний движок рисовал подписи в SVG и наследовал шрифт страницы; холст
 * наследования не знает и требует явную строку, поэтому читаем её с самого
 * элемента, а вне браузера берём фирменный шрифт проекта.
 *
 * @param {object} chart экземпляр Chart.js
 * @returns {string}
 */
function fontFamily(chart) {
  const canvas = chart?.canvas;
  const family = canvas?.ownerDocument?.defaultView?.getComputedStyle?.(canvas)?.fontFamily;
  return family || FALLBACK_FONT;
}

/**
 * Значения сегментов, не скрытых кликом по легенде.
 *
 * @param {object} chart экземпляр Chart.js
 * @returns {Array<{ value: number, index: number }>}
 */
function visibleEntries(chart) {
  const values = chart?.data?.datasets?.[0]?.data ?? [];
  return values
    .map((value, index) => ({ value: Number(value) || 0, index }))
    .filter(({ index }) => chart.getDataVisibility(index));
}

/** Доля сегмента в процентах, нарисованная поверх самого сегмента. */
export const sliceLabelsPlugin = {
  id: 'sliceLabels',

  afterDatasetsDraw(chart) {
    // Пустое кольцо держится на сегменте-заглушке: подписать его «100%» значило
    // бы выдать отсутствие данных за полную долю.
    if (chart.options?.plugins?.sliceLabels?.display === false) return;
    const arcs = chart.getDatasetMeta(0)?.data ?? [];
    const total = visibleEntries(chart).reduce((sum, entry) => sum + entry.value, 0);
    if (!total) return;

    const { ctx } = chart;
    ctx.save();
    ctx.font = `600 12px ${fontFamily(chart)}`;
    ctx.fillStyle = '#ffffff';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';

    arcs.forEach((arc, index) => {
      if (!chart.getDataVisibility(index)) return;
      if (Math.abs((arc.endAngle ?? 0) - (arc.startAngle ?? 0)) < MIN_LABEL_ANGLE) return;
      const value = Number(chart.data.datasets[0].data[index]) || 0;
      const { x, y } = arc.tooltipPosition();
      ctx.fillText(`${Math.round((value / total) * 100)}%`, x, y);
    });

    ctx.restore();
  },
};

/**
 * Подпись в середине кольца: итог по видимым сегментам, а при наведении -
 * имя и значение самого сегмента.
 *
 * Подпись и форматирование берутся из замыкания, а не из
 * `options.plugins.centerLabel`: всякую функцию внутри options Chart.js считает
 * вычисляемой настройкой и зовёт её сам, передавая свой контекст. Формат
 * значения, положенный туда, получал вместо числа объект и ронял отрисовку -
 * в браузере, но не в юните: мок Chart.js настройки не разрешает.
 *
 * @param {{ label?: string, format?: (v: number) => string, total?: number|null }} settings
 *   total - готовый итог: у пустого кольца сегмент-заглушка сложилась бы в свою
 *   единицу вместо нуля.
 * @returns {object} плагин Chart.js
 */
export function centerLabelPlugin({ label = '', format = String, total = null } = {}) {
  return {
    id: 'centerLabel',

    afterDatasetsDraw(chart) {
      // Середина кольца - центр самой дуги, а не области графика: легенда снизу
      // забирает часть области, и кольцо в ней стоит выше центра.
      const anchor = chart.getDatasetMeta(0)?.data?.[0];
      if (!anchor) return;

      const active = chart.getActiveElements?.()?.[0];
      const hovered = active && chart.getDataVisibility(active.index) ? active.index : null;
      // У пустого кольца наводиться не на что: заглушка не должна подменять
      // подпись своим пустым именем.
      const caption = total != null || hovered == null
        ? String(label)
        : String(chart.data.labels?.[hovered] ?? '');
      const value = total != null
        ? total
        : (hovered == null
          ? visibleEntries(chart).reduce((sum, entry) => sum + entry.value, 0)
          : Number(chart.data.datasets[0].data[hovered]) || 0);

      const { ctx } = chart;
      const family = fontFamily(chart);
      ctx.save();
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.font = `400 12px ${family}`;
      ctx.fillStyle = themed(chart, '--text-muted', '#a2a2a2');
      ctx.fillText(caption, anchor.x, anchor.y - 12);
      ctx.font = `700 20px ${family}`;
      // Цвет темы, а не прибитый тёмно-серый: на тёмной карточке итог в центре
      // кольца был почти неразличим.
      ctx.fillStyle = themed(chart, '--text', '#333333');
      ctx.fillText(format(value), anchor.x, anchor.y + 10);
      ctx.restore();
    },
  };
}
