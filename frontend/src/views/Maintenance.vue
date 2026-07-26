<template>
  <div
    class="mt"
    data-testid="maintenance-page"
  >
    <div class="mt__grid" />
    <div
      class="mt__bg-number"
      aria-hidden="true"
    >
      <span>OFF</span>
    </div>

    <header class="mt__topbar">
      <span class="mt__brand">Systemburo</span>
      <div class="mt__chip">
        Плановое обслуживание
      </div>
    </header>

    <main class="mt__main">
      <section class="mt__hero">
        <div class="mt__kicker">
          Статус сервиса
        </div>
        <h1 class="mt__title">
          Технические <em>работы</em>.
        </h1>
        <p class="mt__lede">
          {{ messageText }}
        </p>

        <div class="mt__actions">
          <button
            class="mt__btn mt__btn--primary"
            data-testid="reload-btn"
            @click="reload"
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <polyline points="23 4 23 10 17 10" />
              <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
            </svg>
            Обновить страницу
          </button>
          <p class="mt__hint">
            <span class="mt__tick" />
            <span class="mt__hint-text">
              Авто-проверка статуса каждые <b>{{ secondsLeft }}</b> с. Когда сервис вернётся — вас перебросит на главную автоматически.
            </span>
          </p>
        </div>
      </section>

      <aside class="mt__evidence">
        <div class="mt__evidence-shape">
          <svg viewBox="0 0 200 200">
            <defs>
              <linearGradient
                id="mtgg"
                x1="0%"
                y1="0%"
                x2="100%"
                y2="100%"
              >
                <stop
                  offset="0%"
                  stop-color="#818cf8"
                />
                <stop
                  offset="100%"
                  stop-color="#4F5BDF"
                />
              </linearGradient>
            </defs>
            <g transform="translate(100, 100)">
              <g fill="url(#mtgg)">
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(45)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(90)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(135)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(180)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(225)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(270)"
                />
                <rect
                  x="-12"
                  y="-92"
                  width="24"
                  height="36"
                  rx="6"
                  transform="rotate(315)"
                />
              </g>
              <circle
                r="70"
                fill="url(#mtgg)"
                stroke="#3d49c7"
                stroke-width="3"
              />
              <circle
                r="28"
                fill="#fff"
              />
              <circle
                r="10"
                fill="#4F5BDF"
              />
            </g>
          </svg>
        </div>
        <div class="mt__evidence-inner">
          <div class="mt__evidence-label">
            <span>Deploy log</span>
          </div>
          <pre class="mt__code"># deployment pipeline
<span class="mt-t">[14:02]</span> <span class="mt-step">BACKUP</span>   <span class="mt-ok">ok</span>
<span class="mt-t">[14:08]</span> <span class="mt-step">SERVICES</span> stopped
<span class="mt-t">[14:12]</span> <span class="mt-step">MIGRATE</span>  <span class="mt-run">running...</span>
        schema applied 3/5
<span class="mt-t">[--:--]</span> <span class="mt-step">SMOKE</span>    <span class="mt-wait">pending</span>
<span class="mt-t">[--:--]</span> <span class="mt-step">RESTART</span>  <span class="mt-wait">pending</span></pre>
          <div class="mt__progress">
            <div class="mt__progress-top">
              <span class="mt__progress-label">Общий прогресс обновления</span>
              <span class="mt__progress-pct">60% · 3 из 5</span>
            </div>
            <div class="mt__progress-bar">
              <div class="mt__progress-fill" />
            </div>
          </div>
          <dl class="mt__meta">
            <div>
              <dt>Начало</dt>
              <dd>{{ startedAtText }}</dd>
            </div>
            <div>
              <dt>Ожидаемое окончание</dt>
              <dd>{{ expectedEndText }}</dd>
            </div>
            <div style="grid-column: 1 / -1;">
              <dt>Контакт</dt>
              <dd>
                <a :href="`mailto:${supportEmailText}`">{{ supportEmailText }}</a>
              </dd>
            </div>
          </dl>
        </div>
      </aside>
    </main>
  </div>
</template>

<script>
import { useMaintenanceStore } from '@/stores/maintenance'

const POLL_MS = 30_000

export default {
  name: 'MaintenancePage',
  data() {
    return {
      secondsLeft: 30,
      tickInterval: null,
      pollInterval: null,
    }
  },
  computed: {
    store() {
      return useMaintenanceStore()
    },
    messageText() {
      return this.store.message
        || 'Обновляем систему пропусков. Сервис временно недоступен — мы вернёмся, как только закончим проверки. Страница автоматически обновится, когда работы завершатся.'
    },
    startedAtText() {
      if (!this.store.startedAt) return '—'
      const d = new Date(this.store.startedAt)
      return d.toLocaleString('ru-RU', {
        day: '2-digit', month: 'short', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
      }) + ' МСК'
    },
    expectedEndText() {
      return '—'
    },
    supportEmailText() {
      return this.store.supportEmail || 'support@buropropuskov.ru'
    },
  },
  mounted() {
    this.tickInterval = setInterval(() => {
      this.secondsLeft -= 1
      if (this.secondsLeft <= 0) this.secondsLeft = 30
    }, 1000)
    this.pollInterval = setInterval(async () => {
      await this.store.fetchStatus()
      if (!this.store.enabled) {
        // Режим выключен - редиректим на главную (бутстрап распределит дальше).
        window.location.href = '/'
      }
    }, POLL_MS)
  },
  beforeUnmount() {
    if (this.tickInterval) clearInterval(this.tickInterval)
    if (this.pollInterval) clearInterval(this.pollInterval)
  },
  methods: {
    reload() {
      window.location.reload()
    },
  },
}
</script>

<style scoped>
.mt {
  /* zoom-safe (#1097): vh под корневым zoom раздувается -> низ за экраном, см. Error500. */
  min-height: calc(var(--app-vh, 1vh) * 100);
  /* B.3 (#1097): svh стабилизирует высоту на мобилке, min() держит zoom-корректность, см. Error500. */
  min-height: min(calc(var(--app-vh, 1vh) * 100), 100svh);
  /* Подложка от темы (как на Error500): литералы держали страницу светлой независимо
     от выбранной темы. */
  background:
    radial-gradient(1200px 700px at 15% 0%, color-mix(in srgb, var(--accent) 12%, var(--bg)) 0%, transparent 55%),
    radial-gradient(900px 600px at 100% 100%, color-mix(in srgb, var(--success) 12%, var(--bg)) 0%, transparent 50%),
    var(--bg);
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  color: var(--text);
}
.mt__grid {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--accent) 12%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--accent) 12%, transparent) 1px, transparent 1px);
  background-size: 60px 60px;
  pointer-events: none;
  z-index: 0;
}
.mt__bg-number {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  z-index: 0;
  pointer-events: none;
  user-select: none;
  overflow: hidden;
}
.mt__bg-number span {
  font-family: 'Montserrat', sans-serif;
  font-weight: 900;
  font-size: clamp(400px, calc(var(--app-vh, 1vh) * 82), 900px);
  line-height: 0.78;
  color: var(--accent-text);
  opacity: 0.08;
  letter-spacing: -0.08em;
  white-space: nowrap;
  transform: translateY(-4%);
}
.mt__topbar {
  position: relative;
  z-index: 3;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 28px 48px;
}
.mt__brand {
  font-weight: 700;
  font-size: 15px;
  letter-spacing: 0.5px;
}
.mt__chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  border-radius: 999px;
  background: var(--accent-tint);
  color: var(--accent-text);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 2px;
  text-transform: uppercase;
}
.mt__chip::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
  animation: mt_pulse 1.5s ease-in-out infinite;
}
@keyframes mt_pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.4; transform: scale(1.5); }
}
.mt__main {
  position: relative;
  z-index: 2;
  flex: 1;
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  align-items: center;
  gap: 80px;
  padding: 20px 96px 60px;
}
.mt__kicker {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 4px;
  color: var(--accent-text);
  text-transform: uppercase;
  margin-bottom: 24px;
}
.mt__kicker::before {
  content: '';
  width: 36px;
  height: 2px;
  background: var(--accent);
}
.mt__title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 800;
  font-size: clamp(48px, 6vw, 80px);
  line-height: 1.02;
  letter-spacing: -0.025em;
  margin: 0 0 28px;
}
.mt__title em {
  font-style: normal;
  color: var(--accent-text);
  font-weight: 800;
}
.mt__lede {
  font-size: 17px;
  line-height: 1.6;
  color: var(--text-muted);
  margin: 0 0 36px;
  max-width: 540px;
}
.mt__actions {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 540px;
}
.mt__btn {
  font-family: inherit;
  font-size: 15px;
  font-weight: 500;
  padding: 14px 28px;
  border-radius: 30px;
  border: 1px solid var(--accent);
  background: var(--accent);
  color: var(--accent-contrast);
  cursor: pointer;
  transition: background 0.2s ease, border-color 0.2s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.mt__btn:hover { background: #3d49c7; border-color: #3d49c7; }
.mt__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--text-muted);
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.mt__hint-text { flex: 1; }
.mt__tick {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid var(--accent);
  border-top-color: transparent;
  border-radius: 50%;
  animation: mt_rotate 1s linear infinite;
  flex-shrink: 0;
  margin-top: 3px;
}
@keyframes mt_rotate {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
.mt__evidence { position: relative; }
.mt__evidence-inner {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  padding: 36px;
  box-shadow: 0 3px 10px var(--shadow-drop);
}
.mt__evidence-shape {
  position: absolute;
  top: -34px;
  left: -24px;
  width: 110px;
  height: 110px;
  filter: drop-shadow(0 10px 20px rgba(79, 91, 223, 0.3));
  z-index: 1;
}
.mt__evidence-shape svg {
  width: 100%;
  height: 100%;
  animation: mt_spin 8s linear infinite;
  transform-origin: 50% 50%;
}
@keyframes mt_spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
.mt__evidence-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 3px;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 14px;
  padding-left: 80px;
}
.mt__code {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 14.5px;
  /* Панель намеренно тёмная в любой теме - стилизованная консоль. */
  background: #0f1129;
  color: #a5b4fc;
  padding: 22px 26px;
  border-radius: 20px;
  line-height: 1.75;
  overflow-x: auto;
  margin: 0;
}
.mt__code .mt-t { color: #64748b; }
.mt__code .mt-ok { color: #34d399; }
.mt__code .mt-run { color: #a5b4fc; font-weight: 600; }
.mt__code .mt-wait { color: #64748b; }
.mt__code .mt-step { color: #fff; }
.mt__progress {
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mt__progress-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}
.mt__progress-label { font-weight: 500; }
.mt__progress-pct {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 12px;
  color: var(--accent-text);
  font-weight: 600;
}
.mt__progress-bar {
  height: 8px;
  background: var(--accent-tint);
  border-radius: 999px;
  overflow: hidden;
  position: relative;
}
.mt__progress-fill {
  height: 100%;
  width: 60%;
  background: linear-gradient(90deg, var(--accent) 0%, color-mix(in srgb, var(--accent) 55%, var(--surface)) 100%);
  border-radius: 999px;
  position: relative;
  overflow: hidden;
}
.mt__progress-fill::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.4), transparent);
  animation: mt_shimmer 2s linear infinite;
}
@keyframes mt_shimmer {
  from { transform: translateX(-100%); }
  to   { transform: translateX(100%); }
}
.mt__meta {
  margin: 24px 0 0;
  padding-top: 20px;
  border-top: 1px dashed var(--border);
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px 24px;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 14px;
}
.mt__meta dt {
  font-family: 'Montserrat', sans-serif;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1.5px;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 6px;
}
.mt__meta dd {
  margin: 0 0 4px;
  color: var(--text);
  font-weight: 500;
}
.mt__meta a { color: #4F5BDF; text-decoration: none; }

@media (max-width: 1100px) {
  .mt__main {
    grid-template-columns: 1fr;
    padding: 10px 40px 40px;
    gap: 40px;
  }
  .mt__evidence-shape { display: none; }
  .mt__evidence-label { padding-left: 0; }
  .mt__title { font-size: clamp(40px, 9vw, 64px); }
}
@media (max-width: 640px) {
  .mt__topbar { padding: 20px 24px; }
  .mt__main { padding: 10px 24px 30px; }
  .mt__evidence-inner { padding: 24px 20px; }
  .mt__bg-number span { font-size: 440px; }
  .mt__meta { grid-template-columns: 1fr; }
}
</style>
