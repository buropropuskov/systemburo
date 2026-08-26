<template>
  <Teleport to="body">
    <transition name="dirty-fade">
      <div
        v-if="confirmState.show"
        class="dirty-overlay"
        @click.self="onCancel"
      >
        <div class="dirty-modal">
          <div class="dirty-modal__header">
            Несохранённые изменения
          </div>
          <div class="dirty-modal__content">
            <p class="dirty-modal__message">
              {{ confirmState.message }}
            </p>
            <div
              v-if="confirmState.changes && confirmState.changes.length"
              class="dirty-modal__changes"
            >
              <div class="dirty-modal__changes-title">
                Изменения, которые будут потеряны:
              </div>
              <ul class="dirty-modal__changes-list">
                <li
                  v-for="(item, i) in confirmState.changes"
                  :key="i"
                  class="dirty-modal__changes-item"
                >
                  <template v-if="typeof item === 'string'">{{ item }}</template>
                  <template v-else>
                    <span class="dirty-modal__changes-label">{{ item.label }}</span>
                    <template v-if="item.from || item.to">
                      <span class="dirty-modal__changes-value">{{ item.from }}</span>
                      <svg
                        class="dirty-modal__changes-arrow"
                        viewBox="0 0 14 10"
                        aria-hidden="true"
                      >
                        <path
                          d="M0.5 5h11.5M8.5 1.5L12.5 5l-4 3.5"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="1.4"
                          stroke-linecap="round"
                          stroke-linejoin="round"
                        />
                      </svg>
                      <span class="dirty-modal__changes-value">{{ item.to }}</span>
                    </template>
                  </template>
                </li>
              </ul>
            </div>
          </div>
          <div class="dirty-modal__actions">
            <button
              class="dirty-modal__btn dirty-modal__btn--cancel"
              :disabled="savingAll"
              @click="onCancel"
            >
              Остаться
            </button>
            <button
              class="dirty-modal__btn dirty-modal__btn--confirm"
              :disabled="savingAll"
              @click="onConfirm"
            >
              Уйти без сохранения
            </button>
            <button
              class="dirty-modal__btn dirty-modal__btn--save"
              :disabled="savingAll"
              @click="onSaveAll"
            >
              {{ savingAll ? 'Сохраняем...' : 'Сохранить все изменения' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { confirmState, resolveDirtyConfirm, saveAllDirty } from '@/utils/dirtyTracker';

export default {
  name: 'DirtyConfirmModal',
  data() {
    return { confirmState, savingAll: false };
  },
  methods: {
    onConfirm() {
      resolveDirtyConfirm(true);
    },
    onCancel() {
      resolveDirtyConfirm(false);
    },
    async onSaveAll() {
      if (this.savingAll) return;
      this.savingAll = true;
      try {
        const ok = await saveAllDirty();
        if (ok) resolveDirtyConfirm(true);
      } finally {
        this.savingAll = false;
      }
    },
  },
  mounted() {
    this.escHandler = (e) => {
      if (e.key === 'Escape' && this.confirmState.show) this.onCancel();
    };
    document.addEventListener('keydown', this.escHandler);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.escHandler);
  },
};
</script>

<style scoped>
.dirty-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  /* Глобальный диалог из App.vue: его поднимает router guard из ЛЮБОГО контекста,
     в том числе когда поверх страницы открыта модалка (истории 12000-13000,
     настройка полей вложения 12000). На 11000 вопрос о несохранённых изменениях
     уходил под её оверлей: навигация ждала ответа, кнопки были неклики, и «Назад»
     выглядел сломанным. Выше обычных модалок, но ниже SessionExpiredModal (25000)
     и BanOverlay (26000) - те обязаны перебивать всё. */
  z-index: 21000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.dirty-modal {
  background: var(--surface);
  border-radius: 16px;
  padding: 22px 24px 18px;
  width: 560px;
  max-width: calc(100vw - 32px);
  box-shadow: 0 10px 28px var(--shadow-drop);
}

.dirty-modal__header {
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 10px;
}

.dirty-modal__content {
  margin-bottom: 20px;
}

.dirty-modal__message {
  margin: 0 0 12px;
  font-size: 14px;
  color: var(--text-muted);
  line-height: 1.45;
}

.dirty-modal__changes {
  background: var(--accent-tint);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px 14px;
  max-height: 240px;
  overflow-y: auto;
}

.dirty-modal__changes-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-text);
  margin-bottom: 6px;
}

.dirty-modal__changes-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.dirty-modal__changes-item {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.45;
  padding: 2px 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.dirty-modal__changes-item::before {
  content: '';
  display: inline-block;
  width: 4px;
  height: 4px;
  background: var(--accent);
  border-radius: 50%;
  flex-shrink: 0;
  margin-right: 2px;
}

.dirty-modal__changes-label {
  color: var(--text-muted);
}

.dirty-modal__changes-value {
  font-weight: 600;
  color: var(--text);
}

.dirty-modal__changes-arrow {
  width: 14px;
  height: 10px;
  color: var(--text);
  flex-shrink: 0;
}

.dirty-modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.dirty-modal__btn {
  padding: 9px 18px;
  border: 1px solid var(--border);
  border-radius: 10px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.dirty-modal__btn--cancel {
  background: var(--surface);
  color: var(--text-muted);
}

.dirty-modal__btn--cancel:hover {
  background: var(--surface-2);
  border-color: var(--border);
}

.dirty-modal__btn--confirm {
  background: var(--surface);
  color: var(--danger-text);
  border-color: var(--border);
}

.dirty-modal__btn--confirm:hover {
  background: var(--danger-bg);
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.dirty-modal__btn--save {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.dirty-modal__btn--save:hover:not(:disabled) {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.dirty-modal__btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.dirty-fade-enter-active,
.dirty-fade-leave-active {
  transition: opacity 0.25s ease;
}

.dirty-fade-enter-active .dirty-modal,
.dirty-fade-leave-active .dirty-modal {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.dirty-fade-enter-from,
.dirty-fade-leave-to {
  opacity: 0;
}

.dirty-fade-enter-from .dirty-modal,
.dirty-fade-leave-to .dirty-modal {
  opacity: 0;
  transform: scale(0.95) translateY(-12px);
}
</style>
