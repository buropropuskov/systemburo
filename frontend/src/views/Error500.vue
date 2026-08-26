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
      <button
        class="err500__brand"
        type="button"
        data-testid="error-500-brand"
        @click="goHome"
      >
        Systemburo
      </button>
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
          Мы уже разбираемся. Отчёт отправит детали инцидента разработчикам - так починим быстрее.
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
              Спасибо - команда получила отчёт и уже смотрит.
            </template>
            <template v-else-if="reportError">
              {{ reportError }}
            </template>
            <template v-else>
              Уйдут маршрут, код, ID инцидента и время - без тела ответа сервера.
            </template>
          </p>

          <div class="err500__actions-row">
            <button
              v-if="retryPath"
              class="err500__btn err500__btn--outlined"
              data-testid="error-500-retry"
              @click="retry"
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
              <span class="err500__btn-wide">Повторить попытку</span>
              <span class="err500__btn-narrow">Повторить</span>
            </button>
            <button
              class="err500__btn err500__btn--outlined"
              data-testid="error-500-home"
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
          </div>
        </div>
      </section>

      <aside class="err500__evidence">
        <div class="err500__evidence-inner">
          <div class="err500__evidence-label">
            <span>Системный инцидент</span>
            <span>зафиксирован</span>
          </div>
          <pre class="err500__code"><span class="c500-c"># server response
</span><span class="c500-k">STATUS</span>   <span class="c500-s">{{ ctx.httpStatus }}</span> {{ statusText }}
<span class="c500-k">REQUEST</span>  {{ ctx.route }}<template v-if="retryPath">
<span class="c500-k">PAGE</span>     {{ retryPath }}</template>
<span class="c500-k">ID</span>       {{ bugHash || '...' }}
<span class="c500-k">TIME</span>     {{ formattedTime }}</pre>
          <p class="err500__note">
            Детали исключения остаются в защищённом серверном логе - в поддержку хватит ID инцидента.
          </p>
        </div>
      </aside>
    </main>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
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
        uiRoute: '',
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
    /** Куда уводит "На главную": гостю показывать ленту новостей нечего. */
    homePath() {
      return useAuthStore().isAuthenticated ? '/news' : '/'
    },
    /**
     * Страница, на которой упал запрос. Пустая строка = повторять нечего:
     * адреса нет, это сама /500 или тот же адрес, куда ведёт "На главную" -
     * во всех трёх случаях вторая кнопка была бы дублем первой.
     */
    retryPath() {
      const route = this.ctx.uiRoute || ''
      if (!route || route.startsWith('/500') || route === this.homePath) return ''
      return route
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
      this.$router.push(this.homePath)
    },
    /**
     * Повтор упавшего запроса: возвращаемся на страницу, которая его слала, и
     * она грузит данные заново. replace, а не push - страница инцидента в
     * истории не нужна, кнопка "назад" браузера должна вести дальше в прошлое.
     */
    retry() {
      this.$router.replace(this.retryPath || this.homePath)
    },
  },
}
</script>

<style scoped>
.err500 {
  /* Страница-заглушка живёт ровно в одном экране: скролл на ней некуда вести,
     а обрезка нижних кнопок оставляла бы человека без выхода. Отсюда height, а
     не min-height, и внутренние размеры в долях высоты - см. блоки ниже.
     zoom-safe (#1097): под корневым CSS zoom (viewportScale) единица vh считается
     от НЕзумленной высоты и раздувает блок в zoom раз -> низ уезжает за экран.
     --app-vh уже нормирован на текущий zoom. */
  height: calc(var(--app-vh, 1vh) * 100);
  /* B.3 (#1097): svh стабилизирует высоту на мобилке (адрес-бар браузера), min() держит
     zoom-корректность на десктопе; при отсутствии svh каскад откатится на calc выше. */
  height: min(calc(var(--app-vh, 1vh) * 100), 100svh);
  /* Клапан для экстремально низких окон (телефон в альбомной ориентации): в
     обычных размерах контент уже помещается, и скроллбар не появляется. */
  overflow: auto;
  /* Подложка от темы: литералы держали страницу светлой, и в тёмной теме
     светло-серый текст ложился на почти белый фон. Пятна - примесь акцента и
     аварийного цвета к фону, поэтому в каждой теме они в её тоне. */
  background:
    radial-gradient(1200px 700px at 15% 0%, color-mix(in srgb, var(--accent) 12%, var(--bg)) 0%, transparent 55%),
    radial-gradient(900px 600px at 100% 100%, color-mix(in srgb, var(--danger) 12%, var(--bg)) 0%, transparent 50%),
    var(--bg);
  position: relative;
  display: flex;
  flex-direction: column;
  color: var(--text);
}
.err500__grid {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--accent) 12%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--accent) 12%, transparent) 1px, transparent 1px);
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
  font-size: clamp(320px, calc(var(--app-vh, 1vh) * 82), 900px);
  line-height: 0.78;
  color: var(--accent-text);
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
  gap: 16px;
  padding: clamp(14px, calc(var(--app-vh, 1vh) * 2.6), 28px) clamp(20px, 4vw, 48px);
}
.err500__brand {
  font-family: inherit;
  font-weight: 700;
  font-size: 15px;
  letter-spacing: 0.5px;
  color: var(--text);
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: color 0.15s ease;
}
.err500__brand:hover {
  color: var(--accent-text);
}
.err500__chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  border-radius: 999px;
  background: var(--danger-bg);
  color: var(--danger-text);
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
  background: var(--danger);
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
  min-height: 0;
  display: grid;
  /* minmax(0, …) вместо голых fr: у grid-элемента min-width по умолчанию auto,
     и консоль с длинными строками распирала колонку шире экрана - оверфлоу
     уходил под overflow:hidden контейнера и резал заголовок с кнопками. */
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
  align-items: center;
  gap: clamp(32px, 5vw, 80px);
  padding: 0 clamp(20px, 6vw, 96px) clamp(20px, calc(var(--app-vh, 1vh) * 4), 56px);
}
.err500__hero {
  min-width: 0;
}
.err500__kicker {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 4px;
  color: var(--danger-text);
  text-transform: uppercase;
  margin-bottom: clamp(12px, calc(var(--app-vh, 1vh) * 2.2), 24px);
}
.err500__kicker::before {
  content: '';
  width: 36px;
  height: 2px;
  background: var(--danger);
}
.err500__title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 800;
  /* Размер ограничен и высотой окна: на низком экране (ноутбук 1280x800,
     телефон в альбомной) заголовок в 80px съедал место у кнопок выхода. */
  font-size: clamp(30px, min(5.4vw, calc(var(--app-vh, 1vh) * 7.4)), 72px);
  line-height: 1.04;
  letter-spacing: -0.025em;
  margin: 0 0 clamp(14px, calc(var(--app-vh, 1vh) * 2.6), 28px);
  max-width: 820px;
}
.err500__title em {
  font-style: normal;
  color: var(--accent-text);
  font-weight: 800;
}
.err500__lede {
  font-size: clamp(14px, 1.1vw, 17px);
  line-height: 1.55;
  color: var(--text-muted);
  margin: 0 0 clamp(16px, calc(var(--app-vh, 1vh) * 3), 32px);
  max-width: 540px;
}
.err500__actions {
  display: flex;
  flex-direction: column;
  gap: clamp(10px, calc(var(--app-vh, 1vh) * 1.4), 14px);
  max-width: 540px;
}
.err500__btn {
  font-family: inherit;
  font-size: 15px;
  font-weight: 500;
  padding: clamp(11px, calc(var(--app-vh, 1vh) * 1.6), 14px) 28px;
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
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}
.err500__btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}
.err500__btn--primary:disabled {
  background: var(--surface-2);
  color: var(--text-muted);
  border-color: var(--border);
  cursor: not-allowed;
}
.err500__btn--outlined {
  background: transparent;
  color: var(--accent-text);
  border-color: var(--accent);
}
.err500__btn--outlined:hover {
  background: var(--accent-tint);
}
.err500__actions-row {
  display: flex;
  gap: 12px;
}
/* Подпись кнопки короче на узком экране - перенос в две строки поднимал ряд
   выхода на 12px, и консоль инцидента переставала помещаться на 360x640. */
.err500__btn-narrow {
  display: none;
}
.err500__actions-row .err500__btn {
  flex: 1;
}
.err500__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-muted);
}
.err500__hint--success {
  color: var(--success-text);
  font-weight: 500;
}
.err500__hint--error {
  color: var(--danger-text);
}
.err500__evidence {
  position: relative;
  min-width: 0;
}
.err500__evidence-inner {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  padding: clamp(20px, calc(var(--app-vh, 1vh) * 3.4), 36px);
  box-shadow: 0 3px 10px var(--shadow-drop);
}
.err500__evidence-label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 3px;
  text-transform: uppercase;
  color: var(--text-muted);
  margin-bottom: 14px;
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.err500__code {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: clamp(11.5px, 1vw, 14.5px);
  /* Панель намеренно тёмная в любой теме - это стилизованная консоль, а не
     акцентная плашка. Литералы осознанные, тема их не красит. */
  background: var(--console-bg);
  color: #a5b4fc;
  padding: clamp(16px, calc(var(--app-vh, 1vh) * 2.4), 22px) clamp(16px, 2vw, 26px);
  border-radius: 20px;
  line-height: 1.7;
  overflow-x: auto;
  margin: 0;
}
.err500__code .c500-c { color: inherit; }
.err500__code .c500-k { color: #fbbf24; }
.err500__code .c500-s { color: #fca5a5; }
.err500__note {
  margin: 14px 0 0;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--text-muted);
}

@media (max-width: 1100px) {
  .err500__main {
    grid-template-columns: minmax(0, 1fr);
    align-content: center;
    /* safe: когда контент всё же выше экрана (низкое окно, крупный шрифт ОС),
       обычный center режет ВЕРХ - заголовок наезжал на шапку и уходил под неё. */
    align-content: safe center;
    padding: 0 clamp(20px, 5vw, 40px) clamp(16px, calc(var(--app-vh, 1vh) * 3), 40px);
    gap: clamp(20px, calc(var(--app-vh, 1vh) * 3), 40px);
  }
  .err500__title {
    font-size: clamp(28px, min(7vw, calc(var(--app-vh, 1vh) * 6)), 56px);
  }
  .err500__lede {
    font-size: 15px;
    max-width: none;
  }
  .err500__actions {
    max-width: none;
  }
  .err500__evidence-inner {
    padding: clamp(18px, calc(var(--app-vh, 1vh) * 2.6), 28px);
  }
}
/* Низкое окно (телефон 360x640, ноутбук с масштабом ОС 125%): режем воздух, а
   не содержимое - иначе консоль инцидента уезжает за нижнюю кромку экрана. */
@media (max-height: 700px) {
  .err500__topbar {
    padding-top: 10px;
    padding-bottom: 10px;
  }
  .err500__main {
    row-gap: 14px;
    padding-bottom: 10px;
  }
  .err500__kicker {
    margin-bottom: 8px;
  }
  .err500__title {
    margin-bottom: 10px;
  }
  .err500__lede {
    margin-bottom: 12px;
  }
}
/* Телефон боком: узкая мерка сказала бы "одна колонка", но по высоте там всего
   ~390px - две колонки укладывают ту же страницу в экран без скролла. */
@media (max-height: 520px) and (min-width: 700px) {
  .err500__topbar {
    padding-top: 8px;
    padding-bottom: 8px;
  }
  .err500__main {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    column-gap: 32px;
    padding-bottom: 8px;
  }
  .err500__lede {
    margin-bottom: 10px;
  }
  .err500__evidence-inner {
    padding: 16px 18px;
  }
  .err500__btn-wide {
    display: none;
  }
  .err500__btn-narrow {
    display: inline;
  }
}
@media (max-width: 640px) {
  .err500__topbar {
    padding: 16px 20px;
  }
  .err500__chip {
    padding: 6px 12px;
    letter-spacing: 1.5px;
  }
  .err500__btn {
    font-size: 14px;
    padding: 11px 18px;
  }
  .err500__actions-row .err500__btn {
    font-size: 13.5px;
    padding: 11px 12px;
    gap: 8px;
  }
  .err500__btn-wide {
    display: none;
  }
  .err500__btn-narrow {
    display: inline;
  }
  .err500__lede {
    font-size: 14px;
  }
  .err500__hint {
    font-size: 12px;
  }
  .err500__evidence-label {
    font-size: 10px;
    letter-spacing: 1.5px;
    white-space: nowrap;
    margin-bottom: 12px;
  }
  .err500__code {
    line-height: 1.6;
  }
  .err500__code .c500-c {
    display: none;
  }
  .err500__note {
    margin-top: 12px;
    font-size: 12px;
  }
  .err500__bg-number span {
    font-size: 320px;
  }
}
</style>
