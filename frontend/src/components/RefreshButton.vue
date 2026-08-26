<template>
  <button
    class="refresh-btn"
    :class="{ 'refresh-btn--charging': cooldown }"
    :disabled="cooldown"
    type="button"
    @click="handleRefresh"
  >
    <!-- Перезарядка: три точки по центру, по очереди зажигаются в цвет кнопки (П.32) -->
    <span
      v-if="cooldown"
      class="refresh-btn__dots"
      aria-hidden="true"
    >
      <span
        class="refresh-btn__dot"
        :class="{ 'is-lit': chargeStep >= 1 }"
      />
      <span
        class="refresh-btn__dot"
        :class="{ 'is-lit': chargeStep >= 2 }"
      />
      <span
        class="refresh-btn__dot"
        :class="{ 'is-lit': chargeStep >= 3 }"
      />
    </span>
    <!-- Готова: иконка + текст, появляются с анимацией -->
    <span
      v-else
      class="refresh-btn__content"
    >
      <AppIcon
        name="refresh"
        class="refresh-btn__icon"
      />
      <span class="refresh-btn__text">Обновить</span>
    </span>
  </button>
</template>
<script>
import AppIcon from '@/components/icons/AppIcon.vue';

const CHARGE_STEP_MS = 400;
const CHARGE_MS = CHARGE_STEP_MS * 3;

export default {
  components: { AppIcon },
  props: {
    loading: { type: Boolean, default: false },
  },
  emits: ['refresh'],
  data() {
    return { cooldown: false, chargeStep: 0, timers: [] };
  },
  beforeUnmount() {
    this.clearTimers();
  },
  methods: {
    handleRefresh() {
      if (this.loading || this.cooldown) return;
      this.$emit('refresh');
      this.startCharge();
    },
    startCharge() {
      this.clearTimers();
      this.cooldown = true;
      this.chargeStep = 0;
      // зажигаем точки по очереди в цвет кнопки
      this.timers.push(setTimeout(() => { this.chargeStep = 1; }, CHARGE_STEP_MS));
      this.timers.push(setTimeout(() => { this.chargeStep = 2; }, CHARGE_STEP_MS * 2));
      this.timers.push(setTimeout(() => {
        this.chargeStep = 3;
        this.cooldown = false;
        this.chargeStep = 0;
      }, CHARGE_MS));
    },
    clearTimers() {
      this.timers.forEach(clearTimeout);
      this.timers = [];
    },
  },
}
</script>
<style scoped>
    .refresh-btn {
        width: 100px;
        height: 25px;
        border-radius: 50px;
        border: 1px solid var(--border);
        outline: none;
        background-color: var(--surface);
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 5px;
        cursor: pointer;
        transition: .2s;
    }

    .refresh-btn:hover {
        background-color: var(--surface-2);
    }

    /* Во время перезарядки: без курсора-лоадера, фон не меняется */
    .refresh-btn--charging {
        cursor: default;
    }

    .refresh-btn--charging:hover {
        background-color: var(--surface);
    }

    .refresh-btn__content {
        display: flex;
        align-items: center;
        gap: 5px;
        animation: refresh-btn-in 0.3s ease;
    }

    @keyframes refresh-btn-in {
        from { opacity: 0; transform: scale(0.85); }
        to { opacity: 1; transform: scale(1); }
    }

    .refresh-btn__icon {
        width: 15px;
        height: 15px;
        /* Значок обновления был фирменного синего - в тон подписи кнопки. */
        color: var(--accent-text);
        stroke-width: 2.2;
    }

    .refresh-btn__text {
        color: var(--accent-text);
        font-size: 12px;
        font-weight: 500;
    }

    .refresh-btn__dots {
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .refresh-btn__dot {
        width: 5px;
        height: 5px;
        border-radius: 50%;
        background-color: var(--accent-tint);
        transition: background-color 0.2s ease;
    }

    .refresh-btn__dot.is-lit {
        background-color: var(--accent);
    }

    /* Мобилка: «Обновить» сворачивается в круглую иконку. Текст прячем в clip (не
       display:none) - у кнопки остаётся доступное имя для скринридера. Размер 36px -
       как у прочих мобильных icon-кнопок проекта (rt-btn-compact / rt-header-inline),
       иконка 16px. Порог 767.98px - как во
       всём проекте: per-page оверрайды (pill в шапке Центра) и rt-header-inline
       завязаны на него, ровно на 768px иначе разъехались бы стили. */
    @media (max-width: 767.98px) {
        .refresh-btn {
            width: 36px;
            height: 36px;
            padding: 0;
            gap: 0;
        }

        .refresh-btn__text {
            position: absolute;
            width: 1px;
            height: 1px;
            padding: 0;
            margin: -1px;
            overflow: hidden;
            clip: rect(0, 0, 0, 0);
            white-space: nowrap;
            border: 0;
        }

        .refresh-btn__icon {
            width: 16px;
            height: 16px;
            stroke-width: 2.1;
        }
    }
</style>
