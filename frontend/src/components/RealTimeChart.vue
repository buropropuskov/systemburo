<!-- src/components/RealTimeChart.vue -->
<template>
  <div class="realtime-chart" ref="container">
    <canvas ref="canvas" @mousemove="onMouseMove" @mouseleave="onMouseLeave" :width="width" :height="height"></canvas>
    <div v-if="tooltip.show" class="chart-tooltip" :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }">
      <div><strong>{{ tooltip.time }}</strong></div>
      <div>Количество: {{ tooltip.value }}</div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'RealTimeChart',
  props: {
    data: {
      type: Array,
      required: true,
      default: () => []
    },
    height: {
      type: Number,
      default: 320
    },
    color: {
      type: String,
      default: '#4F5BDF'
    },
    intervalLabel: {
      type: String,
      default: 'Час'
    }
  },
  data() {
    return {
      width: 0,
      ctx: null,
      animationId: null,
      targetData: [],
      smoothPoints: [],
      hoverIndex: -1,
      tooltip: { show: false, x: 0, y: 0, time: '', value: 0 },
    }
  },
  watch: {
    data: {
      deep: true,
      handler(val) {
        this.targetData = val.map(p => p.count)
        this.$nextTick(() => {
          this.calcSmoothPoints()
          if (this.ctx) this.animateTo(this.targetData)
        })
      }
    }
  },
  mounted() {
    this.initCanvas()
    window.addEventListener('resize', this.resizeCanvas)
    this.resizeCanvas()
    this.targetData = this.data.map(p => p.count)
    this.calcSmoothPoints()
    this.draw()
  },
  beforeUnmount() {
    if (this.animationId) cancelAnimationFrame(this.animationId)
    window.removeEventListener('resize', this.resizeCanvas)
  },
  methods: {
    initCanvas() {
      const canvas = this.$refs.canvas
      this.ctx = canvas.getContext('2d')
    },
    resizeCanvas() {
      const container = this.$refs.container
      if (!container) return
      this.width = container.clientWidth
      this.$refs.canvas.width = this.width
      this.$refs.canvas.height = this.height
      this.draw()
    },
    calcSmoothPoints() {
      const data = this.targetData.length ? this.targetData : this.data.map(p => p.count)
      if (data.length < 2) {
        this.smoothPoints = []
        return
      }
      const w = this.width
      const h = this.height
      const maxVal = Math.max(...data, 1)
      const stepY = h / maxVal
      const stepX = w / (data.length - 1)
      const points = data.map((v, i) => ({ x: i * stepX, y: h - v * stepY }))

      const smooth = []
      for (let i = 0; i < points.length - 1; i++) {
        const p0 = points[Math.max(i - 1, 0)]
        const p1 = points[i]
        const p2 = points[i + 1]
        const p3 = points[Math.min(i + 2, points.length - 1)]
        for (let t = 0; t <= 1; t += 0.05) {
          const x = this.catmullRom(p0.x, p1.x, p2.x, p3.x, t)
          const y = this.catmullRom(p0.y, p1.y, p2.y, p3.y, t)
          smooth.push({ x, y })
        }
      }
      smooth.push(points[points.length - 1])
      this.smoothPoints = smooth
    },
    catmullRom(p0, p1, p2, p3, t) {
      const t2 = t * t
      const t3 = t2 * t
      return 0.5 * ((2 * p1) +
        (-p0 + p2) * t +
        (2 * p0 - 5 * p1 + 4 * p2 - p3) * t2 +
        (-p0 + 3 * p1 - 3 * p2 + p3) * t3)
    },
    draw() {
      if (!this.ctx) return
      const w = this.width
      const h = this.height
      const data = this.targetData.length ? this.targetData : this.data.map(p => p.count)
      if (!data.length) {
        this.ctx.clearRect(0, 0, w, h)
        return
      }

      const maxVal = Math.max(...data, 1)
      const stepY = h / maxVal
      const stepX = w / (data.length - 1)
      const points = data.map((v, i) => ({ x: i * stepX, y: h - v * stepY }))

      this.ctx.clearRect(0, 0, w, h)
      this.ctx.save()
      this.ctx.font = '10px Montserrat'
      this.ctx.fillStyle = '#a2a2a2'
      this.ctx.strokeStyle = '#e6e6e6'
      this.ctx.lineWidth = 1

      // Y-axis ticks
      let tickCount = 5
      let maxTick = maxVal
      let tickStep = maxTick / tickCount
      const magnitude = Math.pow(10, Math.floor(Math.log10(tickStep)))
      let niceStep = Math.ceil(tickStep / magnitude) * magnitude
      let niceMax = Math.ceil(maxVal / niceStep) * niceStep
      let niceTicks = []
      for (let i = 0; i <= niceMax / niceStep; i++) {
        niceTicks.push(i * niceStep)
      }
      if (niceTicks.length > 6) {
        niceTicks = niceTicks.filter((_, i) => i % 2 === 0)
      }

      // Horizontal lines and labels
      niceTicks.forEach(val => {
        const y = h - (val * stepY)
        if (y >= 0 && y <= h) {
          this.ctx.beginPath()
          this.ctx.moveTo(0, y)
          this.ctx.lineTo(w, y)
          this.ctx.stroke()
          this.ctx.fillStyle = '#a2a2a2'
          this.ctx.fillText(val.toString(), 5, y - 2)
        }
      })

      // Vertical lines (optional)
      const xStepCount = Math.min(6, data.length)
      const xStep = Math.max(1, Math.floor(data.length / xStepCount))
      for (let i = 0; i < data.length; i += xStep) {
        const x = i * stepX
        this.ctx.beginPath()
        this.ctx.moveTo(x, 0)
        this.ctx.lineTo(x, h)
        this.ctx.stroke()
      }

      // Smooth line
      if (this.smoothPoints.length > 1) {
        this.ctx.beginPath()
        this.ctx.strokeStyle = this.color
        this.ctx.lineWidth = 2.5
        this.ctx.lineJoin = 'round'
        this.ctx.lineCap = 'round'
        this.ctx.moveTo(this.smoothPoints[0].x, this.smoothPoints[0].y)
        for (let i = 1; i < this.smoothPoints.length; i++) {
          this.ctx.lineTo(this.smoothPoints[i].x, this.smoothPoints[i].y)
        }
        this.ctx.stroke()
      }

      // Fill under curve (using original points)
      this.ctx.beginPath()
      this.ctx.moveTo(0, h)
      points.forEach(p => this.ctx.lineTo(p.x, p.y))
      this.ctx.lineTo(w, h)
      this.ctx.fillStyle = `${this.color}20`
      this.ctx.fill()

      // Points
      points.forEach((p, idx) => {
        const isHover = (this.hoverIndex === idx)
        this.ctx.beginPath()
        this.ctx.arc(p.x, p.y, isHover ? 6 : 4, 0, 2 * Math.PI)
        this.ctx.fillStyle = this.color
        this.ctx.fill()
        if (isHover) {
          this.ctx.shadowBlur = 8
          this.ctx.shadowColor = this.color
          this.ctx.fill()
          this.ctx.shadowBlur = 0
        }
      })

      // X-axis labels
      if (this.data.length) {
        let stepLabel = Math.ceil(this.data.length / 6)
        for (let i = 0; i < this.data.length; i += stepLabel) {
          const x = i * stepX
          const ts = new Date(this.data[i].timestamp)
          let label = ''
          switch (this.intervalLabel) {
            case 'Минута':
              label = ts.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
              break
            case 'Час':
              label = ts.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
              break
            case 'День':
              label = ts.toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' })
              break
            case 'Неделя':
              label = ts.toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' })
              break
            case 'Месяц':
              label = ts.toLocaleDateString('ru-RU', { month: 'short', year: 'numeric' })
              break
            case 'Год':
              label = ts.getFullYear()
              break
            default:
              label = ts.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
          }
          this.ctx.fillStyle = '#a2a2a2'
          this.ctx.fillText(label, x - 15, h - 5)
        }
      }
      this.ctx.restore()
    },
    onMouseMove(event) {
      const canvasRect = this.$refs.canvas.getBoundingClientRect()
      const containerRect = this.$refs.container.getBoundingClientRect()
      const mouseX = event.clientX - canvasRect.left
      const mouseY = event.clientY - canvasRect.top
      const w = this.width
      const h = this.height
      const data = this.targetData.length ? this.targetData : this.data.map(p => p.count)
      if (!data.length) return

      const maxVal = Math.max(...data, 1)
      const stepY = h / maxVal
      const stepX = w / (data.length - 1)

      let minDist = 20
      let idx = -1
      for (let i = 0; i < data.length; i++) {
        const x = i * stepX
        const y = h - data[i] * stepY
        const dx = x - mouseX
        const dy = y - mouseY
        const dist = Math.sqrt(dx * dx + dy * dy)
        if (dist < minDist) {
          minDist = dist
          idx = i
        }
      }

      if (idx !== -1 && idx !== this.hoverIndex) {
        this.hoverIndex = idx
        const x = idx * stepX
        const y = h - data[idx] * stepY
        // Calculate position relative to container
        let tooltipX = x + (canvasRect.left - containerRect.left) + 12
        let tooltipY = y + (canvasRect.top - containerRect.top) - 25
        // Adjust if tooltip would overflow container
        const tooltipWidth = 150
        const tooltipHeight = 50
        if (tooltipX + tooltipWidth > containerRect.width) {
          tooltipX = x + (canvasRect.left - containerRect.left) - tooltipWidth - 8
        }
        if (tooltipY < 0) {
          tooltipY = y + (canvasRect.top - containerRect.top) + 15
        }
        if (tooltipY + tooltipHeight > containerRect.height) {
          tooltipY = y + (canvasRect.top - containerRect.top) - tooltipHeight - 8
        }
        this.tooltip.x = tooltipX
        this.tooltip.y = tooltipY

        const ts = new Date(this.data[idx].timestamp)
        let timeStr = ''
        switch (this.intervalLabel) {
          case 'Минута':
            timeStr = ts.toLocaleString('ru-RU')
            break
          case 'Час':
            timeStr = ts.toLocaleString('ru-RU')
            break
          case 'День':
            timeStr = ts.toLocaleDateString('ru-RU', { day: '2-digit', month: 'short', year: 'numeric' })
            break
          case 'Неделя':
            timeStr = `${ts.toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' })} (неделя ${Math.ceil(ts.getDate() / 7)})`
            break
          case 'Месяц':
            timeStr = ts.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' })
            break
          case 'Год':
            timeStr = ts.getFullYear()
            break
          default:
            timeStr = ts.toLocaleString('ru-RU')
        }
        this.tooltip.time = timeStr
        this.tooltip.value = data[idx]
        this.tooltip.show = true
        this.draw()
      } else if (idx === -1 && this.hoverIndex !== -1) {
        this.hoverIndex = -1
        this.tooltip.show = false
        this.draw()
      }
    },
    onMouseLeave() {
      this.hoverIndex = -1
      this.tooltip.show = false
      this.draw()
    },
    animateTo(newData) {
      const start = [...this.targetData]
      const end = [...newData]
      const duration = 500
      const startTime = performance.now()
      const animate = (now) => {
        const elapsed = now - startTime
        const t = Math.min(1, elapsed / duration)
        this.targetData = start.map((v, i) => v + (end[i] - v) * t)
        this.calcSmoothPoints()
        this.draw()
        if (t < 1) {
          this.animationId = requestAnimationFrame(animate)
        } else {
          this.animationId = null
          this.targetData = [...end]
          this.calcSmoothPoints()
          this.draw()
        }
      }
      if (this.animationId) cancelAnimationFrame(this.animationId)
      this.animationId = requestAnimationFrame(animate)
    }
  }
}
</script>

<style scoped>
.realtime-chart {
  position: relative;
  width: 100%;
  overflow-x: auto;
  min-height: 320px;
}
canvas {
  width: 100%;
  height: auto;
  display: block;
}
.chart-tooltip {
  position: absolute;
  background: rgba(0,0,0,0.85);
  color: white;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  pointer-events: none;
  white-space: nowrap;
  font-family: 'Montserrat', sans-serif;
  z-index: 20;
  backdrop-filter: blur(4px);
  box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}
</style>