<template>
  <section class="denials">
    <header class="page-header">
      <h2 class="page-title">
        Журнал отказов в доступе
      </h2>
      <RefreshButton
        :loading="loading"
        @refresh="fetch"
      />
    </header>

    <div class="toggle-row">
      <button
        class="lk-button"
        :class="mode === 'active' ? 'lk-button--primary' : 'lk-button--ghost'"
        @click="setMode('active')"
      >
        Активные (3 мес)
      </button>
      <button
        class="lk-button"
        :class="mode === 'archive' ? 'lk-button--primary' : 'lk-button--ghost'"
        @click="setMode('archive')"
      >
        Архив
      </button>
    </div>

    <form
      class="filters"
      @submit.prevent="applyFilters"
    >
      <input
        v-model="filters.user_id"
        class="lk-input filter-input"
        type="number"
        placeholder="ID пользователя"
      >
      <input
        v-model="filters.resource"
        class="lk-input filter-input"
        type="text"
        placeholder="Ресурс (substring)"
      >
      <select
        v-model="filters.reason"
        class="lk-select filter-input"
      >
        <option value="">
          Причина — все
        </option>
        <option value="permission_denied">
          Нет прав
        </option>
        <option value="account_banned">
          Заблокирован
        </option>
      </select>
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
      <button
        type="submit"
        class="lk-button lk-button--primary"
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

    <div
      v-if="mode === 'active'"
      class="actions-row"
    >
      <button
        class="lk-button lk-button--danger"
        :disabled="!hasActiveFilters || items.length === 0"
        @click="confirmDeleteFiltered"
      >
        Очистить с фильтрами
      </button>
      <button
        class="lk-button lk-button--ghost"
        @click="confirmArchiveAll"
      >
        Архивировать всё за период
      </button>
    </div>

    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Дата</th>
            <th>Пользователь</th>
            <th>Ресурс</th>
            <th>Право</th>
            <th>Причина</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in items"
            :key="item.id"
          >
            <td>{{ item.id }}</td>
            <td class="muted">
              {{ formatDateTime(item.created_at) }}
            </td>
            <td>
              <span v-if="item.user_name">{{ item.user_name }}</span>
              <span
                v-else
                class="muted"
              >ID {{ item.user_id || '-' }}</span>
            </td>
            <td><code>{{ item.resource }}</code></td>
            <td>
              <code v-if="item.permission_key">{{ item.permission_key }}</code>
              <span
                v-else
                class="muted"
              >—</span>
            </td>
            <td>
              <span
                class="badge"
                :class="reasonClass(item.reason)"
              >
                {{ reasonLabel(item.reason) }}
              </span>
            </td>
            <td class="muted">
              {{ item.ip_address || '—' }}
            </td>
          </tr>
          <tr v-if="items.length === 0 && !loading">
            <td
              colspan="7"
              class="empty-cell"
            >
              {{ mode === 'archive' ? 'Архив пуст' : 'Записей нет' }}
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
        @click="page = page - 1; fetch()"
      >
        Назад
      </button>
      <span>Стр. {{ page }} / {{ totalPages }}</span>
      <button
        class="lk-button lk-button--ghost"
        :disabled="page >= totalPages"
        @click="page = page + 1; fetch()"
      >
        Вперёд
      </button>
    </footer>
  </section>
</template>

<script>
import {
  listAccessDenials,
  listAccessDenialsArchive,
  deleteAccessDenials,
  archiveAccessDenials,
} from '@/api/permissions';
import RefreshButton from '@/components/RefreshButton.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

export default {
  name: 'AccessDenialsLog',
  components: { RefreshButton },
  data() {
    return {
      mode: 'active',
      items: [],
      total: 0,
      page: 1,
      limit: 50,
      loading: false,
      filters: { user_id: '', resource: '', reason: '', from: '', to: '' },
    };
  },
  computed: {
    totalPages() {
      return Math.max(1, Math.ceil(this.total / this.limit));
    },
    hasActiveFilters() {
      return Object.values(this.filters).some(v => v !== '');
    },
  },
  mounted() {
    this.fetch();
  },
  methods: {
    setMode(m) {
      this.mode = m;
      this.page = 1;
      this.fetch();
    },
    applyFilters() {
      this.page = 1;
      this.fetch();
    },
    resetFilters() {
      this.filters = { user_id: '', resource: '', reason: '', from: '', to: '' };
      this.page = 1;
      this.fetch();
    },
    async fetch() {
      this.loading = true;
      try {
        const params = {
          page: this.page,
          limit: this.limit,
          ...this.filters,
        };
        const fn = this.mode === 'archive' ? listAccessDenialsArchive : listAccessDenials;
        const data = (await fn(params)) || {};
        this.items = Array.isArray(data.items) ? data.items : [];
        this.total = typeof data.total === 'number' ? data.total : 0;
      } catch (e) {
        console.error('Ошибка загрузки журнала:', e);
      } finally {
        this.loading = false;
      }
    },
    async confirmDeleteFiltered() {
      const ok = await useUiStore().confirm({
        title: 'Очистить записи?',
        message: 'Записи активной таблицы по выбранным фильтрам будут удалены. Архив не затрагивается.',
        confirmText: 'Удалить',
        cancelText: 'Отмена',
        danger: true,
      });
      if (!ok) return;
      try {
        await deleteAccessDenials(this.filters);
        await this.fetch();
        useDeletionsStore().notify({ bold: 'Записи удалены', suffix: ' по выбранным фильтрам' });
      } catch (e) {
        console.error('Ошибка удаления:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить ', bold: 'записи журнала', type: 'error' });
      }
    },
    async confirmArchiveAll() {
      const cutoff = this.filters.to || null;
      const msg = cutoff
        ? `Все записи до ${cutoff} будут перенесены в архив.`
        : 'Все записи старше 3 месяцев будут перенесены в архив.';
      const ok = await useUiStore().confirm({
        title: 'Архивировать записи?',
        message: msg,
        confirmText: 'Архивировать',
        cancelText: 'Отмена',
        danger: false,
      });
      if (!ok) return;
      try {
        await archiveAccessDenials(cutoff);
        await this.fetch();
        useDeletionsStore().notify({ bold: 'Записи архивированы' });
      } catch (e) {
        console.error('Ошибка архивации:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'архивировать записи', type: 'error' });
      }
    },
    formatDateTime(s) {
      if (!s) return '';
      return new Date(s).toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
      });
    },
    reasonLabel(r) {
      return { permission_denied: 'Нет прав', account_banned: 'Заблокирован' }[r] || r;
    },
    reasonClass(r) {
      return r === 'account_banned' ? 'badge--danger' : 'badge--warning';
    },
  },
};
</script>

<style scoped>
.denials {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.toggle-row,
.actions-row {
  display: flex;
  gap: 8px;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.filter-input {
  flex: 1 1 160px;
  max-width: 220px;
}

.table-wrap {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: auto;
  box-shadow: var(--shadow-sm);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.data-table th,
.data-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.data-table th {
  background: var(--color-bg);
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  color: var(--color-text-muted);
}

.data-table tbody tr:last-child td {
  border-bottom: none;
}

.muted {
  color: var(--color-text-muted);
}

code {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
}

.badge {
  display: inline-block;
  padding: 3px 8px;
  border-radius: var(--radius-pill);
  font-size: 11px;
  font-weight: 500;
}

.badge--warning {
  background: #fef3c7;
  color: #92400e;
}

.badge--danger {
  background: #fee2e2;
  color: #991b1b;
}

.empty-cell {
  text-align: center;
  padding: 40px;
  color: var(--color-text-muted);
}

.pagination {
  display: flex;
  align-items: center;
  gap: 12px;
  justify-content: flex-end;
  font-size: 13px;
}
</style>
