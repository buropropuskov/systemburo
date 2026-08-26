<template>
  <BaseModal
    :show="show"
    title="Скопировать привязки"
    width="520px"
    :z-index="1100"
    content-class="mc-modal"
    @close="$emit('close')"
  >
    <div class="mc-body">
      <div class="mc-target">
        Куда: <strong>{{ targetFileName || 'текущий бланк' }}</strong>
      </div>

      <div class="mc-field">
        <label class="mc-label">Откуда взять привязки</label>
        <BaseDropdown
          v-model="selectedTemplateId"
          :options="options"
          label-key="label"
          value-key="template_id"
          placeholder="Выберите шаблон"
          searchable
          teleport
          :menu-z-index="2100"
          :disabled="loading || !options.length"
          data-testid="copy-source-select"
        />
        <span
          v-if="!loading && !options.length"
          class="mc-empty"
        >Настроенных бланков, кроме этого, нет</span>
      </div>

      <label class="mc-check">
        <input
          v-model="replace"
          type="checkbox"
          data-testid="copy-replace"
        >
        <span>Заменить текущие привязки ({{ currentMappingsCount }})</span>
      </label>
      <span class="mc-check-note">{{ replace ? 'Текущие удалятся.' : 'Добавятся к текущим, дубли пропустятся.' }}</span>

      <label class="mc-check">
        <input
          v-model="copyParams"
          type="checkbox"
          data-testid="copy-params"
        >
        <span>Перенести границы списка и разделитель</span>
      </label>

      <div
        v-if="unsavedChanges"
        class="mc-warning"
        data-testid="copy-unsaved-warning"
      >
        В редакторе есть несохранённые привязки. Перенос идёт по сохранённому шаблону, поэтому эти
        правки потеряются: сохраните их до переноса.
      </div>

      <div
        v-if="typeMismatch"
        class="mc-warning"
        data-testid="copy-type-warning"
      >
        У источника тип «{{ typeLabel(selectedSource.attachment_type) }}», у этого бланка -
        «{{ typeLabel(attachmentType) }}». Привязки списка не перенесутся: заполнять их нечем.
      </div>
    </div>

    <template #actions>
      <button
        class="lk-button lk-button--ghost"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="lk-button lk-button--primary"
        :disabled="!selectedTemplateId || saving"
        data-testid="copy-submit"
        @click="submit"
      >
        {{ saving ? 'Переношу...' : 'Скопировать' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import { listTemplateSources, copyMappings } from '@/api/attachment-templates';
import { useDeletionsStore } from '@/stores/deletions';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const TYPE_LABELS = { cars: 'автомобили', people: 'сотрудники', items: 'имущество' };

export default {
  name: 'AttachmentMappingCopyModal',
  components: { BaseModal, BaseDropdown },
  props: {
    show: { type: Boolean, required: true },
    uniqueAttachmentId: { type: Number, required: true },
    attachmentType: { type: String, default: '' },
    currentMappingsCount: { type: Number, default: 0 },
    // Исключаем из источников только САМ активный шаблон: у одного типа вложения
    // несколько файлов, и перенос между ними - основной случай.
    currentTemplateId: { type: Number, default: 0 },
    targetFileName: { type: String, default: '' },
    // Перенос идёт по серверному состоянию шаблона, поэтому несохранённые правки
    // привязок после него потеряются - об этом честно предупреждаем.
    unsavedChanges: { type: Boolean, default: false },
  },
  emits: ['close', 'copied'],
  data() {
    return {
      sources: [],
      loading: false,
      saving: false,
      selectedTemplateId: null,
      replace: true,
      copyParams: false,
    };
  },
  computed: {
    // Файлы этого же типа вложения - первыми: перенос между ними основной случай.
    // Исключаем только активный шаблон, копировать привязки в себя же нечего.
    options() {
      const own = [];
      const others = [];
      for (const s of this.sources) {
        if (s.template_id === this.currentTemplateId || !s.mappings_count) continue;
        const count = `${s.mappings_count} прив.`;
        if (s.unique_attachment_id === this.uniqueAttachmentId) {
          own.push({ ...s, label: `${s.original_file_name || 'без имени'} - ${count}` });
        } else {
          others.push({
            ...s,
            label: `${s.attachment_name || 'без названия'} (${this.typeLabel(s.attachment_type)}) - ${count}`,
          });
        }
      }
      return [...own, ...others];
    },
    selectedSource() {
      return this.options.find(s => s.template_id === this.selectedTemplateId) || null;
    },
    typeMismatch() {
      return !!this.selectedSource && !!this.attachmentType
        && this.selectedSource.attachment_type !== this.attachmentType;
    },
  },
  watch: {
    show(val) {
      if (val) this.reset();
    },
  },
  methods: {
    typeLabel(type) {
      return TYPE_LABELS[type] || type || 'без типа';
    },
    async reset() {
      this.selectedTemplateId = null;
      this.replace = true;
      this.copyParams = false;
      this.loading = true;
      try {
        const data = await listTemplateSources();
        this.sources = Array.isArray(data) ? data : [];
      } catch {
        this.sources = [];
        useDeletionsStore().notify({ bold: 'Не удалось получить список шаблонов', type: 'error' });
      } finally {
        this.loading = false;
      }
    },
    async submit() {
      if (!this.selectedTemplateId || this.saving) return;
      this.saving = true;
      try {
        const res = await copyMappings(this.uniqueAttachmentId, {
          sourceTemplateID: this.selectedTemplateId,
          replace: this.replace,
          copyParams: this.copyParams,
        });
        useDeletionsStore().notify({
          prefix: 'Перенесено привязок: ',
          bold: String(res?.copied ?? 0),
          suffix: this.skippedSuffix(res),
        });
        this.$emit('copied', res);
        this.$emit('close');
      } catch (err) {
        useDeletionsStore().notify({
          prefix: 'Не удалось перенести привязки: ',
          bold: err.message || 'ошибка сервера',
          type: 'error',
        });
      } finally {
        this.saving = false;
      }
    },
    // Пропуски показываем явно: иначе непонятно, почему привязок меньше, чем у источника.
    skippedSuffix(res) {
      if (!res) return '';
      const parts = [];
      if (res.skipped_foreign_list) parts.push(`списка чужого типа: ${res.skipped_foreign_list}`);
      if (res.skipped_custom) parts.push(`своих полей без пары: ${res.skipped_custom}`);
      if (res.skipped_duplicates) parts.push(`дублей: ${res.skipped_duplicates}`);
      return parts.length ? `, пропущено - ${parts.join(', ')}` : '';
    },
  },
};
</script>

<style scoped>
/* base-modal__body идёт без padding - отступы несёт содержимое (как у соседних окон). */
.mc-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 20px 18px;
}

.mc-target {
  font-size: 13px;
  color: var(--text-secondary, #666);
}

.mc-target strong {
  color: var(--text-primary, #1a1a1a);
}

.mc-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.mc-label {
  font-size: 13px;
  font-weight: 500;
}

.mc-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  cursor: pointer;
}

.mc-check input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.mc-check-note {
  margin-top: -8px;
  font-size: 12px;
  color: var(--text-secondary, #666);
}

.mc-empty {
  font-size: 12px;
  color: var(--text-secondary, #666);
}

.mc-warning {
  padding: 8px 10px;
  border-radius: var(--radius-md, 15px);
  background: var(--warning-bg, #fff3cd);
  border: 1px solid var(--warning, #ffc107);
  color: var(--warning-text, #856404);
  font-size: 12px;
  line-height: 1.4;
}
</style>
