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
                  {{ item }}
                </li>
              </ul>
            </div>
          </div>
          <div class="dirty-modal__actions">
            <button
              class="dirty-modal__btn dirty-modal__btn--cancel"
              @click="onCancel"
            >
              Остаться
            </button>
            <button
              class="dirty-modal__btn dirty-modal__btn--confirm"
              @click="onConfirm"
            >
              Уйти без сохранения
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { confirmState, resolveDirtyConfirm } from '@/utils/dirtyTracker';

export default {
  name: 'DirtyConfirmModal',
  data() {
    return { confirmState };
  },
  methods: {
    onConfirm() {
      resolveDirtyConfirm(true);
    },
    onCancel() {
      resolveDirtyConfirm(false);
    },
  },
};
</script>

<style scoped>
.dirty-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 11000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.dirty-modal {
  background: #fff;
  border-radius: 16px;
  padding: 22px 24px 18px;
  width: 560px;
  max-width: calc(100vw - 32px);
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.18);
}

.dirty-modal__header {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 10px;
}

.dirty-modal__content {
  margin-bottom: 20px;
}

.dirty-modal__message {
  margin: 0 0 12px;
  font-size: 14px;
  color: #4b5563;
  line-height: 1.45;
}

.dirty-modal__changes {
  background: #f8f9ff;
  border: 1px solid #eef0ff;
  border-radius: 12px;
  padding: 10px 14px;
  max-height: 240px;
  overflow-y: auto;
}

.dirty-modal__changes-title {
  font-size: 12px;
  font-weight: 600;
  color: #4F5BDF;
  margin-bottom: 6px;
}

.dirty-modal__changes-list {
  margin: 0;
  padding: 0 0 0 18px;
  list-style: disc;
}

.dirty-modal__changes-item {
  font-size: 13px;
  color: #4b5563;
  line-height: 1.45;
  padding: 1px 0;
}

.dirty-modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.dirty-modal__btn {
  padding: 9px 18px;
  border: 1px solid #e6e6e6;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.dirty-modal__btn--cancel {
  background: #fff;
  color: #4b5563;
}

.dirty-modal__btn--cancel:hover {
  background: #f5f5f5;
  border-color: #d4d4d4;
}

.dirty-modal__btn--confirm {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.dirty-modal__btn--confirm:hover {
  background: #3a45c4;
  border-color: #3a45c4;
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
