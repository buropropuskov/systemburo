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
    const ua = typeof navigator !== 'undefined' ? navigator.userAgent : ''
    return {
      visible: false,
      // МОБИЛЬНЫЙ Яндекс-браузер сам рисует стрелку-вверх при скролле - прячем нашу, чтобы не
      // дублировать. Только мобильный: десктопный Яндекс стрелку не рисует, а его UA тоже содержит
      // YaBrowser - поэтому нужен ещё токен Mobile (десктоп его не имеет). У Chrome/Safari/Firefox
      // встроенной стрелки нет - там кнопку показываем.
      isMobileYandex: /YaBrowser/i.test(ua) && /Mobile/i.test(ua),
    }
  },

  mounted() {
    if (this.isMobileYandex) return
    this._scrollHandler = this.handleScroll.bind(this)
    window.addEventListener('scroll', this._scrollHandler, { passive: true })
  },

  beforeUnmount() {
    if (this._scrollHandler) window.removeEventListener('scroll', this._scrollHandler)
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
  background-color: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 50%;
  cursor: pointer;
  box-shadow: 0 4px 12px color-mix(in srgb, var(--accent) 35%, transparent);
  transition: background-color 0.2s, transform 0.2s, box-shadow 0.2s;
  z-index: 900;
}

.scroll-top-btn:hover {
  background-color: var(--color-primary-hover);
  transform: translateY(-2px);
  box-shadow: 0 6px 16px color-mix(in srgb, var(--accent) 40%, transparent);
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

/* Страница с закреплённой панелью действий у нижнего края (списки сотрудников и машин
   на мобилке, атрибут data-bottom-action-bar) поднимает кнопку над панелью: она fixed и
   лежит в том же углу, поэтому иначе круг ложится поверх кнопки «Добавить» и перехватывает
   тап. 76px = высота панели (8 + 44 + 12) плюс 12px зазора; safe-area учтена в самой
   панели, здесь прибавляется тем же env(). Селектор специфичнее правила для <=480
   ((0,3,1) против (0,2,0)), поэтому перекрывает его независимо от порядка. */
body:has([data-bottom-action-bar]) .scroll-top-btn {
  bottom: calc(76px + env(safe-area-inset-bottom, 0px));
}
</style>
