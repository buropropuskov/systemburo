<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      @click.self="close"
    >
      <div class="guide-modal">
        <header class="guide-modal__header">
          <h2
            :id="titleId"
            class="guide-modal__title"
          >
            {{ title }}
          </h2>
          <button
            type="button"
            class="guide-modal__close"
            aria-label="Закрыть"
            @click="close"
          >
            ×
          </button>
        </header>

        <div class="guide-modal__body">
          <nav
            class="guide-modal__sidebar"
            aria-label="Разделы руководства"
          >
            <ul>
              <li
                v-for="section in sections"
                :key="section.id"
              >
                <button
                  type="button"
                  class="guide-modal__nav-link"
                  :class="{ active: active === section.id }"
                  @click="active = section.id"
                >
                  <span class="guide-modal__nav-step">{{ section.step }}</span>
                  <span>{{ section.title }}</span>
                </button>
              </li>
            </ul>
          </nav>

          <article class="guide-modal__content">
            <h3 class="guide-modal__section-title">
              {{ currentSection.title }}
            </h3>
            <div class="guide-modal__section-body">
              <p
                v-for="(p, i) in currentSection.paragraphs"
                :key="i"
              >
                {{ p }}
              </p>
              <ol
                v-if="currentSection.steps && currentSection.steps.length"
                class="guide-modal__steps"
              >
                <li
                  v-for="(step, i) in currentSection.steps"
                  :key="i"
                >
                  {{ step }}
                </li>
              </ol>
              <div
                v-if="currentSection.tip"
                class="guide-modal__tip"
              >
                <strong>Совет.</strong> {{ currentSection.tip }}
              </div>
            </div>
          </article>
        </div>

        <footer class="guide-modal__footer">
          <button
            type="button"
            class="guide-modal__btn guide-modal__btn--ghost"
            :disabled="activeIndex === 0"
            @click="prev"
          >
            Назад
          </button>
          <span class="guide-modal__progress">
            {{ activeIndex + 1 }} / {{ sections.length }}
          </span>
          <button
            type="button"
            class="guide-modal__btn"
            @click="nextOrClose"
          >
            {{ activeIndex === sections.length - 1 ? 'Готово' : 'Далее' }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script>
let uid = 0;

export default {
  name: 'UserGuideModal',
  props: {
    title: { type: String, default: 'Руководство пользования системой' },
    sections: {
      type: Array,
      required: true,
      validator: arr => Array.isArray(arr) && arr.length > 0
    }
  },
  emits: ['close'],
  data() {
    uid++;
    return {
      active: this.sections[0]?.id || '',
      titleId: `guide-modal-title-${uid}`
    };
  },
  computed: {
    activeIndex() {
      return this.sections.findIndex(s => s.id === this.active);
    },
    currentSection() {
      return this.sections[this.activeIndex] || this.sections[0];
    }
  },
  mounted() {
    document.addEventListener('keydown', this.onKey);
    document.body.style.overflow = 'hidden';
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKey);
    document.body.style.overflow = '';
  },
  methods: {
    onKey(e) {
      if (e.key === 'Escape') this.close();
      if (e.key === 'ArrowRight') this.nextOrClose();
      if (e.key === 'ArrowLeft') this.prev();
    },
    prev() {
      if (this.activeIndex > 0) this.active = this.sections[this.activeIndex - 1].id;
    },
    nextOrClose() {
      if (this.activeIndex < this.sections.length - 1) {
        this.active = this.sections[this.activeIndex + 1].id;
      } else {
        this.close();
      }
    },
    close() {
      this.$emit('close');
    }
  }
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(0.1px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 16px;
}

.guide-modal {
  width: min(900px, 100%);
  max-height: 90vh;
  background: #fff;
  border-radius: 18px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
}

.guide-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 22px;
  border-bottom: 1px solid #eee;
}

.guide-modal__title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: #1a1a1a;
}

.guide-modal__close {
  background: transparent;
  border: none;
  font-size: 24px;
  line-height: 1;
  cursor: pointer;
  color: #6e7280;
  padding: 4px 10px;
  border-radius: 8px;
  transition: background 0.15s ease, color 0.15s ease;
}

.guide-modal__close:hover {
  background: #f0f0f5;
  color: #1a1a1a;
}

.guide-modal__body {
  display: grid;
  grid-template-columns: 240px 1fr;
  flex: 1 1 auto;
  min-height: 0;
}

.guide-modal__sidebar {
  border-right: 1px solid #eee;
  background: #fafafa;
  overflow-y: auto;
  padding: 14px 10px;
}

.guide-modal__sidebar ul {
  margin: 0;
  padding: 0;
  list-style: none;
}

.guide-modal__nav-link {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 10px;
  text-align: left;
  font: inherit;
  color: #4b5563;
  transition: background 0.15s ease, color 0.15s ease;
}

.guide-modal__nav-link:hover {
  background: rgba(79, 91, 223, 0.08);
  color: #4F5BDF;
}

.guide-modal__nav-link.active {
  background: #4F5BDF;
  color: #fff;
}

.guide-modal__nav-step {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 22px;
  padding: 0 6px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.06);
  font-size: 11px;
  font-weight: 700;
}

.guide-modal__nav-link.active .guide-modal__nav-step {
  background: rgba(255, 255, 255, 0.18);
}

.guide-modal__content {
  padding: 22px 26px;
  overflow-y: auto;
  color: #1a1a1a;
}

.guide-modal__section-title {
  margin: 0 0 14px;
  font-size: 18px;
  font-weight: 700;
}

.guide-modal__section-body {
  font-size: 14px;
  line-height: 1.6;
}

.guide-modal__section-body p {
  margin: 0 0 12px;
}

.guide-modal__steps {
  margin: 8px 0 16px;
  padding-left: 22px;
}

.guide-modal__steps li {
  margin-bottom: 6px;
}

.guide-modal__tip {
  margin-top: 14px;
  padding: 12px 14px;
  background: rgba(79, 91, 223, 0.08);
  border-left: 3px solid #4F5BDF;
  border-radius: 8px;
  font-size: 13px;
}

.guide-modal__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 22px;
  border-top: 1px solid #eee;
  background: #fafafa;
}

.guide-modal__progress {
  font-size: 12px;
  color: #6e7280;
}

.guide-modal__btn {
  padding: 8px 18px;
  background: #4F5BDF;
  color: #fff;
  border: 1px solid transparent;
  border-radius: 999px;
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.guide-modal__btn:hover:not(:disabled) {
  background: #3d49c7;
}

.guide-modal__btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.guide-modal__btn--ghost {
  background: #fff;
  color: #4F5BDF;
  border-color: #4F5BDF;
}

.guide-modal__btn--ghost:hover:not(:disabled) {
  background: #eef0ff;
}

@media (max-width: 720px) {
  .guide-modal__body {
    grid-template-columns: 1fr;
  }
  .guide-modal__sidebar {
    border-right: none;
    border-bottom: 1px solid #eee;
    max-height: 200px;
  }
}
</style>
