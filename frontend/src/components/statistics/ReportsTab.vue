<template>
  <div class="reports">
    <!-- Загрузка каталога -->
    <div
      v-if="catalogLoading"
      class="reports__state"
    >
      <LoaderSpinner />
      <span>Загружаем конструктор отчётов…</span>
    </div>

    <div
      v-else-if="catalogError"
      class="reports__state reports__state--error"
    >
      {{ catalogError }}
    </div>

    <template v-else-if="catalog">
      <div class="reports-layout">
        <!-- Сайдбар: готовые наборы + мои шаблоны -->
        <aside class="presets-col">
          <component
            :is="isNarrow ? 'button' : 'h3'"
            class="col-heading"
            :class="{ 'col-heading--toggle': isNarrow }"
            :type="isNarrow ? 'button' : null"
            :aria-expanded="isNarrow ? String(presetsOpen) : null"
            @click="togglePresets"
          >
            Готовые наборы
            <span
              v-if="isNarrow && activePresetTitle"
              class="col-heading__sum"
            >{{ activePresetTitle }}</span>
            <span
              v-if="isNarrow"
              class="col-heading__caret"
              :class="{ 'col-heading__caret--open': presetsOpen }"
              aria-hidden="true"
            />
          </component>
          <ReportGallery
            v-show="presetsOpen"
            :catalog="catalog"
            :active-id="activePresetId"
            compact
            @apply="onApplyPreset"
          />

          <component
            :is="isNarrow ? 'button' : 'h3'"
            class="col-heading col-heading--mt"
            :class="{ 'col-heading--toggle': isNarrow }"
            :type="isNarrow ? 'button' : null"
            :aria-expanded="isNarrow ? String(templatesOpen) : null"
            @click="toggleTemplates"
          >
            Мои шаблоны
            <span
              v-if="isNarrow"
              class="col-heading__caret"
              :class="{ 'col-heading__caret--open': templatesOpen }"
              aria-hidden="true"
            />
          </component>

          <div v-show="templatesOpen">
            <!-- Сохранить текущий набор как шаблон -->
            <div
              v-if="savingMode"
              class="tpl-save-form"
            >
              <input
                v-model="newTemplateName"
                class="lk-input"
                placeholder="Название шаблона"
                maxlength="200"
                @keyup.enter="confirmSaveTemplate"
              >
              <div class="tpl-save-actions">
                <button
                  type="button"
                  class="lk-button lk-button--primary"
                  :disabled="!newTemplateName.trim() || savingTemplate"
                  @click="confirmSaveTemplate"
                >
                  {{ savingTemplate ? 'Сохраняем…' : 'Сохранить' }}
                </button>
                <button
                  type="button"
                  class="lk-button lk-button--ghost"
                  @click="savingMode = false"
                >
                  Отмена
                </button>
              </div>
            </div>
            <button
              v-else
              type="button"
              class="lk-button lk-button--ghost tpl-save-btn"
              :disabled="!canSaveTemplate"
              @click="startSaveTemplate"
            >
              + Сохранить текущий
            </button>

            <div
              v-if="templatesLoading"
              class="template-placeholder"
            >
              Загрузка шаблонов…
            </div>
            <div
              v-else-if="!myTemplates.length"
              class="template-placeholder"
            >
              Сохранённых наборов пока нет. Соберите отчёт в мастере и нажмите «Сохранить текущий».
            </div>
            <ul
              v-else
              class="tpl-list"
            >
              <li
                v-for="tpl in myTemplates"
                :key="tpl.id"
                class="tpl-item"
              >
                <button
                  type="button"
                  class="tpl-apply"
                  @click="applyTemplate(tpl)"
                >
                  <span class="tpl-name">{{ tpl.name }}</span>
                  <span
                    v-if="tpl.description"
                    class="tpl-desc"
                  >{{ tpl.description }}</span>
                </button>
                <button
                  type="button"
                  class="tpl-del"
                  :title="`Удалить шаблон ${tpl.name}`"
                  :aria-label="`Удалить шаблон ${tpl.name}`"
                  @click="removeTemplate(tpl)"
                >
                  ×
                </button>
              </li>
            </ul>
          </div>

        </aside>

        <!-- Мастер -->
        <section class="wizard">
          <ReportBuilder
            ref="builderRef"
            :catalog="catalog"
            :period="period"
            :loading="running"
            :preset="presetPayload"
            @run="onRun"
            @change="onBuilderChange"
          />
        </section>
      </div>

      <!-- Результат (полная ширина под мастером). До первого построения не
           рендерим — пустой блок создавал бы лишнюю пустоту во вкладке. -->
      <ReportResult
        v-if="result || running || runError"
        ref="resultRef"
        :result="result"
        :loading="running"
        :error="runError"
        :meta="exportMeta"
        :limit="resultLimit"
        :empty-period-label="showExpandHint ? lastPeriodLabel : ''"
        @export-error="onExportError"
        @expand-period="expandPeriod"
      />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue';
import ReportBuilder from './ReportBuilder.vue';
import ReportResult from './ReportResult.vue';
import ReportGallery from './ReportGallery.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import {
  getReportCatalog, runReport,
  getReportTemplates, saveReportTemplate, deleteReportTemplate,
} from '@/api/statistics';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { getMe } from '@/api/auth';
import { REPORT_PRESETS } from './reportPresets';
import { formatDateRu } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

const props = defineProps({
  from: { type: String, default: '' },
  to: { type: String, default: '' },
});

const period = computed(() => ({ from: props.from, to: props.to }));

// На телефоне колонка наборов встаёт НАД конструктором и занимает больше экрана:
// до «Тип отчёта» приходилось прокручивать весь каталог и блок шаблонов (#2314).
const { isNarrow } = useNarrowScreen();
const presetsOpen = ref(true);
const templatesOpen = ref(true);

watch(isNarrow, (narrow) => {
  presetsOpen.value = !narrow;
  templatesOpen.value = !narrow;
}, { immediate: true });

// Свернуть можно только на телефоне: на десктопе колонка стоит сбоку и не мешает.
function togglePresets() {
  if (isNarrow.value) presetsOpen.value = !presetsOpen.value;
}

function toggleTemplates() {
  if (isNarrow.value) templatesOpen.value = !templatesOpen.value;
}

const activePresetTitle = computed(
  () => REPORT_PRESETS.find((p) => p.id === activePresetId.value)?.title || '',
);

const builderRef = ref(null);
const resultRef = ref(null);

const catalog = ref(null);
const catalogLoading = ref(true);
const catalogError = ref('');

const result = ref(null);
const running = ref(false);
const runError = ref('');
// Подпись выгрузки: период последнего запроса, имя набора и кто выгрузил. Без них
// PDF уходил заказчику озаглавленным «Отчёт по аналитике» и без следов происхождения.
const exportMeta = ref({});
const author = ref('');
// Лимит последнего запроса — по нему результат понимает, что упёрся в потолок.
const resultLimit = ref(0);

// Пресет из галереи: новый объект на каждый клик (даже по той же карточке),
// чтобы watch в ReportBuilder сработал повторно и перезаполнил конструктор.
const presetPayload = ref(null);
const activePresetId = ref('');

// Снимок состояния мастера: нужен для гейта «можно сохранить шаблон» и самого
// сохранения (берём config). Остальные поля снапшота от ReportBuilder храним как
// есть, но здесь читаем только mode/metric/entity/config.
const builderState = ref({ mode: 'aggregate', metric: '', entity: '', config: null });

// Личные шаблоны пользователя (G2). Системные пресеты остаются в галерее «Готовые
// наборы» (reportPresets), здесь — только сохранённые наборы пользователя.
const myTemplates = ref([]);
const templatesLoading = ref(true);
const savingMode = ref(false);
const savingTemplate = ref(false);
const newTemplateName = ref('');

const canSaveTemplate = computed(() => {
  const s = builderState.value;
  return s.mode === 'list' ? Boolean(s.entity) : Boolean(s.metric);
});

async function loadTemplates() {
  templatesLoading.value = true;
  try {
    const all = await getReportTemplates();
    // Только свои личные. Системные пресеты живут в галерее; чужие расшаренные
    // (is_shared) сюда не берём — прав на их удаление нет, кнопка × дала бы 403.
    myTemplates.value = (all || []).filter((t) => !t.is_system && !t.is_shared);
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'загрузить шаблоны', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  } finally {
    templatesLoading.value = false;
  }
}

function startSaveTemplate() {
  newTemplateName.value = '';
  savingMode.value = true;
}

async function confirmSaveTemplate() {
  const name = newTemplateName.value.trim();
  if (!name || !builderState.value.config) return;
  savingTemplate.value = true;
  try {
    await saveReportTemplate({ name, config: builderState.value.config });
    savingMode.value = false;
    await loadTemplates();
    useDeletionsStore().notify({ prefix: 'Шаблон ', bold: name, suffix: ' сохранён', type: 'success' });
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'сохранить шаблон', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  } finally {
    savingTemplate.value = false;
  }
}

function applyTemplate(tpl) {
  activePresetId.value = `tpl-${tpl.id}`;
  presetPayload.value = { ...tpl.config };
}

async function removeTemplate(tpl) {
  const ok = await useUiStore().confirm({
    title: 'Удалить шаблон?',
    message: `Шаблон «${tpl.name}» будет удалён без возможности восстановить.`,
    confirmText: 'Удалить',
    cancelText: 'Отмена',
    danger: true,
  });
  if (!ok) return;
  try {
    await deleteReportTemplate(tpl.id);
    await loadTemplates();
    useDeletionsStore().notify({ prefix: 'Шаблон ', bold: tpl.name, suffix: ' удалён', type: 'success' });
  } catch (e) {
    useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'удалить шаблон', suffix: e?.message ? `: ${e.message}` : '', type: 'error' });
  }
}

function onBuilderChange(snapshot) {
  builderState.value = snapshot;
}

// Ошибку выгрузки показываем тостом, не подменяя :error результата (иначе таблица
// отчёта скрылась бы за текстом ошибки).
function onExportError(message) {
  useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'выгрузить отчёт в Excel', suffix: message ? `: ${message}` : '', type: 'error' });
}

function onApplyPreset(preset) {
  activePresetId.value = preset.id;
  presetPayload.value = { ...preset.form };
}

onMounted(async () => {
  loadTemplates();
  // Подпись «Сформировал» в выгрузке. Молчаливый провал допустим: без имени шапка
  // просто скажет «Пользователь», ронять из-за этого построение отчёта незачем.
  getMe()
    .then((me) => { author.value = [me?.last_name, me?.first_name].filter(Boolean).join(' ') || me?.username || ''; })
    .catch(() => { author.value = ''; });
  try {
    catalog.value = await getReportCatalog();
  } catch (e) {
    catalogError.value = e?.message || 'Не удалось загрузить каталог отчётов';
  } finally {
    catalogLoading.value = false;
  }
});

// Быстрое переключение пресетов запускает несколько runReport параллельно;
// токен последовательности гарантирует, что результат покажет только последний
// запрос (иначе медленный ответ предыдущего пресета затёр бы актуальный).
let runSeq = 0;
// Имя набора для шапки выгрузки: у готового набора берём его название, у собранного
// вручную - краткое описание разреза из каталога.
function buildExportTitle(request) {
  const preset = REPORT_PRESETS.find((p) => p.id === activePresetId.value);
  if (preset) return preset.title;
  if (request.mode === 'list') {
    const entity = (catalog.value?.list_entities || []).find((e) => e.key === request.entity);
    return entity ? `Выгрузка: ${entity.label}` : '';
  }
  const metrics = (catalog.value?.metrics || []).filter((m) => (request.metrics || []).includes(m.key));
  if (!metrics.length) return '';
  const dimension = (catalog.value?.dimensions || []).find((d) => d.key === request.dimension);
  const names = metrics.map((m) => m.label).join(', ');
  return dimension && dimension.key !== 'none' ? `${names} по разрезу «${dimension.label}»` : names;
}

async function onRun(request) {
  const seq = ++runSeq;
  running.value = true;
  runError.value = '';
  try {
    const r = await runReport(request);
    if (seq !== runSeq) return;
    result.value = r;
    resultLimit.value = Number(request.limit) || 0;
    const dr = (request.filters || []).find((f) => f.key === 'date_range');
    const { from = '', to = '' } = dr || {};
    exportMeta.value = {
      title: buildExportTitle(request),
      author: author.value,
      ...(dr ? { period: { from, to } } : {}),
    };
  } catch (e) {
    if (seq !== runSeq) return;
    result.value = null;
    resultLimit.value = 0;
    runError.value = e?.message || 'Не удалось построить отчёт. Проверьте параметры.';
  } finally {
    if (seq === runSeq) {
      running.value = false;
      scrollToResult();
    }
  }
}

// Результат пуст: у выгрузки строк это её собственные строки, у сводки — нормализованные
// metric_rows (старые разрезы отдают плоский rows, отсюда двойная проверка).
const resultIsEmpty = computed(() => {
  const r = result.value;
  if (!r) return false;
  if (r.mode === 'list') return (r.rows?.length || 0) === 0;
  return (r.metric_rows || r.rows || []).length === 0;
});

const lastPeriodLabel = computed(() => {
  const { from = '', to = '' } = exportMeta.value.period || {};
  if (!from || !to) return '';
  return `${formatDateRu(from)} - ${formatDateRu(to)}`;
});

// Расширять период есть куда, только когда он вообще ограничен: у отчёта за весь
// период пустой результат означает, что данных нет совсем, и подсказка бесполезна.
const showExpandHint = computed(
  () => !running.value && !runError.value && resultIsEmpty.value && Boolean(lastPeriodLabel.value),
);

function expandPeriod() {
  builderRef.value?.expandPeriodToAll();
}

// Кнопка построения стоит над результатом, а таблица рендерится ниже сгиба: без
// прокрутки клик выглядит так, будто ничего не произошло.
async function scrollToResult() {
  await nextTick();
  const el = resultRef.value?.$el;
  if (!el?.scrollIntoView) return;
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches;
  el.scrollIntoView({ behavior: reduced ? 'auto' : 'smooth', block: 'start' });
}
</script>

<style scoped>
.reports {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.reports__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 240px;
  color: var(--color-text-muted);
}

.reports__state--error {
  color: var(--danger-text);
}


.reports-layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 16px;
  align-items: start;
}

.presets-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  position: sticky;
  top: 0;
  align-self: start;
  /* Непрозрачный фон обязателен: под липкой колонкой проезжает конструктор. */
  background: var(--surface);
}

.col-heading {
  margin: 4px 2px 8px;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

/* На телефоне заголовок колонки - кнопка-раскрывашка: каталог и шаблоны свёрнуты,
   чтобы первым на вкладке был конструктор, а не список наборов. */
.col-heading--toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.col-heading__sum {
  margin-left: auto;
  font-weight: 400;
  text-transform: none;
  letter-spacing: 0;
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.col-heading__caret {
  flex: none;
  width: 0;
  height: 0;
  border-left: 5px solid currentColor;
  border-top: 4px solid transparent;
  border-bottom: 4px solid transparent;
  transition: transform 0.18s ease;
}
.col-heading__caret--open {
  transform: rotate(90deg);
}
.col-heading--toggle:not(:has(.col-heading__sum)) .col-heading__caret {
  margin-left: auto;
}

.col-heading--mt {
  margin-top: 18px;
}

.template-placeholder {
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  padding: 14px;
  font-size: 12px;
  font-weight: 500;
  line-height: 1.45;
  color: var(--color-text-muted);
}

/* Шаблоны (G2) */
.tpl-save-btn {
  width: 100%;
  justify-content: center;
}

.tpl-save-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpl-save-actions {
  display: flex;
  gap: 8px;
}

.tpl-save-actions .lk-button {
  flex: 1;
  justify-content: center;
}

.tpl-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}

.tpl-item {
  display: flex;
  align-items: stretch;
  gap: 4px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
  transition: border-color 0.18s ease;
}

.tpl-item:hover {
  border-color: var(--accent);
}

.tpl-apply {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 9px 11px;
  border: none;
  background: transparent;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
}

.tpl-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tpl-desc {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tpl-del {
  flex-shrink: 0;
  width: 32px;
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.tpl-del:hover {
  background: var(--color-danger);
  color: var(--fill-text);
}

.wizard {
  min-width: 0;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px;
}

@media (max-width: 1180px) {
  .reports-layout {
    grid-template-columns: 240px 1fr;
  }
}

@media (max-width: 880px) {
  .presets-col {
    position: static;
  }

  .reports-layout {
    grid-template-columns: 1fr;
  }
}

/* Мобилка (#1097): плотнее по вертикали, у карточки-конструктора padding под
   узкий экран — 20px с обеих сторон съедали ширину полей мастера. */
@media (max-width: 768px) {
  .reports,
  .reports-layout {
    gap: 12px;
  }

  .wizard {
    padding: 16px 14px;
  }
}
</style>
