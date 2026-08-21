<template>
  <BaseModal
    :show="show"
    :closable="false"
    width="440px"
    radius="45px"
    theme="light"
    @close="$emit('close')"
  >
    <!-- Отступы и типографика живут на своей разметке внутри слота, а не в
         оверрайдах секций BaseModal: его контент телепортируется в body,
         scope-хэш родителя туда не достаёт, и прежние оверрайды были мёртвыми -
         окно шло с дефолтными 15px радиуса, бордерами между секциями и нулевым
         padding тела, из-за которого текст прилипал к заголовку. -->
    <div
      ref="content"
      class="recovery"
    >
      <h2 class="recovery-title">
        Восстановление доступа
      </h2>

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
          <AppIcon
            name="email"
            class="recovery-contact__icon"
          />
          <span class="recovery-contact__text">{{ bureauEmail }}</span>
        </button>
        <button
          type="button"
          class="recovery-contact"
          data-testid="recovery-copy-phone"
          @click.stop="copyPhone"
        >
          <AppIcon
            name="phone"
            class="recovery-contact__icon"
          />
          <span class="recovery-contact__text">{{ bureauPhone }}</span>
        </button>
      </div>

      <button
        type="button"
        class="recovery-button"
        data-testid="recovery-close"
        @click="$emit('close')"
      >
        Понятно
      </button>
    </div>

    <!-- Пилюля копирования висит над окном, поэтому телепортируется в body:
         внутри окна её обрезал бы скроллящийся контейнер, а на мобилке
         will-change: transform листа сделал бы position: fixed относительным
         листу. Расплата за вынос - вертикаль считается по кромке окна
         (pillStyle), иначе пилюля висит сама по себе высоко над экраном.
         data-theme - тот же светлый остров, что и у окна. -->
    <Teleport to="body">
      <transition name="recovery-notification">
        <div
          v-if="showNotification"
          :key="notificationText"
          class="recovery-notification"
          :style="pillStyle"
          data-theme="light"
          data-testid="recovery-notification"
        >
          {{ notificationText }}
        </div>
      </transition>
    </Teleport>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import AppIcon from '@/components/icons/AppIcon.vue'
import { useContactsStore } from '@/stores/contacts'
import { getViewportZoom } from '@/utils/viewportScale'

// Фолбэк-контакты Бюро, если в настройках системы они ещё не заданы.
const FALLBACK_BUREAU_EMAIL = 'buropropuskov@dreamisland.ru'
const FALLBACK_BUREAU_PHONE = '+7 (910) 083 00-55'

/** Просвет между кромкой окна и пилюлей и её высота из вёрстки - нужны для отступа снизу. */
const PILL_GAP = 14
const PILL_HEIGHT = 25

export default {
  name: 'PasswordRecoveryModal',
  components: { BaseModal, AppIcon },

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
      pillBottom: null,
    }
  },

  computed: {
    /** Пусто -> пилюля садится на фолбэк из стилей (верх экрана). */
    pillStyle() {
      return this.pillBottom === null ? null : { top: 'auto', bottom: `${this.pillBottom}px` }
    },
    bureauEmail() {
      return useContactsStore().email || FALLBACK_BUREAU_EMAIL
    },
    bureauPhone() {
      return useContactsStore().phone || FALLBACK_BUREAU_PHONE
    },
  },

  mounted() {
    useContactsStore().fetch()
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

    /**
     * Отступ пилюли от низа экрана, чтобы она встала прямо над окном.
     * rect приходит в device-px под корневым масштабом, innerHeight - нет,
     * поэтому к layout-px приводятся обе величины (см. utils/viewportScale).
     * Меряется окно целиком, а не контент: на мобилке над контентом ещё ползунок.
     */
    measurePillBottom() {
      const modal = this.$refs.content && this.$refs.content.closest('.base-modal')
      if (!modal || typeof window === 'undefined') return null
      const zoom = getViewportZoom() || 1
      const viewportHeight = window.innerHeight / zoom
      const top = modal.getBoundingClientRect().top / zoom
      const maxBottom = viewportHeight - PILL_HEIGHT - PILL_GAP
      return Math.min(Math.max(PILL_GAP, viewportHeight - top + PILL_GAP), maxBottom)
    },

    showNotificationMessage(text) {
      if (this.notificationTimeout) {
        clearTimeout(this.notificationTimeout)
        this.notificationTimeout = null
      }
      this.notificationText = text
      this.pillBottom = this.measurePillBottom()
      this.showNotification = true
      this.notificationTimeout = setTimeout(() => {
        this.showNotification = false
        this.notificationTimeout = null
      }, 2000)
    },

    async copyEmail() {
      await this.copyToClipboard(this.bureauEmail)
      this.showNotificationMessage('E-mail скопирован')
    },

    async copyPhone() {
      await this.copyToClipboard(this.bureauPhone)
      this.showNotificationMessage('Номер телефона скопирован')
    },
  },
}
</script>

<style scoped>
.recovery {
  padding: 36px 36px 32px;
  text-align: center;
}

.recovery-title {
  margin: 0 0 14px;
  font-size: 26px;
  font-weight: 800;
  color: var(--text);
}

.recovery-text {
  margin: 0 0 18px;
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
}

.recovery-contacts {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.recovery-contact {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 4px 10px;
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
  color: var(--accent-text);
}

.recovery-contact__text {
  font-size: 15px;
  color: var(--accent-text);
  font-weight: 500;
}

.recovery-contact:hover .recovery-contact__text {
  text-decoration: underline;
  text-underline-position: under;
}

.recovery-button {
  margin-top: 26px;
  background: var(--color-primary);
  color: var(--accent-contrast);
  border: none;
  border-radius: 40px;
  padding: 12px 40px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  min-width: 200px;
  transition: background-color 0.2s;
}

.recovery-button:hover {
  background-color: var(--accent);
}

/* Вид пилюли повторяет уведомление о копировании на самом экране входа
   (.notification в LoginComponent): белая плашка с мягкой тенью. */
/* top - фолбэк на случай, если окно не удалось замерить; штатно вертикаль
   приходит инлайном от pillStyle. */
.recovery-notification {
  position: fixed;
  top: 18vh;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  justify-content: center;
  height: 25px;
  padding: 0 15px;
  border-radius: 50px;
  background: var(--surface);
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.2);
  min-width: 150px;
  white-space: nowrap;
  z-index: 1100;
}

.recovery-notification-enter-active,
.recovery-notification-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.recovery-notification-enter-from,
.recovery-notification-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px);
}

.recovery-notification-enter-to,
.recovery-notification-leave-from {
  opacity: 1;
  transform: translate(-50%, 0);
}

@media (max-width: 768px) {
  .recovery {
    padding: 8px 24px 28px;
  }

  .recovery-title {
    font-size: 22px;
  }
}
</style>
