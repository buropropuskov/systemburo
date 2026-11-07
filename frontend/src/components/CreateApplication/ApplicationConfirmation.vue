<template>
  <div>
    <label v-if="document">
      <input
        :checked="Boolean(modelValue)"
        type="checkbox"
        :data-testid="testId"
        @change="confirm"
      >
      {{ label }}
    </label>
    <button
      v-if="document"
      type="button"
      class="revision-link"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      Редакция № {{ document.version }}
    </button>
    <BaseModal
      v-if="document"
      :show="expanded"
      :title="`Обработка персональных данных · редакция № ${document.version}`"
      width="720px"
      @close="expanded = false"
    >
      <article v-html="safeText" />
    </BaseModal>
    <div v-if="failed">
      Не удалось загрузить документ.
      <button type="button" class="lk-button lk-button--ghost" @click="load">Повторить</button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import { getConsentGate } from '@/api/pdConsent';
import { sanitizeHtml, stripHtml } from '@/utils/sanitize';

const props = defineProps({
  modelValue: { type: Object, default: null },
  label: { type: String, required: true },
  testId: { type: String, required: true },
});
const emit = defineEmits(['update:modelValue']);
const document = ref(null);
const failed = ref(false);
const expanded = ref(false);
const safeText = computed(() => sanitizeHtml(document.value?.text || ''));

async function load() {
  failed.value = false;
  document.value = null;
  try {
    const current = await getConsentGate();
    if (!current?.hash || !stripHtml(sanitizeHtml(current.text || '')).trim()) {
      failed.value = true;
      return;
    }
    document.value = current;
    if (props.modelValue && (props.modelValue.version !== current.version || props.modelValue.hash !== current.hash)) {
      emit('update:modelValue', null);
    }
  } catch {
    failed.value = true;
  }
}

function confirm(event) {
  const { version, hash } = document.value;
  emit('update:modelValue', event.target.checked ? { version, hash } : null);
}

onMounted(load);
</script>

<style scoped>
label {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  cursor: pointer;
}

.revision-link {
  margin: 4px 0 0 24px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--accent-text);
  font: inherit;
  text-decoration: underline;
  text-underline-position: under;
  cursor: pointer;
}

input {
  flex-shrink: 0;
}

article {
  overflow-wrap: anywhere;
}
</style>
