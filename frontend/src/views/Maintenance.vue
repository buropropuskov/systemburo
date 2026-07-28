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
      <div class="mt__topbar-right">
        <a
          class="mt__admin-link"
          href="/?admin"
          data-testid="admin-login-link"
        >Вход для администратора</a>
        <div class="mt__chip">
          Плановое обслуживание
        </div>
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
        <p
          class="mt__lede"
          :class="{ 'mt__lede--announced': hasAnnouncement }"
          data-testid="maintenance-message"
        >
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
            <span>Сроки работ</span>
          </div>
          <div
            class="mt__window"
            data-testid="maintenance-window"
          >
            <div class="mt__window-row">
              <span class="mt__window-label">Начало</span>
              <span class="mt__window-value">{{ startText }}</span>
            </div>
            <div class="mt__window-row">
              <span class="mt__window-label">Окончание</span>
              <span class="mt__window-value">{{ endText }}</span>
            </div>
            <div
              v-if="remainingText"
              class="mt__window-row"
            >
              <span class="mt__window-label">Осталось</span>
              <span class="mt__window-value mt__window-value--accent">{{ remainingText }}</span>
            </div>
          </div>

          <div
            v-if="hasWindow"
            class="mt__progress"
          >
            <div class="mt__progress-top">
              <span class="mt__progress-label">Прошло времени от объявленного срока</span>
              <span class="mt__progress-pct">{{ progressPercent }}%</span>
            </div>
            <div class="mt__progress-bar">
              <div
                class="mt__progress-fill"
                :style="{ width: progressPercent + '%' }"
              />
            </div>
          </div>

          <dl
            v-if="hasContacts"
            class="mt__meta"
          >
            <div v-if="supportEmail">
              <dt>Почта поддержки</dt>
              <dd>
                <a :href="`mailto:${supportEmail}`">{{ supportEmail }}</a>
              </dd>
            </div>
            <div v-if="supportPhone">
              <dt>Телефон поддержки</dt>
              <dd>
                <a :href="`tel:${phoneHref}`">{{ supportPhone }}</a>
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
import { formatDateTime } from '@/utils/datetime'

const POLL_MS = 30_000
const DEFAULT_MESSAGE = 'Обновляем систему пропусков. Сервис временно недоступен — мы вернёмся, как только закончим проверки. Страница автоматически обновится, когда работы завершатся.'

export default {
  name: 'MaintenancePage',
  data() {
    return {
      secondsLeft: 30,
      // Пересчитывается тикером: от него зависят остаток и прогресс окна.
      now: Date.now(),
      tickInterval: null,
      pollInterval: null,
    }
  },
  computed: {
    store() {
      return useMaintenanceStore()
    },
    hasAnnouncement() {
      return !!this.store.message
    },
    messageText() {
      return this.store.message || DEFAULT_MESSAGE
    },
    /** Объявленное окно работ; без него страница не показывает ни срок, ни прогресс. */
    hasWindow() {
      return !!(this.store.plannedStart && this.store.plannedEnd)
    },
    startText() {
      return formatDateTime(this.store.plannedStart || this.store.startedAt) || '—'
    },
    endText() {
      return formatDateTime(this.store.plannedEnd) || 'уточняется'
    },
    /** Остаток до объявленного окончания. Пусто, если окна нет или срок вышел. */
    remainingText() {
      if (!this.hasWindow) return ''
      const leftMs = new Date(this.store.plannedEnd).getTime() - this.now
      if (Number.isNaN(leftMs) || leftMs <= 0) return ''
      const minutes = Math.ceil(leftMs / 60000)
      if (minutes < 60) return `${minutes} мин`
      const hours = Math.floor(minutes / 60)
      const rest = minutes % 60
      return rest ? `${hours} ч ${rest} мин` : `${hours} ч`
    },
    progressPercent() {
      if (!this.hasWindow) return 0
      const start = new Date(this.store.plannedStart).getTime()
      const end = new Date(this.store.plannedEnd).getTime()
      if (Number.isNaN(start) || Number.isNaN(end) || end <= start) return 0
      const passed = ((this.now - start) / (end - start)) * 100
      return Math.min(100, Math.max(0, Math.round(passed)))
    },
    supportEmail() {
      return this.store.supportEmail
    },
    supportPhone() {
      return this.store.supportPhone
    },
    /** Телефон для tel:-ссылки - без пробелов, скобок и дефисов. */
    phoneHref() {
      return this.supportPhone.replace(/[^\d+]/g, '')
    },
    hasContacts() {
      return !!(this.supportEmail || this.supportPhone)
    },
  },
  mounted() {
    this.tickInterval = setInterval(() => {
      this.now = Date.now()
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
.mt__topbar-right {
  display: flex;
  align-items: center;
  gap: 18px;
}
.mt__admin-link {
  font-size: 12px;
  color: var(--text-muted);
  text-decoration: none;
  border-bottom: 1px dashed var(--border);
}
.mt__admin-link:hover { color: var(--accent-text); }
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
/* Объявление администратора - главное на странице: читаемый размер, основной
   цвет текста и акцентная подложка, чтобы не терялось рядом с заголовком. */
.mt__lede--announced {
  font-size: 20px;
  font-weight: 500;
  color: var(--text);
  padding: 18px 24px;
  border-left: 4px solid var(--accent);
  border-radius: 0 20px 20px 0;
  background: var(--accent-tint);
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
.mt__window {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 22px 26px;
  border-radius: 20px;
  background: var(--accent-tint);
}
.mt__window-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
}
.mt__window-label {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 1.5px;
  text-transform: uppercase;
  color: var(--text-muted);
}
.mt__window-value {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}
.mt__window-value--accent { color: var(--accent-text); }
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
  background: linear-gradient(90deg, var(--accent) 0%, color-mix(in srgb, var(--accent) 55%, var(--surface)) 100%);
  border-radius: 999px;
  position: relative;
  overflow: hidden;
  transition: width 0.3s ease;
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
.mt__meta a { color: var(--accent-text); text-decoration: none; }
.mt__meta a:hover { text-decoration: underline; }

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
  /* Бренд и чип статуса не помещаются в строку на 390 - раскладываем столбцом. */
  .mt__topbar {
    padding: 20px 24px;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .mt__chip { font-size: 10px; letter-spacing: 1.5px; }
  .mt__topbar-right {
    width: 100%;
    justify-content: space-between;
    gap: 10px;
  }
  .mt__admin-link { font-size: 11px; white-space: nowrap; }
  .mt__main { padding: 10px 24px 30px; }
  .mt__evidence-inner { padding: 24px 20px; }
  .mt__bg-number span { font-size: 440px; }
  .mt__meta { grid-template-columns: 1fr; }
  /* Дата целиком не встаёт рядом с подписью - подпись сверху, значение снизу. */
  .mt__window-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .mt__lede--announced { font-size: 18px; padding: 16px 18px; }
}
</style>
