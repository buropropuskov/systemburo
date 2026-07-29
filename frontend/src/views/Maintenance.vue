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
      <span class="mt__brand">Бюро пропусков</span>
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
        <!-- Шестерня как деталь с чертежа: зубчатый венец с заливкой, обод,
             спицы, ступица и отверстия облегчения. Крутится медленно. -->
        <div class="mt__gear">
          <svg
            viewBox="0 0 124 124"
            fill="none"
            aria-hidden="true"
          >
            <g
              stroke="currentColor"
              stroke-width="2"
              stroke-linejoin="round"
            >
              <path
                class="mt__gear-body"
                d="M98.0 62.0 L107.89 65.13 A46 46 0 0 1 107.14 70.86 L96.77 71.32 A36 36 0 0 1 93.18 80.0 L100.18 87.66 A46 46 0 0 1 96.66 92.24 L87.46 87.46 A36 36 0 0 1 80.0 93.18 L82.24 103.31 A46 46 0 0 1 76.9 105.52 L71.32 96.77 A36 36 0 0 1 62.0 98.0 L58.87 107.89 A46 46 0 0 1 53.14 107.14 L52.68 96.77 A36 36 0 0 1 44.0 93.18 L36.34 100.18 A46 46 0 0 1 31.76 96.66 L36.54 87.46 A36 36 0 0 1 30.82 80.0 L20.69 82.24 A46 46 0 0 1 18.48 76.9 L27.23 71.32 A36 36 0 0 1 26.0 62.0 L16.11 58.87 A46 46 0 0 1 16.86 53.14 L27.23 52.68 A36 36 0 0 1 30.82 44.0 L23.82 36.34 A46 46 0 0 1 27.34 31.76 L36.54 36.54 A36 36 0 0 1 44.0 30.82 L41.76 20.69 A46 46 0 0 1 47.1 18.48 L52.68 27.23 A36 36 0 0 1 62.0 26.0 L65.13 16.11 A46 46 0 0 1 70.86 16.86 L71.32 27.23 A36 36 0 0 1 80.0 30.82 L87.66 23.82 A46 46 0 0 1 92.24 27.34 L87.46 36.54 A36 36 0 0 1 93.18 44.0 L103.31 41.76 A46 46 0 0 1 105.52 47.1 L96.77 52.68 Z"
              />
              <circle
                class="mt__gear-rim"
                cx="62"
                cy="62"
                r="30"
              />
              <path d="M75 62L91 62" />
              <path d="M62 75L62 91" />
              <path d="M49 62L33 62" />
              <path d="M62 49L62 33" />
              <circle
                cx="78.3"
                cy="78.3"
                r="5"
              />
              <circle
                cx="45.7"
                cy="78.3"
                r="5"
              />
              <circle
                cx="45.7"
                cy="45.7"
                r="5"
              />
              <circle
                cx="78.3"
                cy="45.7"
                r="5"
              />
              <circle
                class="mt__gear-hub"
                cx="62"
                cy="62"
                r="13"
              />
              <circle
                cx="62"
                cy="62"
                r="6"
              />
            </g>
          </svg>
        </div>
        <div class="mt__evidence-inner">
          <div
            class="mt__terminal"
            data-testid="maintenance-window"
          >
            <div class="mt__terminal-bar">
              <span class="mt__terminal-dots">
                <i /><i /><i />
              </span>
              <span class="mt__terminal-title">buropropuskov — технические работы</span>
            </div>
            <div class="mt__terminal-body">
              <div class="mt__row">
                <span class="mt__key">начало</span>
                <span class="mt__val">{{ startText }}</span>
              </div>
              <div
                v-if="hasWindow"
                class="mt__row"
              >
                <span class="mt__key">окончание</span>
                <span class="mt__val">{{ endText }}</span>
              </div>
              <div
                v-if="remainingText"
                class="mt__row"
              >
                <span class="mt__key">осталось</span>
                <span class="mt__val mt__val--accent">{{ remainingText }}</span>
              </div>
              <div class="mt__row">
                <span class="mt__key">статус</span>
                <span class="mt__val">{{ statusText }}</span>
              </div>
              <div class="mt__row mt__row--prompt">
                <span class="mt__prompt">maintenance@buropropuskov:~$</span>
                <span class="mt__caret" />
              </div>
            </div>
          </div>

          <div
            v-if="hasWindow"
            class="mt__progress"
          >
            <div class="mt__progress-top">
              <span class="mt__progress-label">Выполнение работ</span>
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
import { formatRussianPhoneForDisplay } from '@/composables/useRussianPhoneMask'
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
    /** Строка состояния в журнале: до срока, после срока и без объявленного срока. */
    statusText() {
      if (!this.hasWindow) return 'работы идут'
      return this.remainingText ? 'работы идут' : 'завершаем, проверяем систему'
    },
    startClock() {
      return this.clockOf(this.store.plannedStart || this.store.startedAt)
    },
    endClock() {
      return this.clockOf(this.store.plannedEnd)
    },
    nowClock() {
      const d = new Date(this.now)
      const p = (n) => String(n).padStart(2, '0')
      return `${p(d.getHours())}:${p(d.getMinutes())}`
    },
    /** Последняя строка вывода: сколько сделано и сколько ждать. */
    progressLine() {
      if (!this.hasWindow) return this.statusText
      if (!this.remainingText) return 'срок вышел, завершаем и проверяем систему'
      return `выполнено ${this.progressPercent}%, осталось ${this.remainingText}`
    },
    supportEmail() {
      return this.store.supportEmail
    },
    /** Номер приводится к маске при показе: в настройках он мог быть сохранён
     *  цифрами подряд, а пользователю нужен читаемый вид. */
    supportPhone() {
      return formatRussianPhoneForDisplay(this.store.supportPhone)
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
    /** Часы и минуты момента для метки в выводе; пусто -> прочерк. */
    clockOf(iso) {
      if (!iso) return '--:--'
      const d = new Date(iso)
      if (Number.isNaN(d.getTime())) return '--:--'
      const p = (n) => String(n).padStart(2, '0')
      return `${p(d.getHours())}:${p(d.getMinutes())}`
    },
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
    radial-gradient(1200px 700px at 15% 0%, color-mix(in srgb, var(--accent) var(--decor-mix), var(--bg)) 0%, transparent 55%),
    radial-gradient(900px 600px at 100% 100%, color-mix(in srgb, var(--success) var(--decor-mix), var(--bg)) 0%, transparent 50%),
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
    linear-gradient(var(--decor-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--decor-line) 1px, transparent 1px);
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
  font-size: 17px;
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
  position: relative;
  z-index: 1;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  padding: 36px;
  box-shadow: 0 3px 10px var(--shadow-drop);
}
/* Контурная шестерня вместо заливной: тонкая линия и медленный оборот читаются
   как технический знак, а не как иллюстрация. */
/* Две сцепленные шестерни вместо иконки: линия под акцент страницы, разный
   размер и встречное вращение - механизм, а не значок. */
/* Шестерня лежит ПОВЕРХ угла карточки - как штамп на документе. Линия под
   акцент страницы, оборот медленный, чтобы не мельтешила. */
.mt__gear {
  position: absolute;
  top: -40px;
  left: -34px;
  width: 116px;
  height: 116px;
  color: var(--accent);
  opacity: 0.85;
  z-index: 2;
  pointer-events: none;
}
.mt__gear svg {
  width: 100%;
  height: 100%;
  animation: mt_spin 26s linear infinite;
  transform-origin: 50% 50%;
}
/* Заливки дают детали объём: венец светлее фона, ступица - плотнее. */
.mt__gear-body {
  fill: color-mix(in srgb, var(--accent) 14%, var(--surface));
}
.mt__gear-rim {
  fill: var(--surface);
}
.mt__gear-hub {
  fill: color-mix(in srgb, var(--accent) 22%, var(--surface));
}
@keyframes mt_spin {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}
/* Окно терминала тёмное в любой теме, но тон берётся из темы: тёмно-синие литералы
   складывались с общей синевой тёмного экрана. Точки-светофор и подсветка значений
   остаются литералами - это цвета терминала, а не интерфейса. */
.mt__terminal {
  border-radius: 16px;
  overflow: hidden;
  background: var(--console-bg);
  border: 1px solid var(--console-border);
  font-family: 'JetBrains Mono', ui-monospace, monospace;
}
.mt__terminal-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--console-bar);
  border-bottom: 1px solid var(--console-border);
}
.mt__terminal-dots {
  display: inline-flex;
  gap: 6px;
}
.mt__terminal-dots i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #3a3f6b;
}
.mt__terminal-dots i:first-child { background: #ff5f57; }
.mt__terminal-dots i:nth-child(2) { background: #febc2e; }
.mt__terminal-dots i:nth-child(3) { background: #28c840; }
.mt__terminal-title {
  font-size: 12px;
  letter-spacing: 0.5px;
  color: #6b7194;
}
.mt__terminal-body {
  padding: 18px 20px 20px;
  font-size: 14px;
  line-height: 1.85;
  color: #c7d2fe;
  overflow-x: auto;
}
.mt__row {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 12px;
}
.mt__row--prompt {
  display: block;
  margin-top: 10px;
}
.mt__key { color: #6b7194; }
.mt__val { color: #c7d2fe; }
.mt__val--accent { color: #34d399; }
.mt__prompt { color: #34d399; }
.mt__caret {
  display: inline-block;
  width: 9px;
  height: 16px;
  background: #c7d2fe;
  vertical-align: -3px;
  animation: mt_blink 1.1s steps(1) infinite;
}
@keyframes mt_blink {
  0%, 50% { opacity: 1; }
  50.01%, 100% { opacity: 0; }
}
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
.mt__meta a:hover {
  text-decoration: underline;
  text-underline-position: under;
}

@media (max-width: 1100px) {
  .mt__main {
    grid-template-columns: 1fr;
    padding: 10px 40px 40px;
    gap: 40px;
  }
  .mt__gear { display: none; }
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
  .mt__terminal-body {
    font-size: 12.5px;
    padding: 14px 16px 16px;
  }
  .mt__terminal-title { font-size: 11px; }
  .mt__lede--announced { font-size: 15.5px; padding: 14px 16px; }
}
</style>
