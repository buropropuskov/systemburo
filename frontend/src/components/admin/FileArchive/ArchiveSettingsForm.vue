<template>
  <div class="asf">
    <div
      v-if="loading"
      class="asf__loading"
    >
      <LoaderSpinner label="Загрузка настроек..." />
    </div>

    <p
      v-else-if="loadError"
      class="form-error"
    >
      {{ loadError }}
    </p>

    <template v-else>
      <section class="asf__section">
        <ToggleSwitch v-model="form.enabled">
          Выгрузка бланков в файловый архив
        </ToggleSwitch>
        <!-- Подпись обязана следовать за тумблером: статический текст «пока
             выключено» под включённым рубильником утверждает обратное тому, что
             система делает, и администратор считает, что бланки не пишутся. -->
        <p class="field-hint">
          {{ form.enabled
            ? 'Включено: бланки завершённых заявок сохраняются на диск сервера по раскладке ниже.'
            : 'Пока выключено, бланки заявок не сохраняются на диск - только настройка раскладки и порогов.' }}
        </p>
      </section>

      <section class="asf__section">
        <h4 class="asf__section-title">
          Раскладка пути
        </h4>
        <TemplatePatternField
          v-model="form.dirTemplate"
          scope="dir"
          label="Шаблон папки заявки"
          :tokens="tokens"
          :default-template="DEFAULT_DIR_TEMPLATE"
          :problems="preview ? preview.dir_problems : []"
          :preview-text="dirPreviewText"
          :disabled="saving"
        />
        <TemplatePatternField
          v-model="form.fileTemplate"
          scope="file"
          label="Шаблон имени файла"
          :tokens="tokens"
          :default-template="DEFAULT_FILE_TEMPLATE"
          :problems="preview ? preview.file_problems : []"
          :preview-text="filePreviewText"
          :disabled="saving"
        />
        <p
          v-if="fullPreviewText"
          class="asf__full-preview"
          data-testid="asf-full-preview"
        >
          Полный путь: <code>{{ fullPreviewText }}</code>
          <span
            v-if="preview && preview.synthetic"
            class="asf__preview-note"
          >(пример - применённых заявок ещё нет)</span>
        </p>
        <p
          v-else-if="previewError"
          class="form-error"
        >
          {{ previewError }}
        </p>
      </section>

      <section class="asf__section">
        <h4 class="asf__section-title">
          Пороги и лимиты
        </h4>
        <div class="asf__grid">
          <div class="form-group">
            <label class="field-label">Квота архива, МБ</label>
            <input
              v-model.number="quotaMB"
              type="number"
              min="0"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-quota"
            >
            <span class="field-hint">0 - без ограничения</span>
          </div>
          <div class="form-group">
            <label class="field-label">Минимум свободного места, МБ</label>
            <input
              v-model.number="minFreeMB"
              type="number"
              min="0"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-min-free"
            >
            <span class="field-hint">Ниже порога выгрузка встаёт, подача заявок не затрагивается</span>
          </div>
          <div class="form-group">
            <label class="field-label">Порог предупреждения, %</label>
            <input
              v-model.number="form.warnPercent"
              type="number"
              min="1"
              max="99"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-warn-percent"
            >
            <span class="field-hint">Заполнение раздела, после которого приходит уведомление</span>
          </div>
          <div class="form-group">
            <label class="field-label">Окно ночной сверки, дней</label>
            <input
              v-model.number="form.recheckDays"
              type="number"
              min="1"
              max="365"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-recheck-days"
            >
          </div>
          <div class="form-group">
            <label class="field-label">Заморозка после завершения заявки, дней</label>
            <input
              v-model.number="form.freezeAfterDays"
              type="number"
              min="0"
              max="3650"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-freeze-days"
            >
            <span class="field-hint">0 - замораживать сразу при завершении</span>
          </div>
          <div class="form-group">
            <label class="field-label">Потолок одной ZIP-выгрузки, МБ</label>
            <input
              v-model.number="zipMaxMB"
              type="number"
              min="1"
              step="1"
              class="lk-input"
              :disabled="saving"
              data-testid="asf-zip-max"
            >
          </div>
        </div>
      </section>

      <p
        v-if="saveError"
        class="form-error"
      >
        {{ saveError }}
      </p>

      <div class="asf__actions">
        <button
          type="button"
          class="lk-button lk-button--primary"
          data-testid="asf-save"
          :disabled="!isDirty || saving"
          @click="requestSave"
        >
          Сохранить
        </button>
      </div>
    </template>

    <ConfirmationModal
      :show="confirmVisible"
      :title="confirmKind === 'disable' ? 'Выключение файлового архива' : 'Изменение пути папки заявки'"
      :message="confirmMessage"
      confirm-text="Сохранить"
      cancel-text="Отмена"
      @confirm="confirmSave"
      @cancel="confirmVisible = false"
    />
  </div>
</template>

<script setup>
import {
  ref, reactive, computed, watch, onMounted, onBeforeUnmount,
} from 'vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import TemplatePatternField from '@/components/admin/TemplatePatternField.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker } from '@/utils/dirtyTracker';
import {
  getArchiveSettings, updateArchiveSettings, getArchiveTokens, previewArchivePath,
} from '@/api/fileArchive';

/**
 * Вкладка «Настройки» файлового архива (#1615, срез C3): глобальный рубильник, два
 * шаблона раскладки пути (папка заявки / имя файла) через TemplatePatternField и
 * числовые пороги. Оба шаблона гоняют ОДИН debounce-запрос превью (не по одному на
 * поле) - правка любого из них планирует единый POST /file-archive/preview.
 */

// Повторяют internal/blankpath/template.go DefaultDirTemplate/DefaultFileTemplate -
// сервер сам подставляет их, пока настройка ни разу не сохранена, но кнопке
// "Стандартный" в конструкторе нужно готовое значение без похода за настройками.
const DEFAULT_DIR_TEMPLATE = '{год}/{месяц_число} {МЕСЯЦ} {год}/{дата}/{дата} №{номер} {организация}';
const DEFAULT_FILE_TEMPLATE = '{тип} - {организация}';

const PREVIEW_DEBOUNCE_MS = 400;

const emit = defineEmits(['saved']);

const deletions = useDeletionsStore();

const loading = ref(false);
const loadError = ref('');
const saving = ref(false);
const saveError = ref('');
const tokens = ref([]);

function emptyForm() {
  return {
    enabled: false,
    dirTemplate: DEFAULT_DIR_TEMPLATE,
    fileTemplate: DEFAULT_FILE_TEMPLATE,
    quotaBytes: 0,
    minFreeBytes: 0,
    warnPercent: 80,
    recheckDays: 30,
    freezeAfterDays: 30,
    zipMaxBytes: 0,
  };
}

function formFromSettings(s) {
  return {
    enabled: !!s.enabled,
    dirTemplate: s.dir_template || DEFAULT_DIR_TEMPLATE,
    fileTemplate: s.file_template || DEFAULT_FILE_TEMPLATE,
    quotaBytes: s.quota_bytes || 0,
    minFreeBytes: s.min_free_bytes || 0,
    warnPercent: s.warn_percent || 0,
    recheckDays: s.recheck_days || 0,
    freezeAfterDays: s.freeze_after_days || 0,
    zipMaxBytes: s.zip_max_bytes || 0,
  };
}

const form = reactive(emptyForm());
const original = reactive(emptyForm());

// МБ - удобная единица ввода поверх байтовых полей формы; байты остаются
// источником истины для сравнения isDirty, конвертация идёт только на границе UI.
const MB = 1024 * 1024;
const quotaMB = computed({
  get: () => Math.round(form.quotaBytes / MB),
  set: v => { form.quotaBytes = Math.max(0, Math.round(Number(v) || 0)) * MB; },
});
const minFreeMB = computed({
  get: () => Math.round(form.minFreeBytes / MB),
  set: v => { form.minFreeBytes = Math.max(0, Math.round(Number(v) || 0)) * MB; },
});
const zipMaxMB = computed({
  get: () => Math.round(form.zipMaxBytes / MB),
  set: v => { form.zipMaxBytes = Math.max(1, Math.round(Number(v) || 0)) * MB; },
});

const FIELD_LABELS = {
  enabled: 'Выгрузка бланков',
  dirTemplate: 'Шаблон папки заявки',
  fileTemplate: 'Шаблон имени файла',
  quotaBytes: 'Квота архива',
  minFreeBytes: 'Минимум свободного места',
  warnPercent: 'Порог предупреждения',
  recheckDays: 'Окно ночной сверки',
  freezeAfterDays: 'Заморозка после завершения',
  zipMaxBytes: 'Потолок ZIP-выгрузки',
};

const changedFields = computed(() => {
  const c = {};
  for (const key of Object.keys(FIELD_LABELS)) {
    if (form[key] !== original[key]) c[key] = form[key];
  }
  return c;
});
const isDirty = computed(() => Object.keys(changedFields.value).length > 0);

function describeChange(key, value) {
  if (key === 'enabled') return value ? 'Архив включён' : 'Архив выключен';
  if (key === 'dirTemplate' || key === 'fileTemplate') {
    return { label: FIELD_LABELS[key], from: original[key], to: value };
  }
  return { label: FIELD_LABELS[key], from: String(original[key]), to: String(value) };
}

async function load() {
  loading.value = true;
  loadError.value = '';
  try {
    const [settings, tokenList] = await Promise.all([getArchiveSettings(), getArchiveTokens()]);
    applySettings(settings);
    tokens.value = Array.isArray(tokenList) ? tokenList : [];
    runPreview();
  } catch (e) {
    loadError.value = e?.message || 'Не удалось загрузить настройки файлового архива';
    deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'настройки файлового архива', type: 'error' });
  } finally {
    loading.value = false;
  }
}

function applySettings(s) {
  Object.assign(form, formFromSettings(s));
  Object.assign(original, formFromSettings(s));
}

// --- Живое превью: один debounce-запрос на оба шаблона + seq-guard (гонка ответов
// при быстрой правке двух полей подряд не должна затереть актуальный превью старым). ---
const preview = ref(null);
const previewError = ref('');
let previewTimer = null;
let previewSeq = 0;

const dirPreviewText = computed(() => (preview.value ? preview.value.levels.join('/') : ''));
const filePreviewText = computed(() => (preview.value ? preview.value.file_name : ''));
const fullPreviewText = computed(() => (preview.value ? preview.value.rel_path : ''));

function schedulePreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(runPreview, PREVIEW_DEBOUNCE_MS);
}

async function runPreview() {
  const seq = (previewSeq += 1);
  try {
    const data = await previewArchivePath({
      dirTemplate: form.dirTemplate,
      fileTemplate: form.fileTemplate,
    });
    if (seq !== previewSeq) return;
    preview.value = data;
    previewError.value = '';
  } catch (e) {
    if (seq !== previewSeq) return;
    previewError.value = e?.message || 'Не удалось построить превью пути';
  }
}

watch(() => form.dirTemplate, schedulePreview);
watch(() => form.fileTemplate, schedulePreview);

// --- Сохранение: подтверждение на выключение архива и на смену пути папки. ---
const confirmVisible = ref(false);
const confirmKind = ref('');
const confirmMessage = computed(() => (confirmKind.value === 'disable'
  ? 'Выключить файловый архив? Новые бланки перестанут сохраняться на диск, уже выгруженные файлы останутся на месте.'
  : 'Изменить шаблон папки заявки? Новые заявки лягут по новому пути, уже выгруженные файлы переедут туда же при следующей выгрузке этой заявки.'));

function requestSave() {
  if (!isDirty.value || saving.value) return;
  const changes = changedFields.value;
  const turningOff = Object.prototype.hasOwnProperty.call(changes, 'enabled') && changes.enabled === false;
  const pathChanged = Object.prototype.hasOwnProperty.call(changes, 'dirTemplate');
  if (turningOff || pathChanged) {
    confirmKind.value = turningOff ? 'disable' : 'path';
    confirmVisible.value = true;
    return;
  }
  doSave();
}

function confirmSave() {
  confirmVisible.value = false;
  doSave();
}

async function doSave() {
  if (saving.value) return;
  saving.value = true;
  saveError.value = '';
  try {
    const updated = await updateArchiveSettings(changedFields.value);
    applySettings(updated);
    deletions.notify({ prefix: 'Настройки файлового архива ', bold: 'сохранены' });
    emit('saved', updated);
  } catch (e) {
    saveError.value = e?.message || 'Не удалось сохранить настройки';
    deletions.notify({ prefix: 'Не удалось сохранить настройки: ', bold: saveError.value, type: 'error' });
  } finally {
    saving.value = false;
  }
}

let stopGuard = null;
onMounted(() => {
  load();
  stopGuard = registerDirtyTracker({
    isDirty: () => isDirty.value,
    getChanges: () => Object.entries(changedFields.value).map(([key, value]) => describeChange(key, value)),
    // Сохранение из глобального "покинуть страницу с изменениями" идёт напрямую,
    // без локального confirm про выключение/смену пути - тот уже подтверждён
    // решением пользователя уйти и сохранить.
    save: () => doSave(),
  });
});
onBeforeUnmount(() => {
  clearTimeout(previewTimer);
  stopGuard?.();
});
</script>

<style scoped>
.asf {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.asf__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.asf__section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.asf__section-title {
  margin: 0;
  font-size: 0.95em;
  font-weight: 600;
  color: var(--text);
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.field-hint {
  font-size: 0.78em;
  color: var(--text-muted);
  line-height: 1.4;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
  margin: 0;
}

.asf__full-preview {
  margin: 0;
  font-size: 0.85em;
  color: var(--text-muted);
  word-break: break-all;
}

.asf__full-preview code {
  background: var(--accent-tint);
  color: var(--accent-text);
  border-radius: 6px;
  padding: 2px 6px;
  font-family: 'Courier New', monospace;
}

.asf__preview-note {
  margin-left: 8px;
  font-style: italic;
}

.asf__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.asf__actions {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 767.98px) {
  .asf__grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .asf__grid {
    grid-template-columns: 1fr;
  }
}
</style>
