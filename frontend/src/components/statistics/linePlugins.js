/**
 * Плагины линейного графика: вертикальная линия под курсором.
 *
 * Chart.js такой линии не рисует, а без неё на длинном ряду не видно, к какому
 * делению оси относится подсказка. Плагин держим отдельным модулем по образцу
 * donutPlugins: его логика - расчёт координат, она проверяется юнитами на
 * поддельном холсте.
 */

import { cssVariable, withAlpha } from './useChartCanvas';

/**
 * Просвет вокруг точки ряда, в который линия не заходит.
 * Чуть больше радиуса точки с обводкой (6 + 3), чтобы просвет читался.
 */
const POINT_GAP = 11;

/**
 * Вертикальная линия от верха области графика до низа, с разрывом у точки ряда.
 *
 * Рисуется поверх графика: под заливкой области линию почти не видно. Разрыв
 * нужен, чтобы линия не перечёркивала саму точку - иначе она перебивает то
 * единственное место, ради которого её и ведут глазами.
 */
export const crosshairPlugin = {
  id: 'crosshair',

  afterDatasetsDraw(chart) {
    const active = chart.getActiveElements?.() ?? [];
    const element = active[0]?.element;
    const area = chart.chartArea;
    if (!element || !area) return;

    const accent = cssVariable(chart.canvas, '--accent', '#4F5BDF');
    const { ctx } = chart;
    ctx.save();
    ctx.setLineDash([4, 4]);
    ctx.lineWidth = 1;
    ctx.strokeStyle = withAlpha(accent, 0.45);
    ctx.beginPath();
    ctx.moveTo(element.x, area.top);
    ctx.lineTo(element.x, element.y - POINT_GAP);
    ctx.moveTo(element.x, element.y + POINT_GAP);
    ctx.lineTo(element.x, area.bottom);
    ctx.stroke();
    ctx.restore();
  },
};
