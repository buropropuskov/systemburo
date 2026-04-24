<template>
  <div
    class="err500"
    data-testid="error-500-page"
  >
    <div class="err500__grid" />
    <div
      class="err500__bg-number"
      aria-hidden="true"
    >
      <span>500</span>
    </div>

    <header class="err500__topbar">
      <span class="err500__brand">Systemburo</span>
      <div class="err500__chip">
        Ошибка сервера
      </div>
    </header>

    <main class="err500__main">
      <section class="err500__hero">
        <div class="err500__kicker">
          Ошибка 500
        </div>
        <h1 class="err500__title">
          Что-то <em>сломалось</em> на нашей стороне.
        </h1>
        <p class="err500__lede">
          Мы уже знаем о проблеме и разбираемся. Помогите быстрее её починить — отправьте один клик с деталями этого инцидента напрямую в чат разработки.
        </p>

        <div class="err500__actions">
          <button
            class="err500__btn err500__btn--primary"
            :disabled="reportSending || reportSent"
            data-testid="send-bug-report-btn"
            @click="handleReport"
          >
            <span v-if="reportSent">
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              ><polyline points="20 6 9 17 4 12" /></svg>
              Отчёт отправлен
            </span>
            <span v-else-if="reportSending">
              Отправляется...
            </span>
            <span v-else>
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M22 2L11 13" />
                <path d="M22 2l-7 20-4-9-9-4z" />
              </svg>
              Отправить отчёт разработчикам
            </span>
          </button>

          <p
            class="err500__hint"
            :class="{ 'err500__hint--success': reportSent, 'err500__hint--error': reportError }"
          >
            <template v-if="reportSent">
              Спасибо — команда получила отчёт и уже смотрит.
            </template>
            <template v-else-if="reportError">
              {{ reportError }}
            </template>
            <template v-else>
              Отправим: маршрут, код ошибки, ID инцидента и время. Без содержимого ответа сервера. Кнопка сработает один раз для одного и того же бага.
            </template>
          </p>

          <div class="err500__actions-row">
            <button
              class="err500__btn err500__btn--outlined"
              @click="goHome"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              ><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" /></svg>
              На главную
            </button>
            <button
              class="err500__btn err500__btn--outlined"
              @click="reload"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <polyline points="23 4 23 10 17 10" />
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
              </svg>
              Обновить
            </button>
          </div>
        </div>
      </section>

      <aside class="err500__evidence">
        <div class="err500__evidence-inner">
          <div class="err500__evidence-label">
            <span>Системный инцидент</span>
            <span>зафиксирован</span>
          </div>
          <pre class="err500__code"># server response
<span class="c500-k">STATUS</span>   <span class="c500-s">{{ ctx.httpStatus }}</span> {{ statusText }}
<span class="c500-k">REQUEST</span>  {{ ctx.route }}
<span class="c500-k">ID</span>       {{ bugHash || '...' }}
<span class="c500-k">TIME</span>     {{ formattedTime }}

# stack trace и детали исключения
# остаются только в защищённом
# серверном логе</pre>
          <dl class="err500__meta">
            <div>
              <dt>ID инцидента</dt>
              <dd>{{ bugHash || '...' }}</dd>
            </div>
            <div>
              <dt>Время</dt>
              <dd>{{ formattedTime }}</dd>
            </div>
            <div>
              <dt>HTTP-код</dt>
              <dd>{{ ctx.httpStatus }} {{ statusText }}</dd>
            </div>
            <div>
              <dt>Маршрут</dt>
              <dd>{{ ctx.route }}</dd>
            </div>
          </dl>
        </div>
      </aside>
    </main>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import {
  loadBugContext,
  buildBugHash,
  isReported,
  markReported,
} from '@/composables/useBugReport'

const DEFAULT_STATUS_TEXT = {
  500: 'Internal Server Error',
  502: 'Bad Gateway',
  503: 'Service Unavailable',
  504: 'Gateway Timeout',
}

export default {
  name: 'ErrorFive00',
  data() {
    return {
      ctx: {
        route: '—',
        httpStatus: 500,
        message: 'Internal Server Error',
        timestamp: new Date().toISOString(),
      },
      bugHash: '',
      reportSending: false,
      reportSent: false,
      reportError: '',
    }
  },
  computed: {
    statusText() {
      return DEFAULT_STATUS_TEXT[this.ctx.httpStatus] || this.ctx.message || 'Server Error'
    },
    formattedTime() {
      const d = new Date(this.ctx.timestamp)
      return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' МСК'
    },
  },
  async mounted() {
    const saved = loadBugContext()
    if (saved) {
      this.ctx = { ...this.ctx, ...saved }
    }
    this.bugHash = await buildBugHash(this.ctx)
    if (isReported(this.bugHash)) {
      this.reportSent = true
    }
  },
  methods: {
    async handleReport() {
      if (!this.bugHash || this.reportSending || this.reportSent) return
      this.reportSending = true
      this.reportError = ''
      try {
        const response = await apiRequest('/bug-report', {
          method: 'POST',
          body: JSON.stringify({
            bug_hash: this.bugHash,
            route: this.ctx.route,
            http_status: this.ctx.httpStatus,
            message: this.ctx.message,
          }),
        })
        if (response.ok || response.status === 409) {
          // 409 = уже отправляли - для юзера считаем отправленным.
          markReported(this.bugHash)
          this.reportSent = true
        } else if (response.status === 429) {
          this.reportError = 'Слишком много репортов подряд, попробуйте через пару минут.'
        } else {
          this.reportError = 'Не удалось отправить отчёт. Попробуйте ещё раз позже.'
        }
      } catch {
        this.reportError = 'Нет связи с сервером. Попробуйте ещё раз через минуту.'
      } finally {
        this.reportSending = false
      }
    },
    goHome() {
      this.$router.push('/')
    },
    reload() {
      window.location.reload()
    },
  },
}
</script>

<style scoped>
.err500 {
  min-height: 100vh;
  background:
    radial-gradient(1200px 700px at 15% 0%, #eef0ff 0%, transparent 55%),
    radial-gradient(900px 600px at 100% 100%, #ffe4e6 0%, transparent 50%),
    #f5f6fa;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  color: #1a1a1a;
}
.err500__grid {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(rgba(79, 91, 223, 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(79, 91, 223, 0.06) 1px, transparent 1px);
  background-size: 60px 60px;
  pointer-events: none;
  z-index: 0;
}
.err500__bg-number {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  z-index: 0;
  pointer-events: none;
  user-select: none;
  overflow: hidden;
}
.err500__bg-number span {
  font-family: 'Montserrat', sans-serif;
  font-weight: 900;
  font-size: clamp(400px, 82vh, 900px);
  line-height: 0.78;
  color: #4F5BDF;
  opacity: 0.08;
  letter-spacing: -0.08em;
  white-space: nowrap;
  transform: translateY(-4%);
}
.err500__topbar {
  position: relative;
  z-index: 3;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 28px 48px;
}
.err500__brand {
  font-weight: 700;
  font-size: 15px;
  letter-spacing: 0.5px;
}
.err500__chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  border-radius: 999px;
  background: #fee2e2;
  color: #dc2626;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 2px;
  text-transform: uppercase;
}
.err500__chip::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #dc2626;
  animation: err500_pulse 1.5s ease-in-out infinite;
}
@keyframes err500_pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.4; transform: scale(1.5); }
}
.err500__main {
  position: relative;
  z-index: 2;
  flex: 1;
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  align-items: center;
  gap: 80px;
  padding: 20px 96px 60px;
}
.err500__kicker {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 4px;
  color: #dc2626;
  text-transform: uppercase;
  margin-bottom: 24px;
}
.err500__kicker::before {
  content: '';
  width: 36px;
  height: 2px;
  background: #dc2626;
}
.err500__title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 800;
  font-size: clamp(48px, 6vw, 80px);
  line-height: 1.02;
  letter-spacing: -0.025em;
  margin: 0 0 28px;
  max-width: 820px;
}
.err500__title em {
  font-style: normal;
  color: #4F5BDF;
  font-weight: 800;
}
.err500__lede {
  font-size: 17px;
  line-height: 1.6;
  color: #6b7280;
  margin: 0 0 36px;
  max-width: 540px;
}
.err500__actions {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 540px;
}
.err500__btn {
  font-family: inherit;
  font-size: 15px;
  font-weight: 500;
  padding: 14px 28px;
  border-radius: 30px;
  border: 1px solid transparent;
  cursor: pointer;
  transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.err500__btn--primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}
.err500__btn--primary:hover:not(:disabled) {
  background: #3d49c7;
  border-color: #3d49c7;
}
.err500__btn--primary:disabled {
  background: #e5e7eb;
  color: #9ca3af;
  border-color: #e5e7eb;
  cursor: not-allowed;
}
.err500__btn--outlined {
  background: transparent;
  color: #4F5BDF;
  border-color: #4F5BDF;
}
.err500__btn--outlined:hover {
  background: #eef0ff;
}
.err500__actions-row {
  display: flex;
  gap: 12px;
}
.err500__actions-row .err500__btn {
  flex: 1;
}
.err500__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: #6b7280;
}
.err500__hint--success {
  color: #047857;
  font-weight: 500;
}
.err500__hint--error {
  color: #dc2626;
}
.err500__evidence {
  position: relative;
}
.err500__evidence-inner {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 30px;
  padding: 36px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.08);
}
.err500__evidence-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 3px;
  text-transform: uppercase;
  color: #6b7280;
  margin-bottom: 14px;
  display: flex;
  justify-content: space-between;
}
.err500__code {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 14.5px;
  background: #0f1129;
  color: #a5b4fc;
  padding: 22px 26px;
  border-radius: 20px;
  line-height: 1.7;
  overflow-x: auto;
  margin: 0;
}
.err500__code .c500-k { color: #fbbf24; }
.err500__code .c500-s { color: #fca5a5; }
.err500__meta {
  margin: 24px 0 0;
  padding-top: 20px;
  border-top: 1px dashed #e6e6e6;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px 24px;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 14px;
}
.err500__meta dt {
  font-family: 'Montserrat', sans-serif;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 1.5px;
  text-transform: uppercase;
  color: #9ca0b0;
  margin-bottom: 6px;
}
.err500__meta dd {
  margin: 0 0 4px;
  color: #1a1a1a;
  font-weight: 500;
}

@media (max-width: 1100px) {
  .err500__main {
    grid-template-columns: 1fr;
    padding: 10px 40px 40px;
    gap: 40px;
  }
  .err500__title {
    font-size: clamp(40px, 9vw, 64px);
  }
}
@media (max-width: 640px) {
  .err500__topbar {
    padding: 20px 24px;
  }
  .err500__main {
    padding: 10px 24px 30px;
  }
  .err500__actions-row {
    flex-direction: column;
  }
  .err500__evidence-inner {
    padding: 24px 20px;
  }
  .err500__bg-number span {
    font-size: 440px;
  }
}
</style>
