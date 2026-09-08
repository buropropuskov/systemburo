<template>
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
        Это не обязательно один человек. Сведения собираются по записи с документом.
      </p>
      <ul class="pds__list">
        <li
          v-for="c in candidates"
          :key="`${c.source}-${c.id}`"
          class="pds__item"
          :class="{ 'pds__item--active': c.id === selectedId && c.source === 'реестр' }"
        >
          <span class="pds__item-name">{{ c.full_name }}</span>
          <span class="pds__item-source">{{ c.source }}</span>
          <button
            v-if="c.source === 'реестр' && c.has_document"
            class="lk-button lk-button--secondary lk-button--sm"
            type="button"
            data-testid="pds-collect"
            @click="collect(c.id)"
          >
            Собрать сведения
          </button>
          <span
            v-else
            class="pds__item-muted"
          >{{ c.has_document ? 'откройте запись реестра' : 'нет документа' }}</span>
        </li>
      </ul>
    </div>

    <div
      v-if="report"
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
          {{ s.title }} <span class="pds__count">{{ s.rows.length }}</span>
        </h4>
        <div class="pds__table-wrap">
          <table
            v-if="s.rows.length"
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
                v-for="(row, i) in s.rows"
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
</template>

<script setup>
import { computed, ref } from 'vue';
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
const selectedId = ref(0);

const canExport = computed(() => permissions.hasPermission('action.pd_subject.export'));

async function search() {
  if (fio.value.trim().split(/\s+/).length < 2) {
    deletions.notify({ bold: 'Укажите хотя бы фамилию и имя', type: 'error' });
    return;
  }
  loading.value = true;
  try {
    candidates.value = await findSubjectCandidates(fio.value);
    report.value = null;
    selectedId.value = 0;
    if (!candidates.value.length) deletions.notify({ bold: 'Записей с таким именем не найдено' });
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'выполнить поиск', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    loading.value = false;
  }
}

async function collect(registryId) {
  loading.value = true;
  try {
    selectedId.value = registryId;
    report.value = await fetchSubjectReport(registryId);
    disclosures.value = await fetchSubjectDisclosures(registryId);
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'собрать сведения', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    loading.value = false;
  }
}

async function runExport(form) {
  exporting.value = true;
  try {
    const { blob, filename } = await exportSubjectReport({ ...form, registry_id: selectedId.value });
    downloadBlob(blob, filename);
    exportOpen.value = false;
    // Журнал перечитываем сразу: выдача уже состоялась, и человек должен увидеть её
    // запись, а не гадать, попала ли она туда.
    disclosures.value = await fetchSubjectDisclosures(selectedId.value);
    deletions.notify({ bold: 'Справка выгружена', suffix: ', выдача внесена в журнал' });
  } catch (e) {
    deletions.notify({ prefix: 'Не удалось ', bold: 'выгрузить справку', suffix: `: ${e.message}`, type: 'error' });
  } finally {
    exporting.value = false;
  }
}

function refresh() {
  if (selectedId.value) collect(selectedId.value);
  else if (fio.value.trim()) search();
}
</script>

<style scoped>
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
