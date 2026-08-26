<template>
  <BaseModal
    :show="show"
    title="Экспорт в Excel"
    width="380px"
    @close="$emit('close')"
  >
    <div class="export-modal">
      <p class="export-modal__label">
        Выберите данные для экспорта:
      </p>
      <div class="export-modal__options">
        <button
          class="export-option"
          :class="{ 'export-option--active': selected === 'both' }"
          type="button"
          @click="selected = 'both'"
        >
          <span class="export-option__dot" />
          Факт + Основная таблица
        </button>
        <button
          class="export-option"
          :class="{ 'export-option--active': selected === 'main' }"
          type="button"
          @click="selected = 'main'"
        >
          <span class="export-option__dot" />
          Только основная таблица
        </button>
        <button
          class="export-option"
          :class="{ 'export-option--active': selected === 'fact' }"
          type="button"
          @click="selected = 'fact'"
        >
          <span class="export-option__dot" />
          Только факт
        </button>
      </div>
    </div>
    <template #actions>
      <button
        class="lk-button lk-button--secondary"
        type="button"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="lk-button lk-button--primary"
        type="button"
        @click="confirm"
      >
        Экспортировать
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';

export default {
  name: 'TableExportModal',
  components: { BaseModal },
  props: {
    show: {
      type: Boolean,
      required: true,
    },
  },
  emits: ['close', 'export'],
  data() {
    return {
      selected: 'both',
    };
  },
  watch: {
    show(val) {
      // Сбрасываем выбор при каждом открытии
      if (val) this.selected = 'both';
    },
  },
  methods: {
    confirm() {
      this.$emit('export', this.selected);
      this.$emit('close');
    },
  },
};
</script>

<style scoped>
.export-modal {
  padding: 20px 20px 8px;
}

.export-modal__label {
  margin: 0 0 12px;
  font-size: 14px;
  color: var(--color-text-secondary, var(--text-muted));
}

.export-modal__options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.export-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1.5px solid var(--color-border, var(--border));
  border-radius: var(--radius-md, 15px);
  background: var(--surface);
  color: var(--color-text, var(--text));
  font-size: 14px;
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s, background 0.15s;
}

.export-option:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.export-option--active {
  border-color: var(--accent);
  background: var(--accent-tint);
  font-weight: 500;
}

.export-option__dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid var(--color-border, var(--border));
  flex-shrink: 0;
  transition: border-color 0.15s, background 0.15s;
}

.export-option--active .export-option__dot {
  border-color: var(--accent);
  background: var(--accent);
}
</style>
