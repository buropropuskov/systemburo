<template>
  <div class="sc">
    <div class="sc__card">
      <header class="sc__header">
        <h1>Системное управление</h1>
        <p class="sc__lede">
          Скрытая страница управления режимом технических работ. Доступна только
          супер-админу (<code>type_id = 6</code>) по прямой ссылке, в меню не
          показывается.
        </p>
      </header>

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
          Начало: {{ formatStart(startedAt) }}
        </p>
      </section>

      <section class="sc__form">
        <label class="sc__field">
          <span class="sc__field-label">Сообщение для пользователей</span>
          <textarea
            v-model="draftMessage"
            class="sc__textarea"
            rows="3"
            placeholder="Например: Обновляем систему до v1.5.0, вернёмся к 17:00 МСК"
            :disabled="busy"
          />
        </label>
        <label class="sc__field">
          <span class="sc__field-label">Контакт поддержки (email)</span>
          <input
            v-model="draftSupportEmail"
            type="email"
            class="sc__input"
            placeholder="support@buropropuskov.ru"
            :disabled="busy"
          >
        </label>
      </section>

      <section class="sc__actions">
        <button
          v-if="!enabled"
          class="sc__btn sc__btn--primary"
          :disabled="busy"
          @click="confirmEnable"
        >
          Включить технические работы
        </button>
        <button
          v-else
          class="sc__btn sc__btn--danger"
          :disabled="busy"
          @click="disable"
        >
          Выключить технические работы
        </button>
        <p class="sc__hint">
          При включении <strong>отзываются все refresh-токены не-админов</strong> —
          через ≤15 минут обычных юзеров выбросит на страницу «Технические работы»
          и они не смогут залогиниться, пока режим активен. Супер-админ
          (<code>type_id = 6</code>) продолжает работать без ограничений.
        </p>
      </section>

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
              Все активные сессии не-админов будут отозваны. Это действие нужно
              только если вы делаете обновление системы или миграцию БД.
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
import { apiRequest } from '@/api/client'
import { useMaintenanceStore } from '@/stores/maintenance'

export default {
  name: 'SystemControl',
  data() {
    return {
      enabled: false,
      message: '',
      startedAt: '',
      supportEmail: '',
      draftMessage: '',
      draftSupportEmail: 'support@buropropuskov.ru',
      busy: false,
      errorText: '',
      confirmOpen: false,
    }
  },
  async mounted() {
    await this.load()
  },
  methods: {
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
      this.supportEmail = data?.support_email || ''
      this.draftMessage = this.message
      if (data?.support_email) this.draftSupportEmail = data.support_email
      useMaintenanceStore().setFromPayload(data)
    },
    confirmEnable() {
      this.confirmOpen = true
    },
    async enable() {
      this.busy = true
      this.errorText = ''
      try {
        const r = await apiRequest('/admin/maintenance', {
          method: 'PUT',
          body: JSON.stringify({
            enabled: true,
            message: this.draftMessage,
            support_email: this.draftSupportEmail,
          }),
        })
        if (!r.ok) {
          this.errorText = 'Не удалось включить режим.'
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
    formatStart(iso) {
      if (!iso) return '—'
      const d = new Date(iso)
      return d.toLocaleString('ru-RU', {
        day: '2-digit', month: 'short', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
      }) + ' МСК'
    },
  },
}
</script>

<style scoped>
.sc {
  /* zoom-safe (#1097): vh под корневым zoom меряется от НЕзумленной высоты. */
  min-height: calc(var(--app-vh, 1vh) * 100 - 80px);
  background: var(--accent-tint);
  padding: 40px 24px;
  display: flex;
  justify-content: center;
}
.sc__card {
  width: 100%;
  max-width: 720px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 30px;
  box-shadow: 0 3px 10px var(--shadow-drop);
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
.sc__status {
  padding: 16px 20px;
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 20px;
  margin-bottom: 28px;
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
  margin-bottom: 28px;
}
.sc__field { display: block; }
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

@media (max-width: 768px) {
  .sc__card { padding: 28px 24px; }
  .sc__status-row { flex-direction: column; align-items: flex-start; gap: 8px; }

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
