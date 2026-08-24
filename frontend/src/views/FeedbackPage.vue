<template>
  <AdminPageShell>
    <div
      class="feedback-container dashboard-card"
      data-testid="ob-admin-feedback"
    >
      <div class="management-header">
        <h3 class="management-title">
          Обратная связь
        </h3>
        <div class="header-controls">
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск по автору или тексту...'"
          />
          <RefreshButton
            :loading="loading"
            @refresh="refresh"
          />
        </div>
      </div>

      <div class="filter-strip">
        <FilterTabs
          v-model="activeFilter"
          :tabs="filterTabs"
        />
      </div>

      <div class="content-container">
        <!-- Список обращений (на мобилке rt-* сворачивает строки в карточки) -->
        <div class="list-section rt-table">
          <div class="list-header rt-head-row">
            <div
              class="header-col col-id"
              :class="{ 'active-sort': sortKey === 'id' }"
              @click="sortBy('id')"
            >
              #
            </div>
            <div
              class="header-col col-author"
              :class="{ 'active-sort': sortKey === 'author' }"
              @click="sortBy('author')"
            >
              Автор
            </div>
            <div class="header-col col-status">
              Статус
            </div>
            <div
              class="header-col col-date"
              :class="{ 'active-sort': sortKey === 'date' }"
              @click="sortBy('date')"
            >
              Дата
            </div>
            <div class="header-col col-flag" />
          </div>

          <div class="list-body">
            <SkeletonTransition :loading="loading">
              <template #skeleton>
                <SkeletonCard
                  v-for="i in 6"
                  :key="i"
                  :lines="2"
                  :show-badge="false"
                />
              </template>

              <div
                v-if="!filteredFeedbacks.length"
                class="empty-list"
                data-testid="fb-empty"
              >
                {{ emptyText }}
              </div>
              <div v-else>
                <div
                  v-for="f in filteredFeedbacks"
                  :key="f.id"
                  class="ticket-row rt-row"
                  data-testid="fb-row"
                  :class="{ unread: !f.is_read, selected: f.id === selectedId }"
                  @click="selectRow(f.id)"
                >
                  <!-- col-id без data-label: в карточке это строка-заголовок "· 14"
                       (dot+id вместе), rt-* её не трогает (правит только [data-label]). -->
                  <div class="cell col-id">
                    <span class="unread-dot" />
                    <span class="id-value">{{ f.id }}</span>
                  </div>
                  <div
                    class="cell col-author"
                    data-label="Автор"
                  >
                    <span
                      class="author"
                      :title="f.user_name"
                    >{{ f.user_name || 'Неизвестный пользователь' }}</span>
                  </div>
                  <div
                    class="cell col-status"
                    data-label="Статус"
                  >
                    <span
                      class="status-badge"
                      :class="f.status === STATUS.RESOLVED ? 'status-resolved' : 'status-open'"
                    >
                      {{ statusLabel(f.status) }}
                    </span>
                  </div>
                  <div
                    class="cell col-date cell-date"
                    data-label="Дата"
                  >
                    {{ formatShortDate(f.created_at) }}
                  </div>
                  <div
                    class="cell col-flag"
                    data-label="Важное"
                  >
                    <button
                      class="flag-btn"
                      :class="{ 'is-flagged': f.flagged }"
                      data-testid="fb-flag"
                      :title="f.flagged ? 'Снять флажок' : 'Пометить: важное / взять в работу'"
                      :disabled="flaggingId === f.id"
                      @click.stop="toggleFlag(f)"
                    >
                      <svg
                        viewBox="0 0 16 16"
                        width="14"
                        height="14"
                        aria-hidden="true"
                      >
                        <path
                          class="flag-pole"
                          d="M4.5 1.5V14.5"
                        />
                        <path
                          class="flag-banner"
                          d="M4.5 2.5H12l-2 2.5 2 2.5H4.5z"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            </SkeletonTransition>
          </div>

          <div class="list-footer">
            <span class="items-count">Всего: {{ filteredFeedbacks.length }}</span>
          </div>
        </div>

        <!-- Детали обращения -->
        <div class="detail-section">
          <div
            v-if="!selectedFeedback"
            class="no-selection"
          >
            Выберите обращение слева для просмотра и ответа
          </div>
          <div
            v-else
            class="detail-body"
            data-testid="fb-detail"
          >
            <div class="detail-header">
              <h3 class="detail-title">
                Обращение #{{ selectedFeedback.id }}
              </h3>
              <span
                class="status-badge"
                :class="selectedFeedback.status === STATUS.RESOLVED ? 'status-resolved' : 'status-open'"
              >
                {{ statusLabel(selectedFeedback.status) }}
              </span>
            </div>

            <div class="detail-meta">
              <span class="meta-author">{{ selectedFeedback.user_name || 'Неизвестный пользователь' }}</span>
              <span class="meta-dates">
                Создано: {{ formatDateTime(selectedFeedback.created_at) }}<template v-if="selectedFeedback.resolved_at"> · Решено: {{ formatDateTime(selectedFeedback.resolved_at) }}</template>
              </span>
            </div>

            <div class="message-card">
              {{ selectedFeedback.message }}
            </div>

            <div
              v-if="selectedFeedback.status === STATUS.RESOLVED && selectedFeedback.resolution_comment"
              class="answer-card"
              data-testid="fb-answer"
            >
              <div class="answer-label">
                Ответ
              </div>
              <div class="answer-text">
                {{ selectedFeedback.resolution_comment }}
              </div>
            </div>

            <div
              v-if="selectedFeedback.status !== STATUS.RESOLVED"
              class="reply-block"
            >
              <label
                class="reply-label"
                :for="`reply-${selectedFeedback.id}`"
              >Ответ заявителю</label>
              <textarea
                :id="`reply-${selectedFeedback.id}`"
                v-model="replyText"
                class="lk-textarea"
                data-testid="fb-reply"
                rows="4"
                maxlength="1000"
                placeholder="Заявитель увидит ваш ответ. Опишите решение или причину отказа."
              />
            </div>

            <div class="detail-actions">
              <button
                v-if="selectedFeedback.status !== STATUS.RESOLVED"
                class="lk-button lk-button--primary"
                data-testid="fb-resolve"
                :disabled="updatingId === selectedFeedback.id"
                @click="resolve(selectedFeedback.id)"
              >
                Отметить решённым
              </button>
              <button
                v-else
                class="lk-button lk-button--danger"
                data-testid="fb-reopen"
                :disabled="updatingId === selectedFeedback.id"
                @click="reopen(selectedFeedback.id)"
              >
                Вернуть в работу
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AdminPageShell>
</template>

<script setup>
import { ref, computed, watch, onMounted, getCurrentInstance } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { readSearchFromRoute } from '@/utils/searchQueryParam';
import { openItemFromRoute } from '@/utils/openQueryParam';
import AdminPageShell from '@/views/admin/AdminPageShell.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import { FilterTabs, SkeletonTransition, SkeletonCard } from '@/components/ui';
import { useDeletionsStore } from '@/stores/deletions';
import { getAllFeedback, updateFeedbackStatus, markFeedbackAsRead, setFeedbackFlag } from '@/api/feedback';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

const STATUS = { OPEN: 'Не решено', RESOLVED: 'Решено' };

const deletions = useDeletionsStore();
// $bus регистрируется в main.js через app.config.globalProperties.$bus.
const bus = getCurrentInstance()?.appContext?.config?.globalProperties?.$bus;

const feedbacks = ref([]);
const loading = ref(false);
const updatingId = ref(null);
const flaggingId = ref(null);
const activeFilter = ref('all');
const route = useRoute();
const router = useRouter();
// Из адреса: переход из сквозного поиска приносит запрос с собой.
const searchQuery = ref(readSearchFromRoute(route));
const searchVariants = computed(() => buildSearchVariants(searchQuery.value));
const sortKey = ref('id');
const sortDir = ref('desc');
const selectedId = ref(null);
const replyText = ref('');

const unreadCount = computed(() => feedbacks.value.filter((f) => !f.is_read).length);
const openCount = computed(() => feedbacks.value.filter((f) => f.status === STATUS.OPEN).length);
const resolvedCount = computed(() => feedbacks.value.filter((f) => f.status === STATUS.RESOLVED).length);

const filterTabs = computed(() => [
  { key: 'all', label: 'Все', count: feedbacks.value.length },
  { key: 'new', label: 'Новые', count: unreadCount.value },
  { key: 'open', label: 'В работе', count: openCount.value },
  { key: 'resolved', label: 'Решено', count: resolvedCount.value },
]);

const filteredFeedbacks = computed(() => {
  let list = feedbacks.value.slice();

  if (activeFilter.value === 'new') list = list.filter((f) => !f.is_read);
  else if (activeFilter.value === 'open') list = list.filter((f) => f.status === STATUS.OPEN);
  else if (activeFilter.value === 'resolved') list = list.filter((f) => f.status === STATUS.RESOLVED);

  const variants = searchVariants.value;
  if (variants.length) {
    list = list.filter((f) => matchesSearch(`${f.user_name || ''} ${f.message || ''}`, variants));
  }

  const dir = sortDir.value === 'asc' ? 1 : -1;
  list.sort((a, b) => {
    if (sortKey.value === 'author') return (a.user_name || '').localeCompare(b.user_name || '') * dir;
    if (sortKey.value === 'date') return (new Date(a.created_at) - new Date(b.created_at)) * dir;
    if (a.is_read !== b.is_read) return a.is_read ? 1 : -1;
    return (a.id - b.id) * dir;
  });
  return list;
});

const selectedFeedback = computed(() => feedbacks.value.find((f) => f.id === selectedId.value) || null);

const emptyText = computed(() => {
  if (searchVariants.value.length) return 'Ничего не найдено по запросу';
  if (activeFilter.value === 'new') return 'Новых обращений нет';
  if (activeFilter.value === 'open') return 'Обращений в работе нет';
  if (activeFilter.value === 'resolved') return 'Решённых обращений нет';
  return 'Обращений пока нет';
});

// Обращение открывается ТОЛЬКО по клику: при входе на вкладку/смене фильтра
// ничего не открываем автоматически. Если ранее выбранное обращение ушло из
// видимого списка, сбрасываем выделение и показываем подсказку.
watch(filteredFeedbacks, (list) => {
  if (selectedId.value != null && !list.some((f) => f.id === selectedId.value)) {
    selectedId.value = null;
  }
});

watch(selectedId, (id) => {
  replyText.value = '';
  if (id != null) autoMarkRead(id);
});

function selectRow(id) {
  selectedId.value = id;
}

function sortBy(key) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortKey.value = key;
    sortDir.value = key === 'author' ? 'asc' : 'desc';
  }
}

function patchLocal(id, patch) {
  const idx = feedbacks.value.findIndex((f) => f.id === id);
  if (idx !== -1) feedbacks.value[idx] = { ...feedbacks.value[idx], ...patch };
}

async function refresh() {
  loading.value = true;
  try {
    const data = await getAllFeedback();
    feedbacks.value = Array.isArray(data) ? data : [];
    // Переход из сквозного поиска: `?open` раскрывает найденное обращение.
    openItemFromRoute({ router, route, items: feedbacks.value, open: (item) => selectRow(item.id) });
  } catch (e) {
    console.error('Ошибка при загрузке обращений:', e);
    deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'обращения', type: 'error' });
  } finally {
    loading.value = false;
  }
}

// Автоотметка прочтения при открытии обращения (персонально для админа).
// Идемпотентна и монотонна (is_read: false -> true) - гонки при быстром
// переключении безопасны, seq-токен не нужен.
async function autoMarkRead(id) {
  const f = feedbacks.value.find((x) => x.id === id);
  if (!f || f.is_read) return;
  try {
    await markFeedbackAsRead(id);
    patchLocal(id, { is_read: true });
    bus?.emit('feedback-read', id);
  } catch (e) {
    // Тихо: неудачная автоотметка не мешает просмотру, счётчик подтянет опрос.
    console.error('Не удалось автоматически отметить обращение прочитанным:', e);
  }
}

// Общий флажок "важное / взять в работу" - виден всем администраторам.
async function toggleFlag(f) {
  const next = !f.flagged;
  flaggingId.value = f.id;
  try {
    await setFeedbackFlag(f.id, next);
    patchLocal(f.id, { flagged: next });
  } catch (e) {
    console.error('Ошибка при переключении флажка обращения:', e);
    deletions.notify({ prefix: 'Не удалось обновить флажок обращения', type: 'error' });
  } finally {
    flaggingId.value = null;
  }
}

async function resolve(id) {
  updatingId.value = id;
  const comment = replyText.value.trim();
  try {
    await updateFeedbackStatus(id, { status: STATUS.RESOLVED, ...(comment ? { comment } : {}) });
    patchLocal(id, {
      status: STATUS.RESOLVED,
      resolution_comment: comment || null,
      resolved_at: new Date().toISOString(),
    });
    replyText.value = '';
    deletions.notify({ prefix: 'Обращение ', bold: `#${id}`, suffix: ' отмечено решённым' });
  } catch (e) {
    console.error('Ошибка при обновлении обращения:', e);
    deletions.notify({ prefix: 'Не удалось обновить обращение', type: 'error' });
  } finally {
    updatingId.value = null;
  }
}

async function reopen(id) {
  updatingId.value = id;
  try {
    await updateFeedbackStatus(id, { status: STATUS.OPEN });
    patchLocal(id, { status: STATUS.OPEN, resolution_comment: null, resolved_at: null });
    deletions.notify({ prefix: 'Обращение ', bold: `#${id}`, suffix: ' возвращено в работу' });
  } catch (e) {
    console.error('Ошибка при обновлении обращения:', e);
    deletions.notify({ prefix: 'Не удалось обновить обращение', type: 'error' });
  } finally {
    updatingId.value = null;
  }
}

function formatDateTime(s) {
  if (!s) return '';
  return new Date(s).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function formatShortDate(s) {
  if (!s) return '';
  return new Date(s).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' });
}

function statusLabel(status) {
  return status === STATUS.RESOLVED ? 'Решено' : 'В работе';
}

onMounted(refresh);
</script>

<style scoped>
.feedback-container {
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--border);
  overflow: hidden;
}

/* Шапка (эталон management-header) */
.management-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Полоса фильтр-чипов со счётчиками */
.filter-strip {
  flex-shrink: 0;
  padding: 10px 20px;
  border-bottom: 1px solid var(--border);
}

/* Master-detail */
.content-container {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

/* Левая колонка: список */
.list-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  min-height: 0;
}

.list-header {
  display: flex;
  align-items: center;
  height: 43px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.header-col {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
  user-select: none;
}

.header-col.col-id,
.header-col.col-author,
.header-col.col-date {
  cursor: pointer;
}

.header-col.active-sort {
  color: var(--text);
}

.col-id {
  width: 14%;
}

.col-author {
  width: 36%;
}

.col-status {
  width: 26%;
}

.col-date {
  width: 16%;
  text-align: right;
}

.col-flag {
  width: 8%;
  text-align: center;
}

.list-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.ticket-row {
  display: flex;
  align-items: center;
  height: 48px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  font-size: 14px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.ticket-row:hover {
  background-color: var(--surface-2);
}

.ticket-row.selected {
  background-color: var(--accent-tint);
}

/* Непрочитанное обращение - жёлтый фон и полоса-акцент слева, тот же язык, что у
   непрочитанной заявки в Центре. Правило стоит ниже :hover/.selected (специфичность
   равная), чтобы подсветка держалась при наведении. */
.ticket-row.unread {
  background-color: var(--unread-bg);
  box-shadow: inset 3px 0 0 0 var(--unread-accent);
}

.ticket-row.unread .author {
  font-weight: 600;
  color: var(--text);
}

.cell {
  padding: 0 8px;
  overflow: hidden;
}

.cell.col-id {
  display: flex;
  align-items: center;
  gap: 7px;
}

.id-value {
  font-weight: 600;
  color: var(--text);
}

.unread-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-primary);
  flex-shrink: 0;
}

.ticket-row:not(.unread) .unread-dot {
  visibility: hidden;
}

.author {
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text);
}

.cell-date {
  text-align: right;
  color: var(--text-muted);
  font-size: 12px;
}

/* Флажок "важное / взять в работу" (Outlook-стиль): скрыт до наведения на строку,
   красный и всегда виден, если установлен. */
.cell.col-flag {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.flag-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  background: none;
  cursor: pointer;
  color: var(--text-muted);
  opacity: 0;
  border-radius: 6px;
  transition: opacity 0.15s ease, color 0.15s ease, background-color 0.15s ease;
}

.ticket-row:hover .flag-btn {
  opacity: 1;
}

.flag-btn:hover {
  background-color: var(--border);
  color: var(--danger-text);
}

.flag-btn.is-flagged {
  opacity: 1;
  color: var(--danger-text);
}

.flag-pole,
.flag-banner {
  stroke: currentColor;
  stroke-width: 1.4;
  fill: none;
  stroke-linejoin: round;
  stroke-linecap: round;
}

.flag-btn.is-flagged .flag-banner {
  fill: currentColor;
}

.status-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  min-width: 78px;
  text-align: center;
}

.status-open {
  background-color: var(--warning-bg);
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-resolved {
  background-color: var(--success-bg);
  color: var(--success-text);
  border: 1px solid var(--success);
}

.empty-list {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
}

.list-footer {
  flex-shrink: 0;
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

/* Правая колонка: детали */
.detail-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.no-selection {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  font-size: 14px;
}

.detail-body {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.detail-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
}

.detail-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-author {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.meta-dates {
  font-size: 13px;
  color: var(--color-text-muted);
}

.message-card {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-bg);
  padding: 16px;
  font-size: 14px;
  line-height: 1.55;
  color: var(--text);
  white-space: pre-wrap;
}

.answer-card {
  border: 1px solid var(--success);
  border-radius: var(--radius-lg);
  background: var(--success-bg);
  padding: 16px;
}

.answer-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--success-text);
  text-transform: uppercase;
  letter-spacing: 0.03em;
  margin-bottom: 6px;
}

.answer-text {
  font-size: 14px;
  line-height: 1.55;
  color: var(--text);
  white-space: pre-wrap;
}

.reply-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reply-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.detail-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

/* Брейкпоинт 767.98 = card-правила responsive-tables.css (на 768 rt-* не включён,
   был бы гибрид "шапка стек, а список ещё таблица"). */
@media (max-width: 767.98px) {
  /* Шапка в столбик: заголовок над строкой "поиск + обновить". В строке (десктоп
     space-between) заголовок "Обратная связь" переносится и налезает на поиск -
     стекаем вертикально. */
  .management-header {
    flex-direction: column;
    align-items: stretch;
    height: auto;
    gap: 10px;
    padding: 12px var(--gutter, 16px);
  }

  .header-controls {
    width: 100%;
  }

  /* Поиск занимает всю оставшуюся ширину строки (десктоп фиксирует .search 220px). */
  .header-controls :deep(.search) {
    flex: 1;
    width: auto;
    min-width: 0;
  }

  /* Master-detail -> вертикальный стек: список сверху, деталь снизу. */
  .content-container {
    flex-direction: column;
  }

  .list-section,
  .detail-section {
    width: 100%;
  }

  .list-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
    max-height: 360px;
  }

  /* Отступ, чтобы карточки не липли к краям скролл-контейнера. */
  .list-body {
    padding: 10px;
  }

  /* Строка -> карточка (rt-* responsive-tables.css красит .rt-row
     background:var(--surface) !important; unread/selected фон возвращаем с большей
     специфичностью + !important, иначе подсветка обращений пропадёт).
     Порядок как на десктопе (см. .ticket-row.unread после .selected выше): при
     равной специфичности .unread идёт ПОСЛЕ .selected, чтобы жёлтая подсветка
     непрочитанного держалась и у выбранной строки (на время async autoMarkRead
     строка бывает одновременно unread+selected). */
  .list-body .ticket-row.selected {
    background: var(--accent-tint) !important;
  }

  .list-body .ticket-row.unread {
    background: var(--unread-bg) !important;
  }

  /* Флажок "важное": на тач-экране ховера нет, показываем всегда (иначе строка
     "Важное" в карточке пустая и кнопка недоступна). */
  .flag-btn {
    opacity: 1;
  }

  /* col-id (без data-label) - строка-заголовок карточки: слева, на всю ширину
     (десктопные 14% в карточке дают узкий центрированный блок). */
  .list-body .col-id {
    width: 100%;
    justify-content: flex-start;
    padding-bottom: 4px;
  }

  /* Длинное ФИО показываем целиком переносом, а не обрезаем: ради этого список и
     разворачиваем в карточки (в таблице автор жёстко ellipsis). */
  .list-body .author {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }
}
</style>
