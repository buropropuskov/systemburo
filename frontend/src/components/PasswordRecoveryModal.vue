<template>
  <BaseModal
    :show="show"
    :closable="false"
    width="440px"
    @close="$emit('close')"
  >
    <div class="recovery-notification-wrapper">
      <transition name="recovery-notification">
        <div
          v-if="showNotification"
          :key="notificationText"
          class="recovery-notification"
          data-testid="recovery-notification"
        >
          {{ notificationText }}
        </div>
      </transition>
    </div>

    <template #header>
      <h2 class="recovery-title">
        Восстановление доступа
      </h2>
    </template>

    <p class="recovery-text">
      Если вы забыли логин или пароль учётной записи, напишите нам или позвоните:
    </p>

    <div class="recovery-contacts">
      <button
        type="button"
        class="recovery-contact"
        data-testid="recovery-copy-email"
        @click.stop="copyEmail"
      >
        <img
          src="@/assets/icons/email-blue.png"
          class="recovery-contact__icon"
          alt=""
        >
        <span class="recovery-contact__text">buropropuskov@dreamisland.ru</span>
      </button>
      <button
        type="button"
        class="recovery-contact"
        data-testid="recovery-copy-phone"
        @click.stop="copyPhone"
      >
        <img
          src="@/assets/icons/phone-blue.png"
          class="recovery-contact__icon"
          alt=""
        >
        <span class="recovery-contact__text">+7 (910) 083 00-55</span>
      </button>
    </div>

    <template #actions>
      <button
        type="button"
        class="recovery-button"
        data-testid="recovery-close"
        @click="$emit('close')"
      >
        Понятно
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
  name: 'PasswordRecoveryModal',
  components: { BaseModal },

  props: {
    show: {
      type: Boolean,
      required: true,
    },
  },

  emits: ['close'],

  data() {
    return {
      showNotification: false,
      notificationText: '',
      notificationTimeout: null,
    }
  },

  beforeUnmount() {
    if (this.notificationTimeout) {
      clearTimeout(this.notificationTimeout)
    }
  },

  methods: {
    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text)
      } catch {
        const textarea = document.createElement('textarea')
        textarea.value = text
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }
    },

    showNotificationMessage(text) {
      if (this.notificationTimeout) {
        clearTimeout(this.notificationTimeout)
        this.notificationTimeout = null
      }
      this.notificationText = text
      this.showNotification = true
      this.notificationTimeout = setTimeout(() => {
        this.showNotification = false
        this.notificationTimeout = null
      }, 2000)
    },

    async copyEmail() {
      await this.copyToClipboard('buropropuskov@dreamisland.ru')
      this.showNotificationMessage('E-mail скопирован')
    },

    async copyPhone() {
      await this.copyToClipboard('+7 (910) 083 00-55')
      this.showNotificationMessage('Номер телефона скопирован')
    },
  },
}
</script>

<style scoped>
/* BaseModal header/actions overrides — убираем border между секциями
   и выравниваем по old_branch-стилю (цельный плоский контент). */
:deep(.base-modal__header) {
  border-bottom: none;
  padding: 24px 32px 16px;
}

:deep(.base-modal__body) {
  padding: 8px 32px 16px;
}

:deep(.base-modal__actions) {
  justify-content: center;
  border-top: none;
  padding: 0 32px 28px;
}

.recovery-title {
  margin: 0;
  font-size: 28px;
  font-weight: 800;
  color: #333;
  text-align: center;
  width: 100%;
}

.recovery-text {
  margin: 0 0 20px;
  font-size: 14px;
  line-height: 1.5;
  color: #000;
  text-align: center;
}

.recovery-contacts {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.recovery-contact {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 6px 10px;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.recovery-contact:hover {
  opacity: 0.8;
}

.recovery-contact__icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.recovery-contact__text {
  font-size: 15px;
  color: var(--color-primary);
  font-weight: 500;
}

.recovery-contact:hover .recovery-contact__text {
  text-decoration: underline;
  text-underline-position: under;
}

.recovery-notification-wrapper {
  position: relative;
  height: 0;
}

.recovery-notification {
  position: fixed;
  top: 18vh;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 26px;
  border-radius: 40px;
  background: var(--color-primary, #4F5BDF);
  font-size: 14px;
  color: #fff;
  font-weight: 600;
  box-shadow: 0 12px 32px rgba(79, 91, 223, 0.4);
  min-width: 220px;
  text-align: center;
  white-space: nowrap;
  z-index: 1100;
}

.recovery-notification-enter-active,
.recovery-notification-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.recovery-notification-enter-from,
.recovery-notification-leave-to {
  opacity: 0;
  transform: translate(-50%, -12px);
}

.recovery-notification-enter-to,
.recovery-notification-leave-from {
  opacity: 1;
  transform: translate(-50%, 0);
}

.recovery-button {
  background: var(--color-primary);
  color: #fff;
  border: none;
  border-radius: 40px;
  padding: 12px 40px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  min-width: 200px;
  margin: 0 auto;
  transition: background-color 0.2s;
}

.recovery-button:hover {
  background-color: #3f4bc9;
}
</style>
