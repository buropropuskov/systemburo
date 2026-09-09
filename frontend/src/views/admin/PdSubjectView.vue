<template>
  <AdminPageShell>
    <section class="pds">
      <header class="page-header">
        <h2 class="page-title">
          Сведения о субъекте персональных данных
        </h2>
        <RefreshButton
          :loading="loading"
          @refresh="refresh"
        />
      </header>

      <p class="pds__hint">
        Что система хранит о человеке: записи реестра, участие в заявках, проходы и посты.
        Раздел отвечает на запросы государственных органов и обращения самих работников.
        Записи склеиваются по документу, а не по имени: однофамильцы существуют.
      </p>

      <form
        class="pds__search"
        @submit.prevent="search"
      >
        <input
          v-model="fio"
          class="lk-input pds__search-input"
          type="text"
          placeholder="Фамилия Имя Отчество"
          data-testid="pds-fio"
        >
        <button
          class="lk-button lk-button--primary"
          type="submit"
          :disabled="loading"
        >
          Найти
        </button>
      </form>

      <div
        v-if="candidates.length"
        class="pds__candidates"
      >
        <h3 class="pds__section-title">
          Найденные записи
        </h3>
        <p class="pds__note">
          <template v-if="fuzzyFound">
            Точных совпадений нет, показаны похожие по написанию - проверьте, тот ли это
            человек: сведения о постороннем уйдут в ответ государственному органу.
          </template>
          <template v-else>
            Это не обязательно один человек. Сведения собираются по записи с документом.
          </template>
        </p>
        <ul class="pds__list">
          <li
            v-for="c in candidates"
            :key="`${c.registry_id}-${c.employee_id}`"
            class="pds__item"
            :class="{ 'pds__item--active': isSelected(c) }"
          >
            <span class="pds__item-name">{{ c.full_name }}</span>
            <span
              v-if="c.fuzzy"
              class="pds__item-fuzzy"
            >похожее написание</span>
            <span class="pds__item-marks">{{ marksOf(c) }}</span>
            <span class="pds__item-source">{{ whereFound(c) }}</span>
            <button
              v-if="c.has_document"
              class="lk-button lk-button--secondary lk-button--sm"
              type="button"
              data-testid="pds-collect"
              @click="collect(c)"
            >
              Собрать сведения
            </button>
            <span
              v-else
              class="pds__item-muted"
            >нет документа</span>
          </li>
        </ul>
      </div>

      <p
        v-if="loading && !report"
        class="pds__note"
      >
        Собираем сведения...
      </p>

      <div
        v-if="report"
        ref="reportBlock"
        class="pds__report"
      >
        <div class="pds__report-head">
          <h3 class="pds__section-title">
            Сведения: {{ report.total }} записей
          </h3>
          <button
            v-if="canExport"
            class="lk-button lk-button--primary"
            type="button"
            data-testid="pds-export"
            @click="exportOpen = true"
          >
            Выгрузить справку
          </button>
        </div>
        <p class="pds__note">
          Основание обработки: {{ report.basis }}
        </p>

        <div
          v-for="s in report.sections"
          :key="s.title"
          class="pds__block"
        >
          <h4 class="pds__block-title">
            {{ s.title }} <span class="pds__count">{{ rowsOf(s).length }}</span>
          </h4>
          <div class="pds__table-wrap">
            <table
              v-if="rowsOf(s).length"
              class="pds__table"
            >
              <thead>
                <tr>
                  <th
                    v-for="h in s.headers"
                    :key="h"
                  >
                    {{ h }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, i) in rowsOf(s)"
                  :key="i"
                >
                  <td
                    v-for="(cell, j) in row"
                    :key="j"
                  >
                    {{ cell }}
                  </td>
                </tr>
              </tbody>
            </table>
            <p
              v-else
              class="pds__empty"
            >
              Записей нет
            </p>
          </div>
        </div>
      </div>

      <div
        v-if="disclosures.length"
        class="pds__disclosures"
      >
        <h3 class="pds__section-title">
          Журнал выдач
        </h3>
        <ul class="pds__list">
          <li
            v-for="d in disclosures"
            :key="d.id"
            class="pds__disclosure"
          >
            <span class="pds__item-name">{{ d.subject_name || 'без имени' }}</span>
            <span>{{ d.recipient }}</span>
            <span class="pds__item-source">{{ d.request_ref }}</span>
          </li>
        </ul>
      </div>

        <PdSubjectExportModal
          :show="exportOpen"
          :loading="exporting"
          @close="exportOpen = false"
          @submit="runExport"
        />
    </section>
  </AdminPageShell>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import PdSubjectExportModal from '@/components/admin/PdSubjectExportModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { downloadBlob } from '@/utils/reportDownload';
import {
  exportSubjectReport,
  fetchSubjectDisclosures,
  fetchSubjectReport,
  findSubjectCandidates,
} from '@/api/pdSubject';

const deletions = useDeletionsStore();
const permissions = usePermissionsStore();

const fio = ref('');
const loading = ref(false);
const exporting = ref(false);
const exportOpen = ref(false);
const candidates = ref([]);
const report = ref(null);
const disclosures = ref([]);
const selected = ref({ registryId: 0, employeeId: 0 });
const reportBlock = ref(null);

const fuzzyFound = computed(() => candidates.value.some((c) => c.fuzzy));
const canExport = computed(() => permissions.hasPermission('action.pd_subject.export'));

// Раздел без строк приходит с rows: null - Go отдаёт пустой срез как null. Обращение
// к null.length роняет рендер целиком, и человека выбрасывает со страницы.
function rowsOf(section) {
  return section.rows || [];
}

async function search() {
  if (fio.value.trim().split(/\s+/).length < 2) {
    deletions.notify({ bold: 'Укажите хотя бы фамилию и имя', type: 'error' });
    return;
  }
  loading.value = true;
  try {
    candidates.value = await findSubjectCandidates(fio.value);
    report.value = null;
    selected.value = { registryId: 0, employeeId: 0 };
    if (!candidates.value.length) deletions.notify({ bold: 'Записей с таким именем не найдено' });
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'выполнить поиск', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    loading.value = false;
  }
}

/**
 * Чем этот человек отличается от однофамильцев: организация, должность и хвост
 * документа. Без них три записи с одним ФИО выглядят одинаково, и выбрать не из чего.
 */
function marksOf(c) {
  const parts = [];
  if (c.document_tail) parts.push(`документ …${c.document_tail}`);
  if (c.organization) parts.push(c.organization);
  if (c.position) parts.push(c.position);
  return parts.join(' · ');
}

/** Где человек встречается: строки склеены по документу, поэтому здесь счётчики. */
function whereFound(c) {
  const parts = [];
  if (c.registry_rows) parts.push(`в реестре: ${c.registry_rows}`);
  if (c.application_rows) parts.push(`в заявках: ${c.application_rows}`);
  return parts.join(', ') || 'нет записей';
}

function isSelected(c) {
  return (c.registry_id && c.registry_id === selected.value.registryId)
    || (c.employee_id && c.employee_id === selected.value.employeeId);
}

async function collect(candidate) {
  loading.value = true;
  try {
    // У человека без записи реестра собираем от строки заявки: иначе по нему нельзя
    // ответить государственному органу вовсе.
    selected.value = { registryId: candidate.registry_id, employeeId: candidate.employee_id };
    report.value = await fetchSubjectReport(selected.value);
    disclosures.value = await fetchSubjectDisclosures(selected.value);
    // Сведения появляются НИЖЕ списка найденных записей, за краем экрана: без
    // прокрутки клик выглядит как «ничего не произошло» (претензия при ручной
    // проверке). Прокручивает ближайший скроллящийся предок - обёртка админской
    // страницы, а не окно.
    await nextTick();
    reportBlock.value?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'собрать сведения', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    loading.value = false;
  }
}

async function runExport(form) {
  exporting.value = true;
  try {
    const { blob, filename } = await exportSubjectReport({
      ...form,
      registry_id: selected.value.registryId,
      employee_id: selected.value.employeeId,
    });
    downloadBlob(blob, filename);
    exportOpen.value = false;
    // Журнал перечитываем сразу: выдача уже состоялась, и человек должен увидеть её
    // запись, а не гадать, попала ли она туда.
    disclosures.value = await fetchSubjectDisclosures(selected.value);
    deletions.notify({ bold: 'Справка выгружена', suffix: ', выдача внесена в журнал' });
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'выгрузить справку', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    exporting.value = false;
  }
}

function refresh() {
  const { registryId, employeeId } = selected.value;
  if (registryId || employeeId) {
    collect({ registry_id: registryId, employee_id: employeeId });
    return;
  }
  if (fio.value.trim()) search();
}
</script>

<style scoped>
/* Раскладка страницы у админских экранов своя, а не из общего layout: те же
   .page-header/.page-title объявлены локально в PdAuditLog и соседях. Без них шапка
   и блоки слипаются - экран выглядит сломанным. */
.pds {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.page-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--color-text, #000);
}

.pds__hint,
.pds__note {
  margin: 0 0 16px;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.5;
}

.pds__search {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
}

.pds__search-input {
  max-width: 360px;
}

.pds__section-title {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
}

.pds__list {
  margin: 0 0 24px;
  padding: 0;
  list-style: none;
}

.pds__item,
.pds__disclosure {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  margin-bottom: 8px;
  background: var(--surface);
}

.pds__item--active {
  border-color: var(--accent);
}

.pds__item-name {
  font-weight: 500;
  min-width: 220px;
}

.pds__item-fuzzy {
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  background: var(--accent-tint);
  color: var(--accent-text);
  font-size: 12px;
  white-space: nowrap;
}

.pds__item-marks {
  color: var(--text);
  font-size: 13px;
  flex: 1;
}

.pds__item-source,
.pds__item-muted {
  color: var(--text-muted);
  font-size: 13px;
}

.pds__item-muted {
  margin-left: auto;
}

.pds__report-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pds__block {
  margin-bottom: 20px;
}

.pds__block-title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
}

.pds__count {
  color: var(--text-muted);
  font-weight: 400;
}

/* Таблицы справки бывают широкими (двенадцать колонок в сведениях), поэтому катаются
   внутри своего контейнера: страница по горизонтали не едет. */
.pds__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
}

.pds__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.pds__table th,
.pds__table td {
  padding: 8px 10px;
  text-align: left;
  white-space: nowrap;
  border-bottom: 1px solid var(--border);
}

.pds__table th {
  background: var(--surface-2);
  font-weight: 600;
}

.pds__empty {
  margin: 0;
  padding: 12px;
  color: var(--text-muted);
  font-size: 13px;
}
</style>
