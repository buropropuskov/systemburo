<template>
  <transition name="scroll-btn">
    <button
      v-show="visible"
      class="scroll-top-btn"
      aria-label="Наверх"
      @click="scrollToTop"
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <polyline points="18 15 12 9 6 15" />
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
    }
  },

  mounted() {
    this._scrollHandler = this.handleScroll.bind(this)
    window.addEventListener('scroll', this._scrollHandler, { passive: true })
  },

  beforeUnmount() {
    window.removeEventListener('scroll', this._scrollHandler)
  },

  methods: {
    handleScroll() {
      this.visible = window.scrollY > 150
    },

    scrollToTop() {
      window.scrollTo({ top: 0, behavior: 'smooth' })
    },
  },
}
</script>

<style scoped>
.scroll-top-btn {
  position: fixed;
  /* env() учитывает home-indicator на iPhone - кнопка не перекрывается */
  bottom: calc(30px + env(safe-area-inset-bottom, 0px));
  right: calc(30px + env(safe-area-inset-right, 0px));
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--color-primary);
  color: #fff;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(79, 91, 223, 0.35);
  transition: background-color 0.2s, transform 0.2s, box-shadow 0.2s;
  z-index: 900;
}

.scroll-top-btn:hover {
  background-color: var(--color-primary-hover);
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(79, 91, 223, 0.4);
}

.scroll-top-btn:active {
  transform: translateY(0);
}

.scroll-btn-enter-active,
.scroll-btn-leave-active {
  transition: opacity 0.3s, transform 0.3s;
}

.scroll-btn-enter-from,
.scroll-btn-leave-to {
  opacity: 0;
  transform: scale(0.6);
}

@media (max-width: 480px) {
  .scroll-top-btn {
    bottom: calc(20px + env(safe-area-inset-bottom, 0px));
    right: calc(20px + env(safe-area-inset-right, 0px));
    width: 40px;
    height: 40px;
  }
}
</style>
