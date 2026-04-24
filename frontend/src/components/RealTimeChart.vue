<template>
  <div
    class="realtime-chart"
    :style="{ height: height + 'px' }"
  >
    <canvas ref="canvas" />
    <div
      v-if="!data.length"
      class="chart-empty"
    >
      <span>Нет данных для отображения</span>
    </div>
  </div>
</template>

<script>
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
    color: {
      type: String,
      default: '#4F5BDF'
    },
    intervalLabel: {
      type: String,
      default: 'мин'
    }
  },
  watch: {
    data: {
      handler() {
        this.draw();
      },
      deep: true
    }
  },
  mounted() {
    this.draw();
    this.resizeObserver = new ResizeObserver(() => this.draw());
    this.resizeObserver.observe(this.$refs.canvas.parentElement);
  },
  beforeUnmount() {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
    }
  },
  methods: {
    draw() {
      const canvas = this.$refs.canvas;
      if (!canvas || !this.data.length) return;

      const parent = canvas.parentElement;
      const dpr = window.devicePixelRatio || 1;
      const w = parent.clientWidth;
      const h = parent.clientHeight;

      canvas.width = w * dpr;
      canvas.height = h * dpr;
      canvas.style.width = w + 'px';
      canvas.style.height = h + 'px';

      const ctx = canvas.getContext('2d');
      ctx.scale(dpr, dpr);
      ctx.clearRect(0, 0, w, h);

      const padding = { top: 20, right: 16, bottom: 32, left: 48 };
      const chartW = w - padding.left - padding.right;
      const chartH = h - padding.top - padding.bottom;

      const counts = this.data.map(d => d.count || 0);
      const maxVal = Math.max(...counts, 1);

      ctx.strokeStyle = '#e6e6e6';
      ctx.lineWidth = 1;
      ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.fillStyle = '#a2a2a2';
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

      if (this.data.length < 2) return;

      const step = chartW / (this.data.length - 1);

      const gradient = ctx.createLinearGradient(0, padding.top, 0, h - padding.bottom);
      gradient.addColorStop(0, this.color + '30');
      gradient.addColorStop(1, this.color + '05');

      ctx.beginPath();
      ctx.moveTo(padding.left, h - padding.bottom);

      for (let i = 0; i < this.data.length; i++) {
        const x = padding.left + i * step;
        const y = padding.top + chartH - (counts[i] / maxVal) * chartH;
        if (i === 0) {
          ctx.lineTo(x, y);
        } else {
          const prevX = padding.left + (i - 1) * step;
          const prevY = padding.top + chartH - (counts[i - 1] / maxVal) * chartH;
          const cpx = (prevX + x) / 2;
          ctx.bezierCurveTo(cpx, prevY, cpx, y, x, y);
        }
      }

      ctx.lineTo(padding.left + (this.data.length - 1) * step, h - padding.bottom);
      ctx.closePath();
      ctx.fillStyle = gradient;
      ctx.fill();

      ctx.beginPath();
      for (let i = 0; i < this.data.length; i++) {
        const x = padding.left + i * step;
        const y = padding.top + chartH - (counts[i] / maxVal) * chartH;
        if (i === 0) {
          ctx.moveTo(x, y);
        } else {
          const prevX = padding.left + (i - 1) * step;
          const prevY = padding.top + chartH - (counts[i - 1] / maxVal) * chartH;
          const cpx = (prevX + x) / 2;
          ctx.bezierCurveTo(cpx, prevY, cpx, y, x, y);
        }
      }
      ctx.strokeStyle = this.color;
      ctx.lineWidth = 2;
      ctx.stroke();

      ctx.fillStyle = '#a2a2a2';
      ctx.textAlign = 'center';
      ctx.font = '10px -apple-system, BlinkMacSystemFont, sans-serif';

      const labelStep = Math.max(1, Math.floor(this.data.length / 6));
      for (let i = 0; i < this.data.length; i += labelStep) {
        const x = padding.left + i * step;
        const ts = this.data[i].timestamp;
        const label = this.formatLabel(ts);
        ctx.fillText(label, x, h - padding.bottom + 16);
      }
    },

    formatLabel(timestamp) {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit'
      });
    }
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
}

.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-size: 13px;
}
</style>
