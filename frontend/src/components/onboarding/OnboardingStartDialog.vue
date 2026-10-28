<script setup>
import { ref, watch, computed } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
const props = defineProps({ show: Boolean, busy: Boolean, canResume: Boolean,
  chapters: { type: Array, default: () => [] } });
const emit = defineEmits(['close', 'start', 'resume']);
const selected = ref(null);
const selectedAvailable = computed(() => props.chapters.some(item => item.id === selected.value));
watch(() => props.show, () => { selected.value = null; });
watch(() => props.chapters, () => { if (!selectedAvailable.value) selected.value = null; });
</script>
<template>
  <BaseModal
    :show="show"
    title="Обучение работе с системой"
    width="560px"
    content-class="ob-start-dialog"
    @close="emit('close')"
  >
    <div class="ob-start-dialog__body">
      <p class="ob-start-dialog__intro">
        Покажем основные действия последовательно.
      </p>
      <button
        class="lk-button lk-button--primary ob-start-dialog__primary"
        :disabled="busy"
        data-testid="ob-start-full"
        @click="emit('start', {})"
      >
        Пройти сначала
      </button>
      <button
        v-if="canResume"
        class="lk-button lk-button--secondary ob-start-dialog__resume"
        :disabled="busy"
        data-testid="ob-start-resume"
        @click="emit('resume')"
      >
        Продолжить обучение
      </button>
      <div class="ob-start-dialog__section">
        <label>Нужен конкретный раздел?</label>
        <BaseDropdown
          v-model="selected"
          :options="chapters"
          placeholder="Выбрать раздел"
          :disabled="busy"
          teleport
        />
        <button
          class="lk-button lk-button--secondary"
          :disabled="busy || !selectedAvailable"
          data-testid="ob-start-section"
          @click="emit('start', { fromStepId: selected })"
        >
          Начать с раздела
        </button>
        <small>Просмотр с раздела не засчитывает весь тур.</small>
      </div>
    </div>
  </BaseModal>
</template>
<style scoped>
.ob-start-dialog__body { padding: 16px 20px; }
.ob-start-dialog__intro { margin: 0 0 20px; line-height: 1.5; }
.ob-start-dialog__primary,
.ob-start-dialog__resume { width: 100%; min-height: 44px; }
.ob-start-dialog__resume { margin-top: 12px; }
.ob-start-dialog__section { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 12px; }
.ob-start-dialog__section label { font-size: 13px; font-weight: 600; }
.ob-start-dialog__section small { font-size: 11px; line-height: 1.5; color: var(--text-muted); }
</style>
