<template>
  <div class="sc">
    <div class="sc__card">
      <header class="sc__header">
        <h1>Системное управление</h1>
        <p class="sc__lede">
          Скрытая страница управления режимом технических работ. Доступна только
          супер-администратору по прямой ссылке, в меню не показывается.
        </p>
      </header>

      <!-- Две колонки: слева объявление и сроки, справа состояние и действия.
           Форма и кнопки видны одновременно, без прокрутки страницы. -->
      <div class="sc__columns">
        <div class="sc__col">
          <section class="sc__form">
            <label class="sc__field">
              <span class="sc__field-label">Сообщение для пользователей</span>
              <textarea
                v-model="draftMessage"
                class="sc__textarea"
                rows="3"
                placeholder="Например: обновляем систему до версии 1.5.0, вернёмся к 17:00"
                :disabled="busy"
              />
            </label>

            <div class="sc__field-row">
              <div class="sc__field">
                <span class="sc__field-label">Начало работ</span>
                <div class="sc__when">
                  <DateFilter
                    class="sc__date"
                    mode="single"
                    data-testid="planned-start"
                    :selected-date="startDate"
                    @update:selected-date="startDate = $event"
                  />
                  <input
                    v-model="startTime"
                    class="sc__input sc__input--time"
                    data-testid="planned-start-time"
                    placeholder="чч:мм"
                    inputmode="numeric"
                    maxlength="5"
                    :disabled="busy"
                    @input="startTime = maskTime($event.target.value)"
                    @blur="startTime = normalizeTime(startTime)"
                  >
                </div>
              </div>
              <div class="sc__field">
                <span class="sc__field-label">Окончание работ</span>
                <div class="sc__when">
                  <DateFilter
                    class="sc__date"
                    mode="single"
                    data-testid="planned-end"
                    :selected-date="endDate"
                    @update:selected-date="endDate = $event"
                  />
                  <input
                    v-model="endTime"
                    class="sc__input sc__input--time"
                    data-testid="planned-end-time"
                    placeholder="чч:мм"
                    inputmode="numeric"
                    maxlength="5"
                    :disabled="busy"
                    @input="endTime = maskTime($event.target.value)"
                    @blur="endTime = normalizeTime(endTime)"
                  >
                </div>
              </div>
            </div>

            <div class="sc__field-row">
              <label class="sc__field">
                <span class="sc__field-label">Почта поддержки</span>
                <input
                  v-model="draftSupportEmail"
                  type="email"
                  class="sc__input"
                  placeholder="support@buropropuskov.ru"
                  :disabled="busy"
                >
              </label>
              <label class="sc__field">
                <span class="sc__field-label">Телефон поддержки</span>
                <input
                  v-model="draftSupportPhone"
                  type="tel"
                  class="sc__input"
                  data-testid="support-phone"
                  placeholder="+7 (495) 123 45-67"
                  :disabled="busy"
                  @input="draftSupportPhone = formatRussianPhone($event.target.value)"
                >
              </label>
            </div>
            <p class="sc__field-note">
              Сообщение и контакты видит каждый, кого система не пускает внутрь. Окно
              работ показывается пользователям как срок и одновременно служит
              предохранителем: по его окончании режим выключается автоматически.
            </p>
          </section>
        </div>
        <aside class="sc__col sc__col--side">
          <section class="sc__status">
            <div class="sc__status-row">
              <span class="sc__status-label">Текущий статус</span>
              <span
                class="sc__status-pill"
                :class="{ 'sc__status-pill--on': enabled }"
              >
                {{ enabled ? 'ТЕХНИЧЕСКИЕ РАБОТЫ ВКЛЮЧЕНЫ' : 'Сервис работает нормально' }}
              </span>
            </div>
            <p
              v-if="enabled && startedAt"
              class="sc__status-meta"
            >
              Включено: {{ formatMoment(startedAt) }}
            </p>
            <p
              v-if="enabled && plannedEnd"
              class="sc__status-meta"
            >
              Объявленное окончание: {{ formatMoment(plannedEnd) }} — после него режим снимется сам
            </p>
          </section>
          <section class="sc__actions">
            <button
              v-if="!enabled"
              class="sc__btn sc__btn--primary"
              data-testid="enable-btn"
              :disabled="busy"
              @click="confirmEnable"
            >
              Включить технические работы
            </button>
            <template v-else>
              <button
                class="sc__btn sc__btn--primary"
                data-testid="save-btn"
                :disabled="busy"
                @click="enable"
              >
                Сохранить сообщение и сроки
              </button>
              <button
                class="sc__btn sc__btn--danger"
                data-testid="disable-btn"
                :disabled="busy"
                @click="disable"
              >
                Выключить технические работы
              </button>
            </template>
            <p class="sc__hint">
              При включении <strong>отзываются все сеансы обычных пользователей</strong> —
              в течение 15 минут их выбросит на страницу «Технические работы», и войти
              заново они не смогут, пока режим активен. Супер-администратор продолжает
              работать без ограничений.
            </p>
            <p class="sc__hint">
              Если войти в систему не получается, режим снимается на сервере командой
              <code>make maintenance-off</code> (для рабочего сервера —
              <code>make deploy-maintenance-off</code>). Пользователи вернутся в систему
              в течение 10 секунд.
            </p>
          </section>
        </aside>
      </div>

      <div
        v-if="errorText"
        class="sc__error"
      >
        {{ errorText }}
      </div>
    </div>

    <!-- Подтверждение включения -->
    <teleport to="body">
      <transition name="sc-fade">
        <div
          v-if="confirmOpen"
          class="sc__modal-overlay"
          @click.self="confirmOpen = false"
        >
          <div class="sc__modal">
            <h2>Включить режим технических работ?</h2>
            <p>
              Сеансы всех пользователей, кроме супер-администратора, будут
              прекращены. Режим нужен на время обновления системы или работ с
              базой данных.
            </p>
            <p
              v-if="plannedStartIso && plannedEndIso"
              class="sc__modal-window"
            >
              Объявленное окно: {{ formatMoment(plannedStartIso) }} — {{ formatMoment(plannedEndIso) }}
            </p>
            <div class="sc__modal-actions">
              <button
                class="sc__btn"
                @click="confirmOpen = false"
              >
                Отмена
              </button>
              <button
                class="sc__btn sc__btn--primary"
                :disabled="busy"
                @click="enable"
              >
                Да, включить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
  </div>
</template>

<script>
import DateFilter from '@/components/DateFilter.vue'
import { apiRequest } from '@/api/client'
import { formatRussianPhone, formatRussianPhoneForDisplay } from '@/composables/useRussianPhoneMask'
import { useMaintenanceStore } from '@/stores/maintenance'
import { formatDateTime } from '@/utils/datetime'

const DEFAULT_WINDOW_HOURS = 2
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/

export default {
  name: 'SystemControl',
  components: { DateFilter },
  data() {
    return {
      enabled: false,
      message: '',
      startedAt: '',
      plannedEnd: '',
      supportEmail: '',
      draftMessage: '',
      startDate: null,
      startTime: '',
      endDate: null,
      endTime: '',
      draftSupportEmail: 'support@buropropuskov.ru',
      draftSupportPhone: '',
      busy: false,
      errorText: '',
      confirmOpen: false,
    }
  },
  computed: {
    plannedStartIso() {
      return this.toIso(this.startDate, this.startTime)
    },
    plannedEndIso() {
      return this.toIso(this.endDate, this.endTime)
    },
  },
  async mounted() {
    await this.load()
  },
  methods: {
    // Маска телефона общая с остальными формами проекта - шаблону она нужна
    // как метод, поэтому импортированная функция прокидывается сюда.
    formatRussianPhone,
    async load() {
      this.errorText = ''
      try {
        const r = await apiRequest('/admin/maintenance', { method: 'GET' })
        if (!r.ok) {
          this.errorText = 'Не удалось загрузить текущий статус.'
          return
        }
        const data = await r.json()
        this.apply(data)
      } catch {
        this.errorText = 'Сеть недоступна.'
      }
    },
    apply(data) {
      this.enabled = !!data?.enabled
      this.message = data?.message || ''
      this.startedAt = data?.started_at || ''
      this.plannedEnd = data?.planned_end || ''
      this.supportEmail = data?.support_email || ''
      this.draftMessage = this.message
      if (data?.support_email) this.draftSupportEmail = data.support_email
      this.draftSupportPhone = formatRussianPhoneForDisplay(data?.support_phone)
      this.fillWindow(data?.planned_start, data?.planned_end)
      useMaintenanceStore().setFromPayload(data)
    },
    /**
     * Заполняет поля окна сохранёнными значениями, а для незаданного окна -
     * подсказкой «ближайшие два часа»: пустые поля админ всё равно обязан
     * заполнить, а вводить дату с нуля каждый раз незачем.
     */
    fillWindow(plannedStart, plannedEnd) {
      const start = plannedStart ? new Date(plannedStart) : new Date()
      const end = plannedEnd
        ? new Date(plannedEnd)
        : new Date(start.getTime() + DEFAULT_WINDOW_HOURS * 60 * 60 * 1000)
      this.startDate = start
      this.startTime = this.timeOf(start)
      this.endDate = end
      this.endTime = this.timeOf(end)
    },
    /** Часы и минуты момента в виде 'ЧЧ:ММ'. */
    timeOf(date) {
      const p = (n) => String(n).padStart(2, '0')
      return `${p(date.getHours())}:${p(date.getMinutes())}`
    },
    /** Двоеточие подставляется по мере ввода, как в сроках заявки. */
    maskTime(raw) {
      const digits = String(raw).replace(/\D/g, '').slice(0, 4)
      return digits.length <= 2 ? digits : `${digits.slice(0, 2)}:${digits.slice(2)}`
    },
    /**
     * Дополняет неполный ввод до 'ЧЧ:ММ' и загоняет в границы суток:
     * '9' -> 09:00, '930' -> 09:30, '0930' -> 09:30, '2599' -> 23:59.
     * Три цифры читаются как Ч ММ - так их и набирают.
     */
    normalizeTime(value) {
      const digits = String(value).replace(/\D/g, '')
      if (!digits) return ''
      let hours = digits.slice(0, 2)
      let minutes = '0'
      if (digits.length === 3) {
        hours = digits.slice(0, 1)
        minutes = digits.slice(1)
      } else if (digits.length === 4) {
        minutes = digits.slice(2)
      }
      const pad = (n) => String(n).padStart(2, '0')
      return `${pad(Math.min(23, Number(hours)))}:${pad(Math.min(59, Number(minutes)))}`
    },
    /** Дата из календаря плюс время из поля - в момент для API. */
    toIso(date, time) {
      if (!date || !TIME_RE.test(time)) return ''
      const [hours, minutes] = time.split(':').map(Number)
      const moment = new Date(date)
      moment.setHours(hours, minutes, 0, 0)
      return moment.toISOString()
    },
    /**
     * Проверяет окно до отправки. Возвращает текст ошибки или пустую строку.
     */
    validateWindow() {
      if (!this.startDate || !this.endDate) {
        return 'Выберите даты начала и окончания технических работ.'
      }
      if (!TIME_RE.test(this.startTime) || !TIME_RE.test(this.endTime)) {
        return 'Укажите время в формате ЧЧ:ММ.'
      }
      if (new Date(this.plannedEndIso) <= new Date(this.plannedStartIso)) {
        return 'Окончание работ должно быть позже начала.'
      }
      return ''
    },
    confirmEnable() {
      this.errorText = this.validateWindow()
      if (this.errorText) return
      this.confirmOpen = true
    },
    async enable() {
      this.errorText = this.validateWindow()
      if (this.errorText) {
        this.confirmOpen = false
        return
      }
      this.busy = true
      try {
        const r = await apiRequest('/admin/maintenance', {
          method: 'PUT',
          body: JSON.stringify({
            enabled: true,
            message: this.draftMessage,
            planned_start: this.plannedStartIso,
            planned_end: this.plannedEndIso,
            support_email: this.draftSupportEmail,
            support_phone: this.draftSupportPhone,
          }),
        })
        if (!r.ok) {
          this.errorText = 'Не удалось сохранить режим технических работ.'
          return
        }
        const data = await r.json()
        this.apply(data)
        this.confirmOpen = false
      } catch {
        this.errorText = 'Сеть недоступна.'
      } finally {
        this.busy = false
      }
    },
    async disable() {
      this.busy = true
      this.errorText = ''
      try {
        const r = await apiRequest('/admin/maintenance', {
          method: 'PUT',
          body: JSON.stringify({ enabled: false }),
        })
        if (!r.ok) {
          this.errorText = 'Не удалось выключить режим.'
          return
        }
        const data = await r.json()
        this.apply(data)
      } catch {
        this.errorText = 'Сеть недоступна.'
      } finally {
        this.busy = false
      }
    },
    formatMoment(iso) {
      return formatDateTime(iso) || '—'
    },
  },
}
</script>

<style scoped>
.sc {
  /* zoom-safe (#1097): vh под корневым zoom меряется от НЕзумленной высоты. */
  min-height: calc(var(--app-vh, 1vh) * 100 - 80px);
  /* Обычный фон рабочей области: accent-tint заливал акцентом весь экран, и страница
     выбивалась из остальной админки синеватым полем. */
  background: var(--bg);
  padding: 40px 24px;
  display: flex;
  justify-content: center;
  /* Карточка по высоте контента: в две колонки он ниже экрана, и растянутая
     карточка выглядела пустой коробкой. */
  align-items: flex-start;
}
.sc__card {
  width: 100%;
  max-width: 1120px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  padding: 36px 40px 32px;
}
.sc__header h1 {
  margin: 0 0 10px;
  font-weight: 700;
  font-size: 24px;
  color: var(--text);
}
.sc__lede {
  margin: 0 0 28px;
  font-size: 14px;
  line-height: 1.55;
  color: var(--text-muted);
}
.sc__lede code {
  background: var(--accent-tint);
  padding: 1px 6px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 12px;
}
/* Горизонтальная раскладка: форма и панель управления рядом, а не стопкой. */
.sc__columns {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 28px;
  align-items: start;
}
.sc__col {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.sc__status {
  padding: 16px 20px;
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 20px;
}
.sc__status-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.sc__status-label {
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
}
.sc__status-pill {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.5px;
  text-transform: uppercase;
  padding: 6px 14px;
  border-radius: 999px;
  background: var(--success-bg);
  color: var(--success-text);
}
.sc__status-pill--on {
  background: var(--danger-bg);
  color: var(--danger-text);
}
.sc__status-meta {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}
.sc__form {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.sc__field {
  display: block;
  min-width: 0;
}
.sc__field-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 18px;
}
/* Дата календарём, время рядом отдельным полем - как в сроках заявки. */
.sc__when {
  display: flex;
  align-items: stretch;
  gap: 10px;
}
.sc__date { flex: 1; min-width: 0; }
/* У DateFilter ширина поля зашита в 215px - в колонке формы он должен тянуться
   по месту, иначе пара «дата + время» вылезает за край. */
.sc__date :deep(.date-filter),
.sc__date :deep(.date-field) {
  width: 100%;
}
.sc__input--time {
  width: 84px;
  flex: 0 0 84px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.sc__field-note {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-muted);
}
.sc__field-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 8px;
}
.sc__textarea,
.sc__input {
  width: 100%;
  padding: 12px 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
  color: var(--text);
  background: var(--surface);
  resize: vertical;
}
.sc__textarea:focus,
.sc__input:focus {
  outline: none;
  border-color: var(--accent);
}
.sc__actions {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.sc__btn {
  font-family: inherit;
  font-size: 14px;
  font-weight: 500;
  padding: 12px 28px;
  border-radius: 30px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
}
.sc__btn:hover:not(:disabled) { background: #f3f4f9; }
.sc__btn:disabled { opacity: 0.6; cursor: not-allowed; }
.sc__btn--primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}
.sc__btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}
.sc__btn--danger {
  background: var(--danger);
  color: var(--fill-text);
  border-color: var(--danger);
}
.sc__btn--danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--danger) 85%, var(--text));
  border-color: var(--danger);
}
.sc__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
  color: var(--text-muted);
}
.sc__hint code {
  background: var(--accent-tint);
  padding: 1px 4px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 11px;
}
.sc__error {
  margin-top: 20px;
  padding: 12px 16px;
  border-radius: 12px;
  background: var(--danger-bg);
  color: var(--danger-text);
  font-size: 13px;
}

.sc__modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}
.sc__modal {
  background: var(--surface);
  border-radius: 30px;
  padding: 32px 36px 28px;
  max-width: 480px;
  width: 100%;
  box-shadow: 0 20px 60px var(--shadow-drop);
}
.sc__modal h2 {
  margin: 0 0 14px;
  font-size: 20px;
  font-weight: 700;
}
.sc__modal p {
  margin: 0 0 24px;
  font-size: 14px;
  line-height: 1.55;
  color: var(--text-muted);
}
.sc__modal-window {
  color: var(--text);
  font-weight: 500;
}
.sc__modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}
.sc-fade-enter-active,
.sc-fade-leave-active {
  transition: opacity 0.2s ease;
}
.sc-fade-enter-from,
.sc-fade-leave-to { opacity: 0; }

/* Узкий экран: колонки схлопываются, состояние и кнопки уходят наверх -
   сначала «что сейчас», потом «что менять». */
@media (max-width: 1000px) {
  .sc__columns { grid-template-columns: 1fr; }
  .sc__col--side { order: -1; }
}

@media (max-width: 768px) {
  .sc__card { padding: 28px 24px; }
  .sc__status-row { flex-direction: column; align-items: flex-start; gap: 8px; }
  .sc__field-row { grid-template-columns: 1fr; }

  /* Bottom-sheet modal на мобильном - прилипает к низу, высота по контенту */
  .sc__modal-overlay {
    padding: 0;
    align-items: flex-end;
  }

  .sc__modal {
    width: 100vw;
    max-width: 100vw;
    max-height: 90dvh;
    border-radius: 16px 16px 0 0;
    margin: 0;
    overflow-y: auto;
  }

  .sc__modal-actions {
    flex-direction: column;
    gap: 10px;
  }

  .sc__modal-actions .sc__btn {
    width: 100%;
  }
}
</style>
