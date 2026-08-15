<template>
  <BaseModal
    :show="show"
    :title="`Сообщение к заявке ${applicationNumber}`"
    width="760px"
    :z-index="12000"
    radius="30px"
    @close="$emit('close')"
  >
    <template #header>
      <div class="msg-modal-title">
        <span class="msg-modal-title__main">Сообщение к заявке</span>
        <span
          v-if="applicationNumber"
          class="msg-modal-title__num"
        >{{ applicationNumber }}</span>
      </div>
    </template>
    <div
      class="text-constructor-content"
      v-html="sanitizedMessage"
    />
  </BaseModal>
</template>

<script setup>
import { computed } from 'vue';
import { sanitizeHtml } from '@/utils/sanitize';
import BaseModal from '@/components/ui/BaseModal.vue';

const props = defineProps({
  show: { type: Boolean, default: false },
  message: { type: String, default: '' },
  applicationNumber: { type: [String, Number], default: '' },
});

defineEmits(['close']);

const sanitizedMessage = computed(() => sanitizeHtml(props.message));
</script>

<style scoped>
/* На широком экране заголовок идёт одной строкой, номер - следом за названием.
   На узком возвращается перенос в две строки: "Сообщение к заявке 20260712/001"
   в строку не помещалось (#1097 R3-10). */
.msg-modal-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

@media (max-width: 767.98px) {
  .msg-modal-title {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
}

.msg-modal-title__main {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.2;
  color: var(--color-text, var(--text));
}

.msg-modal-title__num {
  font-size: 15px;
  white-space: nowrap;
  font-weight: 500;
  line-height: 1.2;
  color: var(--text-muted);
}

/* Безопасный рендер форматированного сообщения (render-safety) */
.text-constructor-content {
  padding: 25px;
  font-size: 15px;
  line-height: 1.5;
  color: var(--text);
  word-break: break-word;
}

.text-constructor-content :deep(img) {
  max-width: 100%;
  height: auto;
}

.text-constructor-content :deep(img:not([height])) {
  height: auto;
}

.text-constructor-content :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.text-constructor-content :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.text-constructor-content :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }
.text-constructor-content::after { content: ''; display: block; clear: both; }

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
.text-constructor-content :deep(.blue-text) { color: var(--accent-text) !important; }

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
