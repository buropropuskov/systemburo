<template>
  <div
    class="realtime-chart"
    :style="{ height: height + 'px' }"
  >
    <canvas
      ref="canvas"
      @mousemove="onMouseMove"
      @mouseleave="onMouseLeave"
    />
    <div
      v-if="!data.length"
      class="chart-empty"
    >
      <span>Нет данных для отображения</span>
    </div>
    <div
      v-if="hoverPoint"
      class="chart-tooltip"
      :style="{
        left: hoverPoint.tooltipX + 'px',
        top: hoverPoint.tooltipY + 'px',
      }"
    >
      <div class="chart-tooltip__time">
        {{ hoverPoint.timeLabel }}
      </div>
      <div class="chart-tooltip__count">
        <span
          class="chart-tooltip__dot"
          :style="{ background: lineColor }"
        />
        {{ hoverPoint.count }} {{ pluralize(hoverPoint.count) }}
      </div>
    </div>
  </div>
</template>

<script>
import { cssVariable, watchTheme, withAlpha } from '@/utils/chartColors';

export default {
  name: 'RealTimeChart',
  props: {
    data: {
      type: Array,
      default: () => []
    },
    height: {
      type: Number,
      default: 200
    },
    // Цвет линии. Пусто - берётся акцент темы: на холсте CSS-переменные не
    // работают, поэтому цвет читается из стилей и обновляется вместе с темой.
    color: {
      type: String,
      default: ''
    },
    intervalLabel: {
      type: String,
      default: 'мин'
    },
    // Три формы склонения единицы для тултипа [одна, две-четыре, пять+].
    // Дефолт — «запрос» (лента запросов в RequestsView); дашборд аналитики
    // передаёт форму по метрике (заявки/проходы/проезды).
    unitForms: {
      type: Array,
      default: () => ['запрос', 'запроса', 'запросов']
    }
  },
  data() {
    return {
      // Цвет точки в подсказке: шаблону нужен готовый цвет, а холст читает
      // палитру сам на каждой отрисовке.
      dotColor: '',
      hoverIndex: null,
      hoverPoint: null,
      geometry: null,
    }
  },
  watch: {
    data: {
      handler() {
        this.hoverIndex = null;
        this.hoverPoint = null;
        this.draw();
      },
      deep: true
    }
  },
  mounted() {
    this.draw();
    this.resizeObserver = new ResizeObserver(() => this.draw());
    this.resizeObserver.observe(this.$refs.canvas.parentElement);
    // Без наблюдателя график остаётся в палитре прошлой темы до следующего ответа.
    this.themeObserver = watchTheme(this.$refs.canvas, this.draw);
  },
  beforeUnmount() {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
    }
    if (this.themeObserver) {
      this.themeObserver.disconnect();
    }
  },
  methods: {
    /**
     * Цвета оформления читаются на каждой отрисовке: холст переменных CSS не
     * понимает, а тема меняется под ним.
     * @returns {{line: string, grid: string, label: string, surface: string}}
     */
    palette() {
      const el = this.$refs.canvas;
      return {
        line: this.color || cssVariable(el, '--accent', '#4F5BDF'),
        grid: cssVariable(el, '--border', '#e6e6e6'),
        label: cssVariable(el, '--text-muted', '#a2a2a2'),
        surface: cssVariable(el, '--surface', '#ffffff'),
      };
    },

    pluralize(n) {
      const [one, few, many] = this.unitForms;
      const mod10 = n % 10;
      const mod100 = n % 100;
      if (mod10 === 1 && mod100 !== 11) return one;
      if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
      return many;
    },

    draw() {
      const canvas = this.$refs.canvas;
      if (!canvas || !this.data.length) {
        this.geometry = null;
        return;
      }

      const parent = canvas.parentElement;
      const dpr = window.devicePixelRatio || 1;
      const w = parent.clientWidth;
      const h = parent.clientHeight;

      canvas.width = w * dpr;
      canvas.height = h * dpr;
      canvas.style.width = w + 'px';
      canvas.style.height = h + 'px';

      const palette = this.palette();
      this.dotColor = palette.line;
      const ctx = canvas.getContext('2d');
      ctx.scale(dpr, dpr);
      ctx.clearRect(0, 0, w, h);

      const padding = { top: 20, right: 16, bottom: 32, left: 48 };
      const chartW = w - padding.left - padding.right;
      const chartH = h - padding.top - padding.bottom;

      const counts = this.data.map(d => d.count || 0);
      const maxVal = Math.max(...counts, 1);

      ctx.strokeStyle = palette.grid;
      ctx.lineWidth = 1;
      ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.fillStyle = palette.label;
      ctx.textAlign = 'right';

      const gridLines = 4;
      for (let i = 0; i <= gridLines; i++) {
        const y = padding.top + (chartH / gridLines) * i;
        const val = Math.round(maxVal - (maxVal / gridLines) * i);
        ctx.beginPath();
        ctx.moveTo(padding.left, y);
        ctx.lineTo(w - padding.right, y);
        ctx.stroke();
        ctx.fillText(val, padding.left - 8, y + 4);
      }

      if (this.data.length < 2) {
        this.geometry = null;
        return;
      }

      const step = chartW / (this.data.length - 1);

      const points = this.data.map((d, i) => ({
        x: padding.left + i * step,
        y: padding.top + chartH - ((d.count || 0) / maxVal) * chartH,
        count: d.count || 0,
        timestamp: d.timestamp,
      }));

      this.geometry = { padding, chartW, chartH, w, h, step, points };

      const gradient = ctx.createLinearGradient(0, padding.top, 0, h - padding.bottom);
      gradient.addColorStop(0, withAlpha(palette.line, 0.19));
      gradient.addColorStop(1, withAlpha(palette.line, 0.02));

      ctx.beginPath();
      ctx.moveTo(padding.left, h - padding.bottom);
      for (let i = 0; i < points.length; i++) {
        const p = points[i];
        if (i === 0) {
          ctx.lineTo(p.x, p.y);
        } else {
          const prev = points[i - 1];
          const cpx = (prev.x + p.x) / 2;
          ctx.bezierCurveTo(cpx, prev.y, cpx, p.y, p.x, p.y);
        }
      }
      ctx.lineTo(points[points.length - 1].x, h - padding.bottom);
      ctx.closePath();
      ctx.fillStyle = gradient;
      ctx.fill();

      ctx.beginPath();
      for (let i = 0; i < points.length; i++) {
        const p = points[i];
        if (i === 0) {
          ctx.moveTo(p.x, p.y);
        } else {
          const prev = points[i - 1];
          const cpx = (prev.x + p.x) / 2;
          ctx.bezierCurveTo(cpx, prev.y, cpx, p.y, p.x, p.y);
        }
      }
      ctx.strokeStyle = palette.line;
      ctx.lineWidth = 2;
      ctx.stroke();

      ctx.fillStyle = palette.label;
      ctx.textAlign = 'center';
      ctx.font = '10px -apple-system, BlinkMacSystemFont, sans-serif';

      const labelStep = Math.max(1, Math.floor(this.data.length / 6));
      for (let i = 0; i < this.data.length; i += labelStep) {
        const p = points[i];
        ctx.fillText(this.formatLabel(p.timestamp), p.x, h - padding.bottom + 16);
      }

      if (this.hoverIndex !== null && points[this.hoverIndex]) {
        const p = points[this.hoverIndex];
        ctx.beginPath();
        ctx.strokeStyle = withAlpha(palette.line, 0.35);
        ctx.lineWidth = 1;
        ctx.setLineDash([4, 4]);
        ctx.moveTo(p.x, padding.top);
        ctx.lineTo(p.x, h - padding.bottom);
        ctx.stroke();
        ctx.setLineDash([]);

        ctx.beginPath();
        ctx.arc(p.x, p.y, 5, 0, Math.PI * 2);
        ctx.fillStyle = palette.surface;
        ctx.fill();
        ctx.strokeStyle = palette.line;
        ctx.lineWidth = 2;
        ctx.stroke();
      }
    },

    formatLabel(timestamp) {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit'
      });
    },

    formatTooltipTime(timestamp) {
      const date = new Date(timestamp);
      return date.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      }).replace(',', '');
    },

    onMouseMove(e) {
      if (!this.geometry || !this.geometry.points.length) return;
      const rect = e.currentTarget.getBoundingClientRect();
      const mouseX = e.clientX - rect.left;
      const { points } = this.geometry;

      let nearest = 0;
      let minDist = Infinity;
      for (let i = 0; i < points.length; i++) {
        const d = Math.abs(points[i].x - mouseX);
        if (d < minDist) { minDist = d; nearest = i; }
      }

      if (this.hoverIndex !== nearest) {
        this.hoverIndex = nearest;
        const p = points[nearest];
        this.hoverPoint = {
          tooltipX: p.x,
          tooltipY: p.y - 12,
          count: p.count,
          timeLabel: this.formatTooltipTime(p.timestamp),
        };
        this.draw();
      }
    },

    onMouseLeave() {
      if (this.hoverIndex === null) return;
      this.hoverIndex = null;
      this.hoverPoint = null;
      this.draw();
    },
  }
};
</script>

<style scoped>
.realtime-chart {
  position: relative;
  width: 100%;
}

.realtime-chart canvas {
  display: block;
  width: 100%;
  height: 100%;
  cursor: crosshair;
}

.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 13px;
}

.chart-tooltip {
  position: absolute;
  transform: translate(-50%, -100%);
  background: var(--hint-bg);
  color: var(--hint-text);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.3;
  white-space: nowrap;
  pointer-events: none;
  box-shadow: 0 8px 20px var(--shadow-drop);
  z-index: 10;
}

.chart-tooltip__time {
  font-weight: 500;
  color: var(--border);
  margin-bottom: 2px;
  font-size: 11px;
}

.chart-tooltip__count {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
}

.chart-tooltip__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}
</style>
