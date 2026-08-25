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
      <span
        v-if="pendingTotal > 0"
        class="afl__queue"
        data-testid="afl-queue-count"
      >
        В очереди: {{ pendingTotal }}
      </span>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="afl-retry-all"
        :disabled="retryableItems.length === 0 || retryingAll || retryingId !== null"
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
        <span class="afl__cell afl__cell--error">Что записано</span>
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
          :title="`Идентификатор в системе: ${item.application_id}`"
        >{{ applicationLabel(item) }}</span>
        <span
          class="afl__cell afl__cell--status"
          data-label="Статус"
        >
          <StatusBadge :status="statusLabel(item.status)" />
        </span>
        <span
          class="afl__cell afl__cell--error"
          data-label="Что записано"
        >
          <span class="afl__what">{{ whatLabel(item) }}</span>
          <span
            v-if="reasonLabel(item)"
            class="afl__reason"
          >{{ reasonLabel(item) }}</span>
        </span>
        <span
          class="afl__cell afl__cell--updated muted"
          data-label="Обновлено"
        >
          {{ formatDateTime(item.updated_at) }}
          <span
            v-if="nextAttemptLabel(item)"
            class="afl__next-attempt"
          >{{ nextAttemptLabel(item) }}</span>
        </span>
        <span class="afl__cell afl__cell--actions">
          <button
            v-if="canRetry(item)"
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
        {{ statusFilter ? 'Записей с этим состоянием нет.' : 'В архиве пока нет ни одной записи.' }}
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
 * Вкладка «Лента» файлового архива (#1615 C4, расширена в followup S5): все
 * записи реестра с фильтром по состоянию, счётчиком очереди и ручным повтором.
 *
 * Раньше вкладка показывала только проблемные состояния и называлась «Ошибки».
 * Из-за этого про очередь узнать было негде: строка, ждущая записи, не попадала
 * ни в один список, и пустая вкладка одинаково означала и «всё хорошо», и «мы
 * ничего не знаем». Лента отвечает на вопрос «что сейчас происходит», фильтр
 * оставляет прежний разбор ошибок.
 *
 * Повтор строки пересобирает ВСЮ заявку (ExportApplication работает на заявку
 * целиком, не на строку) - повтор нескольких строк одной заявки на странице
 * избыточен, но не ломает ничего: второй вызов повторяет ту же работу.
 */
import { ref, computed, onMounted, onUnmounted } from 'vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import Pager from '@/components/ui/Pager.vue';
import { listArchiveItems, reexportApplication } from '@/api/fileArchive';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

const PER_PAGE = 20;

// Перечень повторяет статусы реестра целиком: вкладка показывает ленту всего, а
// не только ошибки - иначе про очередь («уже пишется или ещё ждёт») узнать было
// негде, и молчание вкладки читалось как «работы нет».
const STATUS_OPTIONS = [
  { value: '', label: 'Все записи' },
  { value: 'pending', label: 'Ждёт очереди' },
  { value: 'ok', label: 'Записан' },
  { value: 'failed', label: 'Ошибка выгрузки' },
  { value: 'no_template', label: 'Нет шаблона' },
  { value: 'blocked', label: 'Остановлено местом' },
  { value: 'skipped', label: 'Пропущено' },
  { value: 'orphan', label: 'Вложение удалено' },
];
const STATUS_LABELS = Object.fromEntries(
  STATUS_OPTIONS.filter((o) => o.value).map((o) => [o.value, o.label]),
);

// Статусы, из которых строка сама уже не выберется: их и предлагаем повторить.
// Для ждущих очереди кнопка была бы обманом - работа и так запланирована.
const RETRYABLE = new Set(['failed', 'no_template', 'blocked', 'orphan']);

// Пока в реестре есть незавершённые строки, лента обновляется сама: разбор
// очереди идёт фоном, и без этого администратор смотрит на застывший список,
// не понимая, движется ли что-то.
const AUTO_REFRESH_MS = 15000;

const deletions = useDeletionsStore();

const statusFilter = ref('');
const items = ref([]);
const total = ref(0);
const pendingTotal = ref(0);
const page = ref(1);
const loading = ref(false);
const error = ref('');
const retryingId = ref(null);
const retryingAll = ref(false);
let refreshTimer = null;

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PER_PAGE)));

const canRetry = (item) => RETRYABLE.has(item.status);
const retryableItems = computed(() => items.value.filter(canRetry));

function statusLabel(status) {
  return STATUS_LABELS[status] || status;
}

// Номер заявки в том виде, в каком его знает бюро. Внутренний идентификатор
// человеку ничего не говорит, поэтому он ушёл в подсказку: там он нужен при
// разборе с журналом сервера, но не на экране.
//
// Отсутствие поля и пустое значение - разные вещи, и путать их нельзя: поля нет
// у сервера, который ещё не обновился (фронт выкатывается раньше), а пустое
// значение означает, что заявки действительно не стало. Написать «Заявка
// удалена» в первом случае значит соврать про живую заявку.
function applicationLabel(item) {
  if (item.application_number === undefined) return `№${item.application_id}`;
  // Номер печатается как есть: бэк собирает его уже со знаком номера
  // (application_service.go), свой знак давал «№№ 20260808/027».
  return item.application_number || 'Заявка удалена';
}

// Что за файл в строке. У бланка это наименование вложения из справочника, у
// служебного описания - его назначение словами: имя «заявка.json» отвечает на
// вопрос «как называется файл», а читающему нужно «что это такое».
function whatLabel(item) {
  if (!item.attachment_id) return 'Описание заявки';
  if (item.attachment_name === undefined) return item.file_name || 'Бланк вложения';
  return item.attachment_name || 'Вложение удалено';
}

// Почему строка выглядит именно так. У записанной причины нет вовсе - файл лежит
// и вопросов не вызывает; у остальных состояний она и есть содержание строки.
const REASON_BY_STATUS = {
  no_template: 'бланк не настроен',
  pending: 'ждёт разбора очереди',
  blocked: 'запись остановлена нехваткой места',
  skipped: 'файл не изменился, перезапись не потребовалась',
  orphan: 'вложение удалено, файл остался на диске',
};

function reasonLabel(item) {
  if (item.status === 'ok') return '';
  return item.last_error || REASON_BY_STATUS[item.status] || '';
}

// Срок следующей попытки показываем только тем, кто её ждёт: пауза до повтора
// доходит до пяти минут (подметатель очереди), и без этой подписи строка выглядит
// зависшей.
function nextAttemptLabel(item) {
  if (!item.next_attempt_at || item.status === 'ok') return '';
  return `Повтор в ${formatDateTime(item.next_attempt_at).slice(-5)}`;
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
    await loadPendingCount();
    scheduleRefresh();
  } catch (e) {
    error.value = e?.message || 'Не удалось загрузить список файлового архива';
    deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'ленту файлового архива', type: 'error' });
  } finally {
    loading.value = false;
  }
}

// Счётчик очереди берём отдельным запросом на одну строку: он нужен независимо
// от того, какой фильтр выбран, а тянуть ради числа всю страницу расточительно.
async function loadPendingCount() {
  try {
    const { meta } = await listArchiveItems({ status: 'pending', page: 1, perPage: 1 });
    pendingTotal.value = meta.total;
  } catch {
    // Счётчик - подсказка, а не содержание вкладки: его провал не должен
    // подменять собой ленту сообщением об ошибке.
    pendingTotal.value = 0;
  }
}

function scheduleRefresh() {
  clearTimeout(refreshTimer);
  if (pendingTotal.value === 0) return;
  refreshTimer = setTimeout(load, AUTO_REFRESH_MS);
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
  if (retryingAll.value || retryableItems.value.length === 0) return;
  retryingAll.value = true;
  // Только те строки, которым повтор действительно нужен: на общей ленте рядом
  // лежат записанные и ждущие очереди, и пересоздавать их скопом бессмысленно.
  const ids = [...new Set(retryableItems.value.map((i) => i.application_id))];
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

// Таймер живёт дольше вкладки, если его не снять: уход со страницы оставил бы
// фоновый опрос архива навсегда.
onUnmounted(() => clearTimeout(refreshTimer));
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

/* Счётчик очереди прижат к фильтру, а не к кнопке: он про состояние архива,
   а не про действие, которое кнопка выполняет. */
.afl__queue {
  margin-right: auto;
  font-size: 13px;
  color: var(--text-muted);
}

.afl__next-attempt {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
}

/* Название файла и причина - разные вопросы («что это» и «почему так»), поэтому
   разными строками, а не одной склейкой через тире. */
.afl__what {
  display: block;
}

.afl__reason {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
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
