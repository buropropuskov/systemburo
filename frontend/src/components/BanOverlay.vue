<template>
  <Teleport to="body">
    <transition name="ban-fade">
      <div
        v-if="isBanned"
        class="ban-overlay"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="ban-title"
        @click.self="blockClose"
      >
        <div class="ban-modal">
          <div class="ban-modal__top">
            <div
              class="ban-modal__icon"
              aria-hidden="true"
            >
              <svg
                width="30"
                height="30"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.9"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <rect
                  x="4"
                  y="11"
                  width="16"
                  height="10"
                  rx="2"
                />
                <path d="M8 11V7a4 4 0 0 1 8 0v4" />
              </svg>
            </div>
            <h2
              id="ban-title"
              class="ban-modal__title"
            >
              Аккаунт заблокирован
            </h2>
            <p class="ban-modal__subtitle">
              Доступ к системе ограничен администратором
            </p>
          </div>

          <div class="ban-modal__body">
            <div
              v-if="banReason"
              class="ban-reason"
            >
              <div class="ban-reason__label">
                Причина блокировки
              </div>
              <div class="ban-reason__text">
                {{ banReason }}
              </div>
            </div>

            <p class="ban-modal__hint">
              Если вы считаете, что блокировка ошибочна, обратитесь к администратору системы.
            </p>

            <div
              v-if="hasContacts"
              class="ban-contacts"
            >
              <div class="ban-contacts__label">
                Контакты Бюро пропусков
              </div>
              <a
                v-if="bureauPhone"
                class="ban-contacts__item"
                :href="`tel:${bureauPhone}`"
              >{{ bureauPhone }}</a>
              <a
                v-if="bureauEmail"
                class="ban-contacts__item"
                :href="`mailto:${bureauEmail}`"
              >{{ bureauEmail }}</a>
            </div>

            <div class="ban-modal__actions">
              <button
                type="button"
                class="ban-modal__logout"
                @click="handleLogout"
              >
                Выйти из системы
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { usePermissionsStore } from '@/stores/permissions'
import { useContactsStore } from '@/stores/contacts'

/**
 * Полноэкранный неснимаемый оверлей для забаненного пользователя (Фаза 4
 * эпика прав). Источник статуса - permissions-стор (banned/banReason из
 * GET /permissions/my). Лежит выше всей лестницы модалок (z-index 26000),
 * но ниже тоста (29000), чтобы уведомление "Учётная запись заблокирована"
 * оставалось видно. Закрыть нельзя - выход только кнопкой или после
 * разблокировки администратором (стор обновится на навигации, ≤30s).
 */
export default {
  name: 'BanOverlay',
  emits: ['logout'],
  computed: {
    isBanned() {
      return usePermissionsStore().banned
    },
    banReason() {
      return usePermissionsStore().banReason
    },
    bureauPhone() {
      return useContactsStore().phone
    },
    bureauEmail() {
      return useContactsStore().email
    },
    hasContacts() {
      return useContactsStore().hasAny
    },
  },
  mounted() {
    useContactsStore().fetch()
  },
  methods: {
    handleLogout() {
      this.$emit('logout')
    },
    blockClose() {
      // Блокирующий takeover: клик по оверлею не закрывает.
    },
  },
}
</script>

<style scoped>
.ban-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  /* Бан - перманентный блокирующий takeover: поверх любых стопок модалок
     (деталь/карточка/override/история ≤12000, confirm 20000, session 25000).
     Ниже тоста (29000) - уведомление о блокировке остаётся видно. */
  z-index: 26000;
  background:
    radial-gradient(900px 500px at 50% 18%, rgba(220, 53, 69, 0.1), transparent 70%),
    rgba(120, 18, 28, 0.62);
  backdrop-filter: blur(3px) saturate(0.85);
  -webkit-backdrop-filter: blur(3px) saturate(0.85);
}

.ban-modal {
  width: 480px;
  max-width: 95vw;
  background: var(--surface);
  border-radius: 30px;
  overflow: hidden;
  box-shadow: 0 30px 80px rgba(70, 6, 12, 0.5);
  text-align: center;
}

.ban-modal__top {
  padding: 30px 28px 26px;
  color: var(--accent-contrast);
  background: linear-gradient(135deg, var(--danger), color-mix(in srgb, var(--danger) 75%, var(--text)));
}

.ban-modal__icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 14px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.16);
}

.ban-modal__title {
  font-size: 21px;
  font-weight: 700;
}

.ban-modal__subtitle {
  margin-top: 6px;
  font-size: 13px;
  font-weight: 500;
  opacity: 0.92;
}

.ban-modal__body {
  padding: 24px 28px 26px;
}

.ban-reason {
  text-align: left;
  padding: 14px 16px;
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: var(--radius-md, 15px);
  background: var(--danger-bg);
}

.ban-reason__label {
  margin-bottom: 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--color-danger, var(--danger-text));
}

.ban-reason__text {
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text, var(--text));
  overflow-wrap: anywhere;
}

.ban-modal__hint {
  margin-top: 18px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-muted, var(--text-muted));
}

.ban-contacts {
  margin-top: 16px;
  padding: 14px 16px;
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-md, 15px);
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ban-contacts__label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--color-text-muted, var(--text-muted));
  margin-bottom: 2px;
}

.ban-contacts__item {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-primary, var(--accent-text));
  text-decoration: none;
  overflow-wrap: anywhere;
}

.ban-contacts__item:hover {
  text-decoration: underline;
}

.ban-modal__actions {
  margin-top: 22px;
}

.ban-modal__logout {
  width: 100%;
  padding: 11px 20px;
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-pill, 999px);
  background: transparent;
  color: var(--color-text, var(--text));
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}

.ban-modal__logout:hover {
  background: var(--accent-tint);
  border-color: var(--border);
}

/* Анимация: только transform/opacity (rules/web). */
.ban-fade-enter-active {
  transition: opacity 0.25s ease-out;
}
.ban-fade-leave-active {
  transition: opacity 0.2s ease-in;
}
.ban-fade-enter-from,
.ban-fade-leave-to {
  opacity: 0;
}
.ban-fade-enter-active .ban-modal {
  transition: transform 0.25s ease-out;
}
.ban-fade-enter-from .ban-modal {
  transform: translateY(16px);
}

@media (max-width: 480px) {
  .ban-modal__top {
    padding: 26px 22px 22px;
  }
  .ban-modal__body {
    padding: 22px 22px 24px;
  }
}
</style>
