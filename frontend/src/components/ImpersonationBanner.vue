<template>
  <Teleport to="body">
    <transition name="impersonation-bar">
      <div
        v-if="active"
        class="impersonation-bar"
        role="status"
        aria-live="polite"
        data-testid="impersonation-bar"
      >
        <span
          class="impersonation-bar__icon"
          aria-hidden="true"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.9"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
            <circle
              cx="12"
              cy="7"
              r="4"
            />
          </svg>
        </span>
        <p class="impersonation-bar__text">
          Вы работаете от имени
          <strong data-testid="impersonation-bar-name">{{ name }}</strong>
          <span
            v-if="remainingText"
            class="impersonation-bar__timer"
            data-testid="impersonation-bar-timer"
          >{{ remainingText }}</span>
        </p>
        <button
          type="button"
          class="lk-button lk-button--secondary impersonation-bar__back"
          data-testid="impersonation-bar-back"
          :disabled="returning"
          @click="back"
        >
          {{ returning ? 'Возвращаемся…' : 'Вернуться в свою учётную запись' }}
        </button>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { useAuthStore } from '@/stores/auth';
import { useDeletionsStore } from '@/stores/deletions';

const MINUTE_MS = 60 * 1000;

/**
 * Постоянная полоса режима «войти как пользователь» (#1912). Висит поверх любого
 * экрана, пока сеанс от чужого имени открыт: администратор, забывший, от чьего
 * имени действует, - худший исход из возможных, хуже самого режима.
 */
export default {
  name: 'ImpersonationBanner',
  data() {
    return {
      returning: false,
      // Пересчитывается по таймеру: без него остаток застывает на значении,
      // посчитанном при открытии режима.
      now: Date.now(),
      tick: null,
    };
  },
  computed: {
    active() {
      return useAuthStore().isImpersonating;
    },
    name() {
      return useAuthStore().impersonatedName;
    },
    /**
     * Остаток срока сеанса словами. Пустая строка, если срок неизвестен или уже
     * вышел: обещать «осталось 0 минут» смысла нет, истечение и так закроет режим.
     */
    remainingText() {
      const expiresAt = useAuthStore().impersonation?.expiresAt;
      if (!expiresAt) return '';
      const left = new Date(expiresAt).getTime() - this.now;
      if (!Number.isFinite(left) || left <= 0) return '';
      return `осталось ${Math.max(1, Math.ceil(left / MINUTE_MS))} мин`;
    },
  },
  mounted() {
    this.tick = setInterval(() => { this.now = Date.now(); }, 30 * 1000);
  },
  beforeUnmount() {
    clearInterval(this.tick);
  },
  methods: {
    async back() {
      if (this.returning) return;
      this.returning = true;
      const name = this.name;
      try {
        const restored = await useAuthStore().endImpersonation();
        if (restored) {
          useDeletionsStore().notify({ prefix: `Работа от имени ${name} завершена` });
          // Возврат меняет набор прав: раздел, открытый глазами работника, может
          // быть недоступен уже не ему, а самому администратору - и наоборот.
          this.$router.push('/news').catch(() => {});
        } else {
          useDeletionsStore().notify({
            prefix: 'Не удалось вернуться в свою учётную запись, войдите заново',
            type: 'error',
          });
          this.$router.push('/').catch(() => {});
        }
      } finally {
        this.returning = false;
      }
    },
  },
};
</script>

<style scoped>
.impersonation-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  /* Выше плашки блокировки (26000) и окна согласия (25500), ниже тостов (29000).
     Кнопка возврата - единственный выход из чужой учётной записи, и перекрыть её
     не должно ничто: и блокировка, и требование согласия могут догнать сеанс уже
     внутри режима, а тогда администратор окажется заперт в чужой учётке. */
  z-index: 26500;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  /* Пара warning-bg/warning-text перевёрнута в тёмной теме сама - у полосы, которая
     висит на каждом экране, контраст не может зависеть от выбранной палитры. */
  background: var(--warning-bg);
  color: var(--warning-text);
  border-top: 2px solid var(--warning);
  box-shadow: 0 -4px 16px rgb(0 0 0 / 18%);
}

.impersonation-bar__icon {
  display: flex;
  flex-shrink: 0;
}

.impersonation-bar__text {
  flex: 1 1 auto;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  line-height: 1.3;
}

.impersonation-bar__timer {
  margin-left: 8px;
  opacity: 0.85;
  white-space: nowrap;
}

.impersonation-bar__back {
  flex-shrink: 0;
}

/* Анимация только по transform и opacity - полоса появляется на любом экране,
   в том числе поверх длинных списков. */
.impersonation-bar-enter-active,
.impersonation-bar-leave-active {
  transition: transform 200ms ease-out, opacity 200ms ease-out;
}

.impersonation-bar-enter-from,
.impersonation-bar-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

@media (width <= 768px) {
  .impersonation-bar {
    flex-wrap: wrap;
    padding: 10px 14px;
  }

  .impersonation-bar__back {
    width: 100%;
  }
}
</style>
