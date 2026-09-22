<template>
  <div
    v-if="doc"
    class="details-section"
  >
    <div class="doc-detail-preview">
      <FileTypeIcon
        :ext="doc.file_ext || 'file'"
        :size="48"
      />
      <div>
        <div class="doc-detail-filename">
          {{ doc.file_name }}
        </div>
        <div class="doc-detail-meta">
          {{ formatBytes(doc.file_size) }} &middot;
          загружен {{ formatDate(doc.created_at) }}
        </div>
      </div>
    </div>

    <div class="form-group">
      <label class="form-label">Наименование (видно на сайте)</label>
      <input
        v-model="title"
        type="text"
        maxlength="255"
        class="lk-input"
      >
    </div>

    <div class="form-group">
      <label class="form-label">Описание (серый текст)</label>
      <textarea
        v-model="description"
        rows="3"
        class="lk-input lk-textarea"
      />
    </div>

    <div class="form-group">
      <label class="form-label">Пояснение бюро (видно в окне документа)</label>
      <textarea
        v-model="comment"
        rows="4"
        class="lk-input lk-textarea"
        data-testid="document-comment-field"
        placeholder="Зачем нужен документ, как заполнять, куда нести"
      />
    </div>

    <div class="form-group">
      <label class="form-label">Группа</label>
      <select
        v-model="group_id"
        class="lk-select"
      >
        <option :value="null">
          — без группы (Прочее) —
        </option>
        <option
          v-for="g in groups"
          :key="g.id"
          :value="g.id"
        >
          {{ g.name }}
        </option>
      </select>
    </div>

    <div class="form-group">
      <label class="form-label">Дата публикации</label>
      <input
        v-model="published_at"
        type="date"
        class="lk-input"
        style="max-width: 200px"
      >
    </div>

    <div class="switch-row">
      <div>
        <div class="switch-label">
          Показывать на «Обзор и новости»
        </div>
        <div class="switch-desc">
          Скрытый документ остаётся в админке, но не виден пользователям
        </div>
      </div>
      <button
        class="toggle-switch"
        :class="{ 'toggle-switch--on': is_visible }"
        :aria-pressed="is_visible"
        @click="is_visible = !is_visible"
      />
    </div>

    <div
      v-if="error"
      class="form-error"
    >
      {{ error }}
    </div>

    <div class="detail-actions">
      <button
        class="lk-button lk-button--ghost"
        @click="$emit('download')"
      >
        Скачать
      </button>
      <label
        class="lk-button lk-button--ghost"
        style="cursor: pointer;"
      >
        Заменить файл
        <input
          ref="replaceFileInput"
          type="file"
          accept=".doc,.docx,.pdf,.xlsx,.pptx"
          style="display: none"
          @change="$emit('replace-file', $event)"
        >
      </label>
      <button
        class="lk-button lk-button--primary"
        :disabled="isSaving"
        @click="$emit('save')"
      >
        Сохранить
      </button>
      <button
        class="lk-button lk-button--danger"
        :disabled="isDeleting"
        @click="$emit('delete')"
      >
        Удалить
      </button>
    </div>
  </div>
</template>

<script>
import FileTypeIcon from '@/components/ui/FileTypeIcon.vue';
import { formatMomentDate } from '@/utils/datetime';
import { formatBytes } from '@/utils/download';

/**
 * Панель правки документа в админке: наименование, описание строкой списка,
 * пояснение бюро для окна документа, группа, дата и видимость.
 *
 * Вынесена из DocumentsManagement: тот упирался в предел размера шаблона, а поля
 * документа продолжают прибавляться.
 */
const FORM_FIELDS = ['title', 'description', 'comment', 'group_id', 'published_at', 'is_visible'];

export default {
  name: 'DocumentEditPanel',
  components: { FileTypeIcon },
  props: {
    doc: { type: Object, default: null },
    form: { type: Object, required: true },
    groups: { type: Array, default: () => [] },
    error: { type: String, default: '' },
    isSaving: { type: Boolean, default: false },
    isDeleting: { type: Boolean, default: false },
    isReplacing: { type: Boolean, default: false },
  },
  emits: ['save', 'delete', 'download', 'replace-file', 'update:form'],
  // Поля читаются прямо из пропса, а правка уходит наружу новым объектом: форму
  // держит родитель. Локальная копия с парой deep-watch зацикливала обновления -
  // каждый ввод символа гонял update:form по кругу и ронял вкладку (#2561).
  computed: FORM_FIELDS.reduce((acc, field) => ({
    ...acc,
    [field]: {
      get() { return this.form[field]; },
      set(value) { this.$emit('update:form', { ...this.form, [field]: value }); },
    },
  }), {}),
  methods: {
    formatBytes,
    formatDate(dt) {
      return dt ? formatMomentDate(new Date(dt)) : '';
    },
  },
};
</script>
