<template>
  <Teleport to="body">
    <transition name="notif-detail-fade">
      <div
        v-if="show"
        class="notif-detail-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="notif-detail-dialog"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          role="dialog"
          aria-modal="true"
          @mousedown.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />

          <header class="notif-detail-dialog__header">
            <div class="notif-detail-dialog__heading">
              <Badge
                v-if="categoryBadge"
                :label="categoryBadge.label"
                :variant="categoryBadge.variant"
                size="sm"
              />
              <h3 class="notif-detail-dialog__title">
                {{ title }}
              </h3>
            </div>
            <button
              class="notif-detail-dialog__close"
              aria-label="Закрыть"
              @click="emit('close')"
            >
              &times;
            </button>
          </header>

          <div
            ref="bodyEl"
            class="notif-detail-dialog__body"
          >
            <p
              v-if="message"
              class="notif-detail-dialog__message"
            >
              {{ message }}
            </p>

            <div class="notif-detail-dialog__time">
              <span
                v-if="relativeTime"
                class="notif-detail-dialog__time-relative"
              >{{ relativeTime }}</span>
              <span
                v-if="exactTime"
                class="notif-detail-dialog__time-exact"
              >{{ exactTime }}</span>
            </div>

            <dl
              v-if="fields.length"
              class="notif-detail-dialog__fields"
            >
              <template
                v-for="f in fields"
                :key="f.label"
              >
                <dt>{{ f.label }}</dt>
                <dd>{{ f.value }}</dd>
              </template>
            </dl>
          </div>

          <footer class="notif-detail-dialog__footer">
            <button
              v-if="actionLabel"
              type="button"
              class="lk-button lk-button--primary"
              @click="emit('action')"
            >
              {{ actionLabel }}
            </button>
            <button
              type="button"
              class="lk-button lk-button--secondary"
              @click="emit('unread')"
            >
              Вернуть в непрочитанные
            </button>
            <button
              type="button"
              class="lk-button lk-button--danger"
              @click="emit('delete')"
            >
              Удалить
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
// Раскрытие подробностей уведомления (#1748 S6): текст в карточке UserNotifications/
// UserNotificationsInline обрезан line-clamp:2, а поля из data (номер заявки, кто
// передал, срок ожидания...) вообще не показывались. Компонент только представление -
// решения (перейти к заявке, вернуть в непрочитанные, удалить) принимает родитель.
import { computed, ref, watch, onBeforeUnmount } from 'vue';
import Badge from '@/components/ui/Badge.vue';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { formatDateTime, formatTimeAgo } from '@/utils/datetime';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useEscapeClose } from '@/composables/useEscapeClose';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { notificationDetailFields, notificationCategory, notificationActionLabel } from '@/utils/notificationDetails';

const props = defineProps({
  show: { type: Boolean, default: false },
  notification: { type: Object, default: null },
});
const emit = defineEmits(['close', 'action', 'unread', 'delete']);

const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
useEscapeClose(() => emit('close'), () => props.show);

// Bottom-sheet свайп-вниз-закрытие на мобилке - контракт окна проекта (#1097 R9).
const bodyEl = ref(null);
const swipe = useSwipeDismiss(() => emit('close'), {
  getScrollTop: () => bodyEl.value?.scrollTop ?? 0,
  handleSelector: '.sheet-handle',
});
const sheetOffset = swipe.offset;
const sheetDragging = swipe.isDragging;
const onSheetTouchStart = swipe.onTouchStart;
const onSheetTouchMove = swipe.onTouchMove;
const onSheetTouchEnd = swipe.onTouchEnd;

// Владелец блокировки скролла - токен инстанса, в script setup `this` недоступен.
const scrollLockOwner = {};
watch(() => props.show, (visible) => setBodyScrollLock(scrollLockOwner, visible));
onBeforeUnmount(() => releaseBodyScrollLock(scrollLockOwner));

const title = computed(() => props.notification?.title || 'Уведомление');
const message = computed(() => props.notification?.message || '');
const fields = computed(() => notificationDetailFields(props.notification));
const actionLabel = computed(() => notificationActionLabel(props.notification));
const relativeTime = computed(() => formatTimeAgo(props.notification?.created_at));
const exactTime = computed(() => formatDateTime(props.notification?.created_at));

const CATEGORY_BADGES = {
  application: { label: 'Заявка', variant: 'primary' },
  security: { label: 'Безопасность', variant: 'danger' },
  passage: { label: 'Проезд', variant: 'warning' },
  content: { label: 'Публикации', variant: 'success' },
  system: { label: 'Система', variant: 'neutral' },
};
const categoryBadge = computed(() => CATEGORY_BADGES[notificationCategory(props.notification?.type)] || null);
</script>

<style scoped>
.notif-detail-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  /* Выше панели уведомлений (десктоп-дропдаун 200, мобильный sheet 10000/10001),
     ниже DirtyConfirmModal(11000)/ConfirmDialog(20000)/стека тостов(29000). */
  z-index: 10500;
  padding: 20px;
}

.notif-detail-dialog {
  width: 100%;
  max-width: 480px;
  max-height: 80vh;
  background: var(--surface);
  border-radius: 30px;
  box-shadow: 0 20px 60px var(--shadow-drop);
  display: flex;
  flex-direction: column;
  font-family: 'Montserrat', sans-serif;
}

.sheet-handle {
  display: none;
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 2px;
  flex-shrink: 0;
}

.notif-detail-dialog__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.notif-detail-dialog__heading {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.notif-detail-dialog__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  word-break: break-word;
}

.notif-detail-dialog__close {
  flex-shrink: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  line-height: 1;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.notif-detail-dialog__close:hover {
  color: var(--text);
  background: var(--surface-2);
}

.notif-detail-dialog__body {
  padding: 16px 24px 20px;
  overflow-y: auto;
  flex: 1;
}

.notif-detail-dialog__message {
  margin: 0 0 14px;
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.notif-detail-dialog__time {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 14px;
  font-size: 12px;
  color: var(--text-muted);
}

.notif-detail-dialog__time-exact::before {
  content: '\00b7';
  margin-right: 8px;
}

.notif-detail-dialog__fields {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 16px;
  margin: 0;
  padding: 14px 16px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 20px;
}

.notif-detail-dialog__fields dt {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 400;
  white-space: nowrap;
}

.notif-detail-dialog__fields dd {
  margin: 0;
  font-size: 13px;
  color: var(--text);
  font-weight: 500;
  word-break: break-word;
}

.notif-detail-dialog__footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.notif-detail-fade-enter-active,
.notif-detail-fade-leave-active {
  transition: opacity 0.2s ease;
}

.notif-detail-fade-enter-from,
.notif-detail-fade-leave-to {
  opacity: 0;
}

.notif-detail-fade-enter-active .notif-detail-dialog,
.notif-detail-fade-leave-active .notif-detail-dialog {
  transition: transform 0.2s ease-out, opacity 0.2s ease-out;
}

.notif-detail-fade-enter-from .notif-detail-dialog,
.notif-detail-fade-leave-to .notif-detail-dialog {
  transform: translateY(20px);
  opacity: 0;
}

@media (max-width: 768px) {
  .notif-detail-overlay {
    padding: 0;
    align-items: flex-end;
  }

  .notif-detail-dialog {
    max-width: 100%;
    max-height: 90dvh;
    border-radius: 16px 16px 0 0;
    transition: transform 0.3s ease;
  }

  .notif-detail-dialog.is-dragging {
    transition: none;
  }

  .sheet-handle {
    display: block;
  }

  .notif-detail-fade-enter-from .notif-detail-dialog,
  .notif-detail-fade-leave-to .notif-detail-dialog {
    transform: translateY(100%);
  }

  .notif-detail-dialog__footer {
    flex-direction: column;
    align-items: stretch;
  }

  .notif-detail-dialog__footer .lk-button {
    width: 100%;
  }
}
</style>
