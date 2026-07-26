<template>
  <teleport to="body">
    <transition name="ou-modal">
      <div
        v-if="show"
        class="ou-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          ref="dialogRef"
          class="ou-modal"
          role="dialog"
          aria-modal="true"
          aria-label="Пользователи онлайн"
          tabindex="-1"
          @mousedown.stop
        >
          <div class="ou-modal__header">
            <h3 class="ou-modal__title">
              Пользователи онлайн<span
                v-if="!loading && !error"
                class="ou-modal__count"
              > · {{ users.length }}</span>
            </h3>
            <button
              class="ou-modal__close"
              type="button"
              aria-label="Закрыть"
              @click="emit('close')"
            >
              &times;
            </button>
          </div>

          <div class="ou-modal__body">
            <div
              v-if="loading"
              class="ou-modal__state"
            >
              <LoaderSpinner label="Загрузка списка…" />
            </div>
            <p
              v-else-if="error"
              class="ou-modal__state ou-modal__state--error"
            >
              {{ error }}
            </p>
            <p
              v-else-if="!users.length"
              class="ou-modal__state"
            >
              Сейчас никого онлайн
            </p>
            <ul
              v-else
              class="ou-list"
            >
              <li
                v-for="u in users"
                :key="u.id"
                class="ou-row"
              >
                <span
                  class="ou-row__dot"
                  aria-hidden="true"
                />
                <div class="ou-row__main">
                  <div class="ou-row__name">
                    {{ u.full_name || u.login }}
                  </div>
                  <div class="ou-row__meta">
                    <span
                      v-if="roleType(u)"
                      class="ou-row__role"
                    >{{ roleType(u) }}</span>
                    <span class="ou-row__login">@{{ u.login }}</span>
                  </div>
                </div>
                <span class="ou-row__seen">{{ formatTimeAgo(u.last_seen) }}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { formatTimeAgo } from '@/utils/datetime.js';

const props = defineProps({
  show: { type: Boolean, required: true },
  users: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
});
const emit = defineEmits(['close']);

const dialogRef = ref(null);
const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));

/** Подпись разреза: бизнес-роль приоритетнее, иначе тип пользователя. */
function roleType(u) {
  return u.role || u.user_type || '';
}

function onKeydown(e) {
  if (props.show && e.key === 'Escape') emit('close');
}

// Владелец блокировки - токен инстанса: в script setup `this` недоступен.
const scrollLockOwner = {};

// Блокируем прокрутку фона и переводим фокус в модалку при открытии (a11y).
watch(
  () => props.show,
  (val) => {
    setBodyScrollLock(scrollLockOwner, val);
    if (val) nextTick(() => dialogRef.value?.focus());
  }
);

onMounted(() => document.addEventListener('keydown', onKeydown));
onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown);
  releaseBodyScrollLock(scrollLockOwner);
});
</script>

<style scoped>
.ou-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 12000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.ou-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 460px;
  max-width: 100%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
  overflow: hidden;
}

/* .ou-modal — программная цель фокуса при открытии (tabindex=-1), рамку не показываем. */
.ou-modal:focus {
  outline: none;
}

.ou-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
  border-bottom: 1px solid var(--color-border);
}

.ou-modal__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.ou-modal__count {
  color: var(--color-text-muted);
  font-weight: 500;
}

.ou-modal__close {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--color-text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  flex-shrink: 0;
  transition: color 0.15s, background 0.15s;
}

.ou-modal__close:hover {
  color: var(--color-text);
  background: var(--color-bg-secondary);
}

.ou-modal__body {
  padding: 12px 16px;
  overflow-y: auto;
}

.ou-modal__state {
  margin: 0;
  padding: 32px 8px;
  text-align: center;
  color: var(--color-text-muted);
}

.ou-modal__state--error {
  color: var(--danger-text);
}

.ou-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.ou-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
}

.ou-row + .ou-row {
  margin-top: 2px;
}

.ou-row:hover {
  background: var(--color-bg-secondary);
}

.ou-row__dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--color-success);
  flex-shrink: 0;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--success) 22%, transparent);
}

.ou-row__main {
  flex: 1 1 auto;
  min-width: 0;
}

.ou-row__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ou-row__meta {
  display: flex;
  gap: 8px;
  align-items: baseline;
  margin-top: 2px;
  font-size: 12px;
  min-width: 0;
}

.ou-row__role {
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ou-row__login {
  color: var(--accent-text);
  flex-shrink: 0;
}

.ou-row__seen {
  font-size: 12px;
  color: var(--color-text-muted);
  flex-shrink: 0;
  white-space: nowrap;
}

/* Анимация открытия/закрытия: только opacity + transform. */
.ou-modal-enter-active {
  transition: opacity 0.25s ease;
}
.ou-modal-leave-active {
  transition: opacity 0.2s ease;
}
.ou-modal-enter-from,
.ou-modal-leave-to {
  opacity: 0;
}

.ou-modal-enter-active .ou-modal {
  animation: ou-scale-in 0.25s ease;
}
.ou-modal-leave-active .ou-modal {
  animation: ou-scale-out 0.2s ease;
}

@keyframes ou-scale-in {
  from {
    transform: scale(0.96);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes ou-scale-out {
  from {
    transform: scale(1);
    opacity: 1;
  }
  to {
    transform: scale(0.96);
    opacity: 0;
  }
}

@media (max-width: 768px) {
  .ou-overlay {
    align-items: flex-end;
    padding: 0;
  }
  .ou-modal {
    width: 100%;
    max-width: 100%;
    max-height: 88vh;
    border-radius: 16px 16px 0 0;
  }
  .ou-modal__close {
    width: 40px;
    height: 40px;
  }
}
</style>
