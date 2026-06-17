<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="message-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>Сообщение к заявке {{ applicationNumber }}</h3>
            <button
              class="modal-close"
              @click="requestClose"
            >
              ×
            </button>
          </div>
          <div class="modal-content">
            <div
              class="text-constructor-content"
              v-html="sanitizedMessage"
            />
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { sanitizeHtml } from '@/utils/sanitize';
import { useOverlayClose } from '@/composables/useOverlayClose';

const props = defineProps({
  show: { type: Boolean, default: false },
  message: { type: String, default: '' },
  applicationNumber: { type: [String, Number], default: '' },
});

const emit = defineEmits(['close']);

const visible = ref(false);
const requestClose = () => { visible.value = false; };
const onAfterLeave = () => { emit('close'); };

const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);

const sanitizedMessage = computed(() => sanitizeHtml(props.message));

const onKeydown = (e) => {
  if (e.key === 'Escape' && visible.value) requestClose();
};

watch(() => props.show, (val) => { visible.value = val; }, { immediate: true });
watch(visible, (val) => { document.body.style.overflow = val ? 'hidden' : ''; });

onMounted(() => document.addEventListener('keydown', onKeydown));
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown);
  document.body.style.overflow = '';
});
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  z-index: 12000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.modal-fade-enter-active {
  transition: opacity 0.25s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .message-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .message-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes modal-scale-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

.message-modal {
  background: #fff;
  border-radius: 30px;
  width: 760px;
  max-width: 95%;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 25px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #000;
}

.modal-close {
  background: none;
  border: none;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  color: #999;
  padding: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s ease;
}

.modal-close:hover {
  color: #000;
}

.modal-content {
  padding: 25px;
  overflow-y: auto;
  flex: 1;
}

/* Безопасный рендер форматированного сообщения (render-safety) */
.text-constructor-content {
  font-size: 15px;
  line-height: 1.5;
  color: #000;
  word-break: break-word;
}

.text-constructor-content :deep(img) {
  max-width: 100%;
  height: auto;
}

.text-constructor-content :deep(img:not([height])) {
  height: auto;
}

.text-constructor-content :deep(strong) { font-weight: 700; }
.text-constructor-content :deep(em) { font-style: italic; }
.text-constructor-content :deep(u) { text-decoration: underline; }

.text-constructor-content :deep(ul),
.text-constructor-content :deep(ol) {
  padding-left: 22px;
  margin: 8px 0;
}

.text-constructor-content :deep(li) { margin: 2px 0; }

.text-constructor-content :deep(h1),
.text-constructor-content :deep(.heading-h1) {
  font-size: 24px;
  font-weight: 700;
  margin: 12px 0 8px;
}

.text-constructor-content :deep(h2),
.text-constructor-content :deep(.heading-h2) {
  font-size: 20px;
  font-weight: 600;
  margin: 10px 0 6px;
}

.text-constructor-content :deep(h3) {
  font-size: 17px;
  font-weight: 600;
  margin: 8px 0 6px;
}

.text-constructor-content :deep(.black-text) { color: #000 !important; }
.text-constructor-content :deep(.red-text) { color: #FF0000 !important; }
.text-constructor-content :deep(.green-text) { color: #079D1D !important; }
.text-constructor-content :deep(.blue-text) { color: #4F5BDF !important; }

.text-constructor-content :deep(.font-size-10) { font-size: 10px !important; }
.text-constructor-content :deep(.font-size-12) { font-size: 12px !important; }
.text-constructor-content :deep(.font-size-14) { font-size: 14px !important; }
.text-constructor-content :deep(.font-size-16) { font-size: 16px !important; }
.text-constructor-content :deep(.font-size-18) { font-size: 18px !important; }
.text-constructor-content :deep(.font-size-20) { font-size: 20px !important; }

.text-constructor-content :deep(.font-weight-300) { font-weight: 300 !important; }
.text-constructor-content :deep(.font-weight-400) { font-weight: 400 !important; }
.text-constructor-content :deep(.font-weight-500) { font-weight: 500 !important; }
.text-constructor-content :deep(.font-weight-600) { font-weight: 600 !important; }
.text-constructor-content :deep(.font-weight-900) { font-weight: 900 !important; }

.text-constructor-content :deep(.text-align-left) { text-align: left !important; }
.text-constructor-content :deep(.text-align-center) { text-align: center !important; }
.text-constructor-content :deep(.text-align-right) { text-align: right !important; }
</style>
