<template>
  <section class="pda">
    <header class="page-header">
      <h2 class="page-title">
        Журнал доступа к персональным данным
      </h2>
      <RefreshButton
        :loading="loading"
        @refresh="fetch"
      />
    </header>

    <p class="pda__hint">
      Кто и когда обращался к данным сотрудников: карточки и списки, выгрузка бланков,
      просмотр доступных вложений. Записи хранятся по требованию 152-ФЗ и не удаляются.
    </p>

    <!-- Десктоп: форма фильтров инлайн (как было). -->
    <form
      v-if="!isNarrow"
      class="filters"
      @submit.prevent="applyFilters"
    >
      <input
        v-model="filters.username"
        class="lk-input filter-input"
        type="text"
        placeholder="Логин пользователя"
        data-testid="pda-filter-username"
      >
      <BaseDropdown
        v-model="filters.action"
        class="filter-input"
        :options="actionOptions"
        value-key="value"
        label-key="label"
        placeholder="Действие - все"
      />
      <BaseDropdown
        v-model="filters.resource"
        class="filter-input"
        :options="resourceOptions"
        value-key="value"
        label-key="label"
        placeholder="Данные - все"
      />
      <input
        v-model="filters.from"
        class="lk-input filter-input"
        type="datetime-local"
      >
      <input
        v-model="filters.to"
        class="lk-input filter-input"
        type="datetime-local"
      >
      <label class="pda__denied">
        <input
          v-model="filters.only_denied"
          type="checkbox"
          data-testid="pda-filter-denied"
        >
        Только отказы
      </label>
      <button
        type="submit"
        class="lk-button lk-button--primary"
        data-testid="pda-apply"
      >
        Применить
      </button>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        @click="resetFilters"
      >
        Сбросить
      </button>
    </form>

    <!-- Мобилка: весь тулбар фильтров свёрнут в кнопку «Фильтр» (отдельного поиска
         у журнала нет - вся форма это фильтры, применяемые по «Применить»). -->
    <div
      v-else
      class="filters-mobile"
    >
      <FilterButton
        :active="filtersActive"
        data-testid="pda-filter-btn"
        @click="openFilterSheet"
      />
    </div>

    <!-- Мобилка: фильтры в bottom-sheet. Черновик формы откатывается при закрытии
         без «Применить», чтобы пагинация не подхватила неприменённые значения. -->
    <FilterSheet
      v-if="isNarrow"
      :show="showFilterSheet"
      @close="closeFilterSheet"
    >
      <div class="filter-section">
        <span class="filter-label">Логин пользователя</span>
        <input
          v-model="filters.username"
          class="lk-input"
          type="text"
          placeholder="Логин пользователя"
          data-testid="pda-sheet-username"
        >
      </div>
      <div class="filter-section">
        <span class="filter-label">Действие</span>
        <BaseDropdown
          v-model="filters.action"
          :options="actionOptions"
          value-key="value"
          label-key="label"
          placeholder="Действие - все"
          teleport
          data-testid="pda-sheet-action"
        />
      </div>
      <div class="filter-section">
        <span class="filter-label">Данные</span>
        <BaseDropdown
          v-model="filters.resource"
          :options="resourceOptions"
          value-key="value"
          label-key="label"
          placeholder="Данные - все"
          teleport
          data-testid="pda-sheet-resource"
        />
      </div>
      <div class="filter-section">
        <span class="filter-label">С даты</span>
        <input
          v-model="filters.from"
          class="lk-input"
          type="datetime-local"
          data-testid="pda-sheet-from"
        >
      </div>
      <div class="filter-section">
        <span class="filter-label">По дату</span>
        <input
          v-model="filters.to"
          class="lk-input"
          type="datetime-local"
          data-testid="pda-sheet-to"
        >
      </div>
      <label class="pda__denied">
        <input
          v-model="filters.only_denied"
          type="checkbox"
          data-testid="pda-sheet-denied"
        >
        Только отказы
      </label>

      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--ghost"
          :disabled="!filtersActive"
          data-testid="pda-sheet-reset"
          @click="resetFromSheet"
        >
          Сбросить
        </button>
        <button
          type="button"
          class="lk-button lk-button--primary"
          data-testid="pda-sheet-apply"
          @click="applyFromSheet"
        >
          Применить
        </button>
      </template>
    </FilterSheet>

    <div class="table-wrap">
      <table class="data-table rt-table">
        <thead class="rt-head-row">
          <tr>
            <th>Дата</th>
            <th>Пользователь</th>
            <th>Действие</th>
            <th>Данные</th>
            <th>Адрес запроса</th>
            <th>Ответ</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in items"
            :key="item.id"
            class="rt-row"
            data-testid="pda-row"
          >
            <td
              class="muted"
              data-label="Дата"
            >
              {{ formatDateTime(item.created_at) }}
            </td>
            <td data-label="Пользователь">
              <span v-if="item.user_name">{{ item.user_name }}</span>
              <span
                v-else
                class="muted"
              >{{ item.username ? formatLogin(item.username) : '—' }}</span>
              <span
                v-if="item.user_name && item.username"
                class="pda__login"
              >{{ formatLogin(item.username) }}</span>
            </td>
            <td data-label="Действие">
              {{ actionLabel(item.action) }}
            </td>
            <td data-label="Данные">
              {{ resourceLabel(item.resource) }}
            </td>
            <td
              class="pda__path"
              data-label="Адрес запроса"
            >
              <code>{{ item.path }}</code>
            </td>
            <td data-label="Ответ">
              <span
                class="badge"
                :class="statusClass(item.status_code)"
              >
                {{ statusLabel(item.status_code) }}
              </span>
            </td>
            <td
              class="muted"
              data-label="IP"
            >
              {{ item.ip_address || '—' }}
            </td>
          </tr>
          <tr v-if="items.length === 0 && !loading">
            <td
              colspan="7"
              class="empty-cell"
            >
              Записей нет
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <footer class="pagination">
      <span class="muted">Всего: {{ total }}</span>
      <button
        class="lk-button lk-button--ghost"
        :disabled="page <= 1"
        @click="goToPage(page - 1)"
      >
        Назад
      </button>
      <span>Стр. {{ page }} / {{ totalPages }}</span>
      <button
        class="lk-button lk-button--ghost"
        :disabled="page >= totalPages"
        data-testid="pda-next"
        @click="goToPage(page + 1)"
      >
        Вперёд
      </button>
    </footer>
  </section>
</template>

<script>
import { listPDAudit } from '@/api/pd-audit';
import { formatLogin } from '@/utils/formatName';
import RefreshButton from '@/components/RefreshButton.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { useDeletionsStore } from '@/stores/deletions';

// Подписи действий и видов данных: в журнале лежат технические значения,
// которые пишет middleware (internal/middleware/pd_audit.go).
const ACTION_LABELS = {
  view: 'Просмотр',
  create: 'Создание',
  update: 'Изменение',
  delete: 'Удаление',
};

// Разделы журнала обращений. Перечень обязан покрывать все значения, которые
// возвращает pathToResource в internal/middleware/pd_audit.go: неописанный раздел
// отсутствует в фильтре и показывается в строке служебным кодом.
const RESOURCE_LABELS = {
  employee: 'Сотрудники заявок',
  unique_employee: 'Реестр сотрудников',
  attachment: 'Вложения заявок',
  attachment_blank: 'Выгрузка бланка',
  available_attachment: 'Доступное вложение',
  application_participants: 'Участники заявки',
  application_file: 'Файлы заявки',
  application_archive: 'Архив заявки',
  pd_consent_collection: 'Сбор согласий',
  file_archive: 'Файловый архив',
  applications_export: 'Выгрузка реестра заявок',
  request_logs_export: 'Выгрузка журнала обращений',
  search: 'Сквозной поиск',
};

const EMPTY_FILTERS = {
  username: '', action: '', resource: '', from: '', to: '', only_denied: false,
};

export default {
  name: 'PdAuditLog',
  components: {
    RefreshButton, BaseDropdown, FilterButton, FilterSheet,
  },
  setup() {
    const { isNarrow } = useNarrowScreen();
    return { isNarrow };
  },
  data() {
    return {
      items: [],
      total: 0,
      page: 1,
      limit: 50,
      loading: false,
      filters: { ...EMPTY_FILTERS },
      // Мобилка: фильтры свёрнуты в bottom-sheet; backup хранит применённое
      // состояние для отката черновика при закрытии без «Применить».
      showFilterSheet: false,
      filtersBackup: { ...EMPTY_FILTERS },
      actionOptions: [
        { value: '', label: 'Действие - все' },
        ...Object.entries(ACTION_LABELS).map(([value, label]) => ({ value, label })),
      ],
      resourceOptions: [
        { value: '', label: 'Данные - все' },
        ...Object.entries(RESOURCE_LABELS).map(([value, label]) => ({ value, label })),
      ],
    };
  },
  computed: {
    totalPages() {
      return Math.max(1, Math.ceil(this.total / this.limit));
    },
    // Есть ли значимые фильтры (учитываем boolean-флаг «Только отказы»): точка на
    // кнопке «Фильтр» и доступность «Сбросить». Пустой EMPTY_FILTERS -> false.
    filtersActive() {
      const f = this.filters;
      return !!(f.username || f.action || f.resource || f.from || f.to || f.only_denied);
    },
  },
  mounted() {
    this.fetch();
  },
  methods: {
    formatLogin,

    applyFilters() {
      this.page = 1;
      this.fetch();
    },
    resetFilters() {
      this.filters = { ...EMPTY_FILTERS };
      this.page = 1;
      this.fetch();
    },
    openFilterSheet() {
      this.filtersBackup = { ...this.filters };
      this.showFilterSheet = true;
    },
    // Крестик/overlay/Escape/свайп: откатываем неприменённый черновик, чтобы вне
    // sheet filters всегда равнялись применённому (пагинация шлёт именно filters).
    closeFilterSheet() {
      this.filters = { ...this.filtersBackup };
      this.showFilterSheet = false;
    },
    applyFromSheet() {
      this.showFilterSheet = false;
      this.applyFilters();
    },
    resetFromSheet() {
      this.showFilterSheet = false;
      this.resetFilters();
    },
    goToPage(page) {
      this.page = page;
      this.fetch();
    },
    async fetch() {
      this.loading = true;
      try {
        const data = (await listPDAudit({
          page: this.page,
          limit: this.limit,
          ...this.filters,
        })) || {};
        this.items = Array.isArray(data.items) ? data.items : [];
        this.total = typeof data.total === 'number' ? data.total : 0;
      } catch (err) {
        this.items = [];
        this.total = 0;
        useDeletionsStore().notify({
          prefix: 'Не удалось загрузить журнал: ',
          bold: err.message || 'ошибка сервера',
          type: 'error',
        });
      } finally {
        this.loading = false;
      }
    },
    actionLabel(action) {
      return ACTION_LABELS[action] || action;
    },
    resourceLabel(resource) {
      return RESOURCE_LABELS[resource] || resource;
    },
    // Код ответа переводим в понятный статус: для проверки важно отличить
    // состоявшийся просмотр от неудачной попытки.
    statusLabel(code) {
      if (code >= 500) return `Сбой ${code}`;
      if (code === 403) return 'Отказано';
      if (code === 404) return 'Не найдено';
      if (code >= 400) return `Ошибка ${code}`;
      return 'Просмотрено';
    },
    statusClass(code) {
      if (code >= 500) return 'badge--danger';
      if (code >= 400) return 'badge--warning';
      return 'badge--ok';
    },
    formatDateTime(value) {
      if (!value) return '—';
      const d = new Date(value);
      if (Number.isNaN(d.getTime())) return value;
      return d.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
      });
    },
  },
};
</script>

<style scoped>
.pda {
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

.pda__hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--color-text-muted, #666);
}

.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.filters-mobile {
  display: flex;
}

.filter-input {
  min-width: 180px;
  flex: 0 1 200px;
}

.pda__denied {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--color-text, #000);
  cursor: pointer;
}

.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border, #e6e6e6);
  border-radius: var(--radius-md, 15px);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.data-table th,
.data-table td {
  padding: 9px 12px;
  text-align: left;
  border-bottom: 1px solid var(--color-border, #e6e6e6);
}

.data-table th {
  font-weight: 600;
  white-space: nowrap;
}

.pda__path code {
  font-size: 12px;
  word-break: break-all;
}

.pda__login {
  margin-left: 6px;
  font-size: 12px;
  color: var(--color-text-muted, #999);
}

.muted {
  color: var(--color-text-muted, #777);
}

.empty-cell {
  padding: 20px;
  text-align: center;
  color: var(--color-text-muted, #777);
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 12px;
  white-space: nowrap;
}

.badge--ok {
  background: var(--success-bg);
  color: #2e7d47;
}

.badge--warning {
  background: #fff3cd;
  color: #856404;
}

.badge--danger {
  background: rgba(192, 57, 43, 0.14);
  color: #c0392b;
}

.pagination {
  display: flex;
  align-items: center;
  gap: 10px;
}

@media (max-width: 767.98px) {
  .filter-input {
    flex: 1 1 100%;
  }

  .pda__path code {
    font-size: 11px;
  }
}
</style>
