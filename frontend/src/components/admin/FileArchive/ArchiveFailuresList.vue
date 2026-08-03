<template>
  <section class="afl">
    <div class="afl__toolbar">
      <BaseDropdown
        :model-value="statusFilter"
        class="afl__status-select"
        :options="STATUS_OPTIONS"
        label-key="label"
        value-key="value"
        data-testid="afl-status-select"
        @update:model-value="onStatusChange"
      />
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="afl-retry-all"
        :disabled="items.length === 0 || retryingAll || retryingId !== null"
        @click="retryAll"
      >
        {{ retryingAll ? 'Повторяем...' : 'Повторить все на странице' }}
      </button>
    </div>

    <p
      v-if="error"
      class="form-error"
    >
      {{ error }}
    </p>

    <div
      v-else
      class="afl__table rt-table"
    >
      <div class="afl__row afl__row--head rt-head-row">
        <span class="afl__cell afl__cell--app">Заявка</span>
        <span class="afl__cell afl__cell--status">Статус</span>
        <span class="afl__cell afl__cell--error">Ошибка</span>
        <span class="afl__cell afl__cell--updated">Обновлено</span>
        <span class="afl__cell afl__cell--actions" />
      </div>

      <div
        v-for="item in items"
        :key="item.id"
        class="afl__row rt-row"
      >
        <span
          class="afl__cell afl__cell--app"
          data-label="Заявка"
        >№{{ item.application_id }}</span>
        <span
          class="afl__cell afl__cell--status"
          data-label="Статус"
        >
          <StatusBadge :status="statusLabel(item.status)" />
        </span>
        <span
          class="afl__cell afl__cell--error"
          data-label="Ошибка"
        >{{ item.last_error || '—' }}</span>
        <span
          class="afl__cell afl__cell--updated muted"
          data-label="Обновлено"
        >{{ formatDateTime(item.updated_at) }}</span>
        <span class="afl__cell afl__cell--actions">
          <button
            type="button"
            class="lk-button lk-button--ghost afl__retry-btn"
            data-testid="afl-retry-row"
            :disabled="retryingId === item.application_id || retryingAll"
            @click="retryOne(item)"
          >
            {{ retryingId === item.application_id ? 'Повторяем...' : 'Повторить' }}
          </button>
        </span>
      </div>

      <p
        v-if="!loading && items.length === 0"
        class="afl__empty"
      >
        Строк с этим статусом нет.
      </p>
    </div>

    <Pager
      class="afl__pager"
      :page="page"
      :total-pages="totalPages"
      :total="total"
      :loading="loading"
      page-prefix="Стр. "
      @update:page="goToPage"
    />
  </section>
</template>

<script setup>
/**
 * Вкладка «Ошибки» файлового архива (#1615, срез C4): реестр строк с проблемным
 * статусом (failed/no_template/blocked/orphan - pending/ok/skipped не считаются
 * ошибкой) с постраничным списком и ручным повтором. Повтор строки пересобирает
 * ВСЮ заявку (ExportApplication работает на заявку целиком, не на строку) -
 * повтор нескольких failed-строк одной заявки на странице избыточен, но не
 * ломает ничего: второй вызов просто повторяет ту же работу.
 */
import { ref, computed, onMounted } from 'vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import Pager from '@/components/ui/Pager.vue';
import { listArchiveItems, reexportApplication } from '@/api/fileArchive';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

const PER_PAGE = 20;

const STATUS_OPTIONS = [
  { value: 'failed', label: 'Ошибка выгрузки' },
  { value: 'no_template', label: 'Нет шаблона' },
  { value: 'blocked', label: 'Заблокировано' },
  { value: 'orphan', label: 'Вложение удалено' },
];
const STATUS_LABELS = Object.fromEntries(STATUS_OPTIONS.map((o) => [o.value, o.label]));

const deletions = useDeletionsStore();

const statusFilter = ref('failed');
const items = ref([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const error = ref('');
const retryingId = ref(null);
const retryingAll = ref(false);

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PER_PAGE)));

function statusLabel(status) {
  return STATUS_LABELS[status] || status;
}

async function load() {
  loading.value = true;
  error.value = '';
  try {
    const { items: rows, meta } = await listArchiveItems({
      status: statusFilter.value, page: page.value, perPage: PER_PAGE,
    });
    items.value = rows;
    total.value = meta.total;
  } catch (e) {
    error.value = e?.message || 'Не удалось загрузить список файлового архива';
    deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'список ошибок архива', type: 'error' });
  } finally {
    loading.value = false;
  }
}

function onStatusChange(value) {
  statusFilter.value = value;
  page.value = 1;
  load();
}

function goToPage(next) {
  if (next < 1 || next > totalPages.value) return;
  page.value = next;
  load();
}

async function retryOne(item) {
  retryingId.value = item.application_id;
  try {
    await reexportApplication(item.application_id);
    deletions.notify({ prefix: 'Заявка ', bold: `№${item.application_id}`, suffix: ' поставлена на пересоздание' });
    await load();
  } catch (e) {
    deletions.notify({
      prefix: 'Не удалось пересоздать файлы заявки ', bold: `№${item.application_id}`, suffix: `: ${e?.message || ''}`, type: 'error',
    });
  } finally {
    retryingId.value = null;
  }
}

async function retryAll() {
  if (retryingAll.value || items.value.length === 0) return;
  retryingAll.value = true;
  const ids = [...new Set(items.value.map((i) => i.application_id))];
  let ok = 0;
  let failed = 0;
  // Последовательно, не Promise.all: пересборка целых заявок не должна бить
  // писатель архива параллельным потоком запросов на десятках строк страницы.
  for (const id of ids) {
    try {
      await reexportApplication(id);
      ok += 1;
    } catch {
      failed += 1;
    }
  }
  retryingAll.value = false;
  await load();
  if (failed === 0) {
    deletions.notify({ prefix: 'Пересоздание запущено для ', bold: `${ok} заявок` });
  } else {
    deletions.notify({
      prefix: 'Пересоздано ', bold: `${ok} из ${ids.length}`, suffix: ` заявок, ошибок: ${failed}`, type: 'warning',
    });
  }
}

onMounted(load);
defineExpose({ refresh: load });
</script>

<style scoped>
.afl {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.afl__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.afl__status-select {
  max-width: 260px;
}

.afl__row {
  display: grid;
  grid-template-columns: 90px 160px minmax(0, 1fr) 140px 120px;
  gap: 12px;
  align-items: center;
  padding: 10px 4px;
}

.afl__row--head {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
}

.afl__row:not(.afl__row--head) {
  border-bottom: 1px solid var(--border);
  font-size: 13px;
  color: var(--text);
}

.afl__cell--error {
  overflow-wrap: anywhere;
}

.afl__cell--actions {
  display: flex;
  justify-content: flex-end;
}

.muted {
  color: var(--text-muted);
}

.afl__empty {
  padding: 30px 0;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
}

.afl__pager {
  justify-content: flex-end;
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
  margin: 0;
}

/* Планшет: колонка ошибки получает остаток от пяти фиксированных ширин, и на
   768 ей доставалось 102px - путь к шаблону рассыпался в узкий столбик на
   двенадцать строк. Узкие колонки поджимаются, ошибке остаётся вдвое больше. */
@media (max-width: 1023.98px) {
  .afl__row {
    grid-template-columns: 74px 130px minmax(0, 1fr) 110px 112px;
    gap: 8px;
  }
}

@media (max-width: 767.98px) {
  .afl__toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .afl__status-select {
    max-width: 100%;
  }

  /* Тач-таргет даём самой кнопке дропдауна: min-height обёртки её не
     растягивает, у неё свой 30px (образец - AccessDenialsLog). */
  .afl__status-select :deep(.base-dropdown__button) {
    min-height: 44px;
  }

  .afl__toolbar .lk-button {
    min-height: 44px;
  }

  /* В карточке строка становится колонкой, и align-items:center из десктопной
     раскладки центрирует ячейки по горизонтали: колонка действий (она без
     data-label, ширину от card-правил не получает) сжималась до кнопки и висела
     посреди карточки. */
  .afl__row {
    align-items: stretch;
  }

  /* Текст ошибки - единственная многострочная ячейка карточки: прижатый вправо
     он рвётся лесенкой, поэтому подпись сверху, значение под ней слева.
     Селектор из трёх классов - иначе проигрывает card-правилу
     responsive-tables.css (.rt-table .rt-row > [data-label], те же свойства). */
  .afl__table .afl__row > .afl__cell--error {
    flex-direction: column;
    align-items: flex-start;
    text-align: left;
  }

  .afl__cell--actions {
    padding-top: 6px;
  }

  .afl__retry-btn {
    width: 100%;
    min-height: 44px;
  }

  .afl__pager {
    justify-content: center;
  }

  .afl__pager :deep(.lk-button) {
    min-height: 44px;
  }
}
</style>
