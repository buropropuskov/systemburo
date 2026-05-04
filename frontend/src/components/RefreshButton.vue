<template>
  <button
    class="refresh-btn"
    :class="{ 'refresh-btn--loading': loading }"
    :disabled="loading"
    type="button"
    @click="handleRefresh"
  >
    <img
      src="@/assets/icons/refresh.png"
      class="refresh-btn__icon"
      :class="{ 'refresh-btn__icon--spinning': loading }"
      alt=""
    >
    <p class="refresh-btn__text">
      Обновить
    </p>
  </button>
</template>
<script>
export default {
  props: {
    loading: { type: Boolean, default: false },
  },
  emits: ['refresh'],
  methods: {
    handleRefresh() {
      if (this.loading) return;
      this.$emit('refresh');
    }
  }
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

    .refresh-btn--loading {
        cursor: progress;
        opacity: 0.85;
    }

    .refresh-btn--loading:hover {
        background-color: #FFF;
    }

    .refresh-btn__icon {
        width: 15px;
        height: 15px;
    }

    .refresh-btn__icon--spinning {
        animation: refresh-btn-spin 0.9s linear infinite;
    }

    @keyframes refresh-btn-spin {
        from { transform: rotate(0deg); }
        to { transform: rotate(-360deg); }
    }

    .refresh-btn__text {
        color: #4F5BDF;
        font-size: 12px;
        font-weight: 500;
    }
</style>