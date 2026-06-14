<template>
  <button
    class="refresh-btn"
    :class="{ 'refresh-btn--charging': cooldown }"
    :disabled="cooldown"
    type="button"
    @click="handleRefresh"
  >
    <!-- Перезарядка: три точки по центру, каждую секунду зажигается одна (П.32) -->
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
      <img
        src="@/assets/icons/refresh.png"
        class="refresh-btn__icon"
        alt=""
      >
      <span class="refresh-btn__text">Обновить</span>
    </span>
  </button>
</template>
<script>
const CHARGE_MS = 3000;

export default {
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
      // каждую секунду зажигаем одну точку чёрным
      this.timers.push(setTimeout(() => { this.chargeStep = 1; }, 1000));
      this.timers.push(setTimeout(() => { this.chargeStep = 2; }, 2000));
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
        border: 1px solid #e6e6e6;
        outline: none;
        background-color: #FFF;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 5px;
        cursor: pointer;
        transition: .2s;
    }

    .refresh-btn:hover {
        background-color: #f2f2f2;
    }

    /* Во время перезарядки: без курсора-лоадера, фон не меняется */
    .refresh-btn--charging {
        cursor: default;
    }

    .refresh-btn--charging:hover {
        background-color: #FFF;
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
    }

    .refresh-btn__text {
        color: #4F5BDF;
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
        background-color: #ccc;
        transition: background-color 0.3s ease;
    }

    .refresh-btn__dot.is-lit {
        background-color: #333;
    }
</style>
