<template>
  <transition name="fade-scale">
    <button
      v-if="visible"
      class="scroll-top"
      @click="scrollToTop"
      aria-label="Прокрутить вверх"
    >
      <svg
        class="scroll-top__icon"
        viewBox="0 0 24 24"
        width="24"
        height="24"
      >
        <path
          d="M12 4l-8 8h5v8h6v-8h5l-8-8z"
          fill="currentColor"
        />
      </svg>
    </button>
  </transition>
</template>

<script>
export default {
  name: 'ScrollTopButton',
  data() {
    return {
      visible: false,
      scrollThreshold: 150,
    };
  },
  methods: {
    handleScroll() {
      this.visible = window.scrollY > this.scrollThreshold;
    },
    scrollToTop() {
      window.scrollTo({
        top: 0,
        behavior: 'smooth',
      });
    },
  },
  mounted() {
    window.addEventListener('scroll', this.handleScroll);
    this.handleScroll();
  },
  beforeUnmount() {
    window.removeEventListener('scroll', this.handleScroll);
  },
};
</script>

<style scoped>
.scroll-top {
  position: fixed;
  bottom: 30px;
  right: 30px;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: #ffffff;
  border: 1px solid #b0b0b0;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #333;
  transition: box-shadow 0.2s ease, transform 0.2s ease;
  z-index: 9999;
  outline: none;
  padding: 0;
}

/* Лёгкий эффект при клике — масштабирование */
.scroll-top:active {
  transform: scale(0.95);
}

.scroll-top__icon {
  width: 20px;
  height: 20px;
  transition: transform 0.2s ease;
}

/* Анимации появления/исчезновения */
.fade-scale-enter-active,
.fade-scale-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}
.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: scale(0.5);
}
.fade-scale-enter-to,
.fade-scale-leave-from {
  opacity: 1;
  transform: scale(1);
}

/* Адаптивность */
@media (max-width: 768px) {
  .scroll-top {
    bottom: 20px;
    right: 20px;
    width: 44px;
    height: 44px;
  }
  .scroll-top__icon {
    width: 18px;
    height: 18px;
  }
}
</style>