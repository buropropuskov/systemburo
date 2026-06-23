<template>
  <div class="guide-mgmt dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Управление руководством
      </h3>
      <div class="header-controls">
        <RefreshButton
          :loading="isLoading"
          @refresh="load"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список разделов (фиксированный набор) -->
      <div
        class="table-section"
        :class="{ 'with-details': selectedRole }"
      >
        <div class="table-container">
          <div class="table-header">
            <div class="header-col role-col">
              <p>Раздел</p>
            </div>
            <div class="header-col file-col">
              <p>Файл</p>
            </div>
          </div>

          <div class="table-body">
            <div
              v-if="isLoading && !sections.length"
              class="guide-loading"
            >
              <LoaderSpinner />
            </div>
            <template v-else>
              <div
                v-for="s in sections"
                :key="s.role"
                class="table-row"
                data-testid="guide-row"
                :class="{ selected: selectedRole === s.role }"
                @click="selectSection(s.role)"
              >
                <div class="table-col role-col">
                  <span
                    class="truncate-text"
                    :title="s.title"
                  >{{ s.title }}</span>
                </div>
                <div class="table-col file-col">
                  <span
                    v-if="s.file"
                    class="file-flag file-flag--yes"
                  >PDF</span>
                  <span
                    v-else
                    class="file-flag file-flag--no"
                  >нет</span>
                </div>
              </div>
              <div
                v-if="!sections.length"
                class="no-results"
              >
                Разделы не найдены
              </div>
            </template>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего: {{ sections.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - редактор выбранного раздела -->
      <div
        v-if="selectedSection"
        class="details-section"
        data-testid="guide-details"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedSection.title }}
              </h3>
            </div>
          </div>

          <div class="details-body">
            <label class="field-label">Вступительный текст</label>
            <textarea
              v-model="form.lead"
              class="lk-textarea"
              rows="3"
              maxlength="2000"
              placeholder="Короткое описание раздела"
              data-testid="guide-lead"
            />

            <label class="field-label items-label">Пункты «Что внутри»</label>
            <div
              class="items-editor"
              data-testid="guide-items"
            >
              <div
                v-for="(item, i) in form.items"
                :key="i"
                class="item-row"
              >
                <input
                  v-model="form.items[i]"
                  type="text"
                  class="lk-input"
                  maxlength="300"
                  placeholder="Текст пункта"
                  data-testid="guide-item-input"
                >
                <button
                  type="button"
                  class="item-remove"
                  aria-label="Удалить пункт"
                  data-testid="guide-item-remove"
                  @click="removeItem(i)"
                >
                  ×
                </button>
              </div>
              <button
                type="button"
                class="add-item-btn"
                data-testid="guide-add-item"
                @click="addItem"
              >
                + Добавить пункт
              </button>
            </div>

            <div
              v-if="error"
              class="form-error"
            >
              {{ error }}
            </div>

            <label class="field-label file-label">Файл руководства (PDF)</label>
            <div
              v-if="selectedSection.file"
              class="file-card"
            >
              <span class="file-card__icon">
                <FileTypeIcon
                  :ext="selectedSection.file.ext || 'pdf'"
                  :size="40"
                />
              </span>
              <div class="file-card__main">
                <div class="file-card__name">
                  {{ selectedSection.file.name }}
                </div>
                <div class="file-card__meta">
                  {{ fileTypeLabel(selectedSection.file) }}
                  <span class="sep">·</span>
                  {{ formatSize(selectedSection.file.size) }}
                  <span class="sep">·</span>
                  обновлено {{ formatDate(selectedSection.file.updated_at) }}
                </div>
              </div>
              <div class="file-card__actions">
                <button
                  type="button"
                  class="file-action file-action--ghost"
                  :disabled="downloadingRole === selectedSection.role"
                  data-testid="guide-download"
                  @click="download(selectedSection)"
                >
                  {{ downloadingRole === selectedSection.role ? 'Скачивание…' : 'Скачать' }}
                </button>
                <button
                  type="button"
                  class="file-action file-action--ghost"
                  :disabled="isFileBusy"
                  data-testid="guide-replace"
                  @click="triggerUpload"
                >
                  Заменить
                </button>
                <button
                  type="button"
                  class="file-action file-action--danger"
                  :disabled="isFileBusy"
                  data-testid="guide-delete-file"
                  @click="removeFile"
                >
                  Удалить
                </button>
              </div>
            </div>
            <div
              v-else
              class="file-card file-card--empty"
            >
              <span class="file-card__icon">
                <FileTypeIcon
                  ext="pdf"
                  :size="40"
                />
              </span>
              <div class="file-card__main">
                <div class="file-card__name">
                  Файл руководства ещё не загружен
                </div>
                <div class="file-card__meta">
                  Загрузите PDF, чтобы он стал доступен для скачивания
                </div>
              </div>
              <div class="file-card__actions">
                <button
                  type="button"
                  class="file-action file-action--primary"
                  :disabled="isFileBusy"
                  data-testid="guide-upload"
                  @click="triggerUpload"
                >
                  {{ isUploading ? 'Загрузка…' : 'Загрузить' }}
                </button>
              </div>
            </div>

            <input
              ref="fileInput"
              type="file"
              accept="application/pdf,.pdf"
              class="file-input-hidden"
              data-testid="guide-file-input"
              @change="onFileChange"
            >

            <div class="save-row">
              <button
                type="button"
                class="lk-button lk-button--primary"
                :disabled="!isDirty || isSaving"
                data-testid="guide-save"
                @click="save"
              >
                {{ isSaving ? 'Сохранение…' : 'Сохранить' }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите раздел для редактирования</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue';
import RefreshButton from './RefreshButton.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import FileTypeIcon from './ui/FileTypeIcon.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import {
  listAllGuideSections,
  updateGuideSection,
  uploadGuideFile,
  deleteGuideFile,
  downloadGuideFile,
} from '@/api/guide';

const sections = ref([]);
const selectedRole = ref(null);
const isLoading = ref(false);
const isSaving = ref(false);
const isUploading = ref(false);
const downloadingRole = ref(null);
const error = ref('');
const fileInput = ref(null);

const form = reactive({ lead: '', items: [] });

const selectedSection = computed(
  () => sections.value.find((s) => s.role === selectedRole.value) || null,
);

const isFileBusy = computed(() => isUploading.value || isSaving.value);

const isDirty = computed(() => {
  const sec = selectedSection.value;
  if (!sec) return false;
  if (form.lead.trim() !== (sec.lead || '').trim()) return true;
  const formItems = form.items.map((s) => s.trim()).filter(Boolean);
  const secItems = (sec.items || []).map((s) => s.trim()).filter(Boolean);
  if (formItems.length !== secItems.length) return true;
  return formItems.some((it, i) => it !== secItems[i]);
});

function resetForm(sec) {
  form.lead = sec?.lead || '';
  form.items = [...(sec?.items || [])];
  error.value = '';
}

function replaceSection(updated) {
  const i = sections.value.findIndex((s) => s.role === updated.role);
  if (i !== -1) sections.value.splice(i, 1, updated);
}

async function load() {
  isLoading.value = true;
  try {
    const data = await listAllGuideSections();
    sections.value = Array.isArray(data) ? data : [];
    if (selectedRole.value && !selectedSection.value) selectedRole.value = null;
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'разделы руководства', type: 'error' });
    if (import.meta.env.DEV) console.error(e);
  } finally {
    isLoading.value = false;
  }
}

async function selectSection(role) {
  if (role === selectedRole.value) return;
  if (isDirty.value && !(await confirmIfAnyDirty())) return;
  selectedRole.value = role;
  resetForm(selectedSection.value);
}

function addItem() {
  form.items.push('');
}

function removeItem(i) {
  form.items.splice(i, 1);
}

async function save() {
  const sec = selectedSection.value;
  if (!sec || !isDirty.value) return;
  error.value = '';
  isSaving.value = true;
  try {
    const items = form.items.map((s) => s.trim()).filter(Boolean);
    const updated = await updateGuideSection(sec.role, { lead: form.lead.trim(), items });
    replaceSection(updated);
    resetForm(updated);
    useDeletionsStore().notify({ prefix: 'Раздел ', bold: updated.title, suffix: ' сохранён' });
  } catch (e) {
    error.value = e?.message || 'Не удалось сохранить раздел';
    useDeletionsStore().notify({ prefix: 'Не удалось сохранить раздел: ', bold: e?.message || 'ошибка', type: 'error' });
  } finally {
    isSaving.value = false;
  }
}

function triggerUpload() {
  fileInput.value?.click();
}

function isPdf(file) {
  return file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf');
}

async function onFileChange(e) {
  const file = e.target.files?.[0];
  if (!file) return;
  const sec = selectedSection.value;
  if (!sec) return;
  if (!isPdf(file)) {
    useDeletionsStore().notify({ prefix: 'Можно загрузить только ', bold: 'PDF-файл', type: 'error' });
    e.target.value = '';
    return;
  }
  isUploading.value = true;
  try {
    const updated = await uploadGuideFile(sec.role, file);
    replaceSection(updated);
    useDeletionsStore().notify({ prefix: 'Файл раздела ', bold: updated.title, suffix: ' загружен' });
  } catch (err) {
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить файл: ', bold: err?.message || 'ошибка', type: 'error' });
  } finally {
    isUploading.value = false;
    e.target.value = '';
  }
}

async function removeFile() {
  const sec = selectedSection.value;
  if (!sec?.file) return;
  const ok = await useUiStore().confirm({
    title: 'Удалить файл',
    message: `Удалить PDF раздела «${sec.title}»? Скачивание станет недоступно.`,
    confirmText: 'Удалить',
    danger: true,
  });
  if (!ok) return;
  isUploading.value = true;
  try {
    const updated = await deleteGuideFile(sec.role);
    replaceSection(updated);
    useDeletionsStore().notify({ prefix: 'Файл раздела ', bold: updated.title, suffix: ' удалён' });
  } catch (err) {
    useDeletionsStore().notify({ prefix: 'Не удалось удалить файл: ', bold: err?.message || 'ошибка', type: 'error' });
  } finally {
    isUploading.value = false;
  }
}

async function download(sec) {
  if (!sec?.file) return;
  downloadingRole.value = sec.role;
  try {
    await downloadGuideFile(sec.file.download_url, sec.file.name);
  } catch (err) {
    useDeletionsStore().notify({ prefix: 'Не удалось скачать файл: ', bold: err?.message || 'ошибка', type: 'error' });
  } finally {
    downloadingRole.value = null;
  }
}

function fileTypeLabel(file) {
  const ext = (file.ext || '').replace(/^\./, '').toUpperCase();
  return ext || 'PDF';
}

function formatSize(bytes) {
  if (!bytes) return '';
  const mb = bytes / (1024 * 1024);
  if (mb >= 1) return `${mb.toFixed(1).replace('.', ',')} МБ`;
  const kb = Math.max(1, Math.round(bytes / 1024));
  return `${kb} КБ`;
}

function formatDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

let unregisterDirty = null;
onMounted(() => {
  unregisterDirty = registerDirtyTracker({
    isDirty: () => isDirty.value,
    save: () => save(),
  });
  load();
});
onBeforeUnmount(() => {
  if (unregisterDirty) unregisterDirty();
});

defineExpose({ load });
</script>

<style scoped>
.guide-mgmt {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
  gap: 12px;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

/* Master-detail layout (эталон TableConstructor) */
.content-container {
  display: flex;
  height: 500px;
  width: 100%;
  overflow: hidden;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
  background: #fff;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  user-select: none;
}

.header-col p {
  margin: 0;
}

.role-col {
  width: 70%;
  min-width: 140px;
}

.file-col {
  width: 30%;
  min-width: 70px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 48px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f8f9ff;
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
  font-weight: 500;
  color: #000;
}

.file-flag {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 9px;
  border-radius: 50px;
}

.file-flag--yes {
  background: #eef0ff;
  color: #4F5BDF;
}

.file-flag--no {
  background: #f3f4f6;
  color: #a2a2a2;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

.guide-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

/* Details */
.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fff;
  line-height: 1.5;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  gap: 12px;
}

.details-title-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  min-width: 0;
}

.details-title {
  margin: 0;
  color: #000;
  font-size: 1.2em;
  font-weight: 600;
  word-break: break-word;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 0.85em;
  color: #666;
  font-weight: 500;
}

.items-label,
.file-label {
  margin-top: 12px;
}

.items-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.item-row .lk-input {
  flex: 1;
}

.item-remove {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border: 1px solid #fecaca;
  background: #fff;
  color: #dc3545;
  border-radius: 50%;
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s;
}

.item-remove:hover {
  background: #fff1f2;
  border-color: #dc3545;
}

.add-item-btn {
  align-self: flex-start;
  padding: 6px 14px;
  border: 1px dashed #c8ccf5;
  background: #fff;
  color: #4F5BDF;
  border-radius: 30px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s, border-color 0.2s;
}

.add-item-btn:hover {
  background: #eef0ff;
  border-color: #4F5BDF;
}

.form-error {
  color: #d73a3a;
  font-size: 0.85em;
}

/* Файл-карточка (образец B2/DocumentsBlock) */
.file-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  background: #fafbff;
}

.file-card--empty {
  background: #f6f6f9;
}

.file-card--empty .file-card__name {
  color: #6a6a7d;
}

.file-card__icon {
  flex-shrink: 0;
  display: inline-flex;
}

.file-card--empty .file-card__icon {
  opacity: 0.55;
}

.file-card__main {
  flex: 1;
  min-width: 0;
}

.file-card__name {
  font-size: 14.5px;
  font-weight: 700;
  color: #1a1a2e;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-card__meta {
  margin-top: 4px;
  font-size: 12px;
  color: #a2a2b4;
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
}

.file-card__meta .sep {
  color: #d7d9e8;
}

.file-card__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.file-action {
  display: inline-flex;
  align-items: center;
  height: 36px;
  padding: 0 16px;
  border-radius: 30px;
  font-weight: 600;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
  white-space: nowrap;
}

.file-action:disabled {
  opacity: 0.6;
  cursor: progress;
}

.file-action--primary {
  border: none;
  background: #4F5BDF;
  color: #fff;
}

.file-action--primary:hover:not(:disabled) {
  background: #4049c4;
}

.file-action--ghost {
  border: 1px solid #4F5BDF;
  background: #fff;
  color: #4F5BDF;
}

.file-action--ghost:hover:not(:disabled) {
  background: #eef0ff;
}

.file-action--danger {
  border: 1px solid #fecaca;
  background: #fff;
  color: #dc3545;
}

.file-action--danger:hover:not(:disabled) {
  background: #fff1f2;
  border-color: #dc3545;
}

.file-input-hidden {
  display: none;
}

.save-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
}

@media (max-width: 768px) {
  .management-header {
    flex-direction: column;
    height: auto;
    padding: 16px;
    align-items: stretch;
    gap: 12px;
  }
  .content-container {
    flex-direction: column;
    height: auto;
  }
  .table-section,
  .table-section.with-details,
  .details-section,
  .no-selection-message {
    width: 100%;
  }
  .table-section {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
  }
  .table-body {
    max-height: 300px;
  }
  .file-card {
    flex-wrap: wrap;
  }
  .file-card__actions {
    width: 100%;
  }
}
</style>
