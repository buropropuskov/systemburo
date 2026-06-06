<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      data-testid="system-table-history-modal"
    >
    <div class="modal-content">
      <div class="modal-header">
        <h3 class="modal-title">
          История таблицы «{{ table.display_name }}»
        </h3>
        <button
          class="modal-close"
          aria-label="Закрыть"
          @click="$emit('close')"
        >
          ×
        </button>
      </div>
      <div class="modal-body">
        <div
          v-if="isLoading"
          class="loader"
        >
          Загрузка...
        </div>
        <div
          v-else-if="!groups.length"
          class="empty"
        >
          История пуста
        </div>
        <ul
          v-else
          class="history-list"
        >
          <li
            v-for="(g, gi) in groups"
            :key="gi"
            class="history-item"
          >
            <div class="history-row">
              <span
                class="history-badge"
                :class="`history-badge--${g.action_type}`"
              >{{ actionLabel(g.action_type) }}</span>
              <span class="history-user">{{ g.user_name || 'Система' }}</span>
              <span class="history-time">{{ formatDate(g.created_at) }}</span>
            </div>
            <button
              v-if="g.entries.length > 1 || hasDetails(g.entries[0])"
              class="history-toggle"
              @click="toggle(gi)"
            >
              {{ expanded[gi] ? 'Свернуть' : `Раскрыть (${g.entries.length})` }}
            </button>
            <ul
              v-if="expanded[gi]"
              class="history-details"
            >
              <li
                v-for="entry in g.entries"
                :key="entry.id"
                class="history-detail-row"
              >
                <pre>{{ formatDetails(entry.details) }}</pre>
              </li>
            </ul>
          </li>
        </ul>
      </div>
      <div class="modal-footer">
        <button
          class="lk-btn"
          @click="$emit('close')"
        >
          Закрыть
        </button>
      </div>
    </div>
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client';

const ACTION_LABELS = {
  created: 'Создана',
  updated: 'Изменена',
  archived: 'Архивирована',
  restored: 'Восстановлена',
  columns_updated: 'Столбцы',
  appearance_updated: 'Оформление',
};

// Окно для слипания соседних записей одного юзера и одного типа.
const GROUP_WINDOW_MS = 60_000;

export default {
  name: 'SystemTableHistoryModal',
  props: {
    table: { type: Object, required: true },
  },
  emits: ['close'],
  data() {
    return {
      history: [],
      isLoading: false,
      expanded: {},
    };
  },
  computed: {
    groups() {
      // Склеиваем соседние записи одного user_id + action_type в окно 60с.
      const out = [];
      for (const h of this.history) {
        const tail = out[out.length - 1];
        const sameType = tail && tail.action_type === h.action_type;
        const sameUser = tail && tail.user_id === h.user_id;
        const close = tail && Math.abs(
          new Date(tail.created_at).getTime() - new Date(h.created_at).getTime(),
        ) <= GROUP_WINDOW_MS;
        if (sameType && sameUser && close) {
          tail.entries.push(h);
        } else {
          out.push({
            action_type: h.action_type,
            user_id: h.user_id,
            user_name: h.user_name,
            created_at: h.created_at,
            entries: [h],
          });
        }
      }
      return out;
    },
  },
  mounted() {
    this.load();
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.$emit('close');
    },
    async load() {
      this.isLoading = true;
      try {
        const response = await apiRequest(`/system-tables/${this.table.id}/history`);
        if (response.ok) {
          const data = await response.json();
          this.history = Array.isArray(data) ? data : [];
        }
      } catch (e) {
        console.error('Error loading history:', e);
      } finally {
        this.isLoading = false;
      }
    },
    toggle(gi) {
      this.expanded = { ...this.expanded, [gi]: !this.expanded[gi] };
    },
    hasDetails(entry) {
      return entry && entry.details && Object.keys(entry.details).length > 0;
    },
    actionLabel(t) {
      return ACTION_LABELS[t] || t;
    },
    formatDate(s) {
      if (!s) return '';
      return new Date(s).toLocaleString('ru-RU');
    },
    formatDetails(d) {
      if (!d) return '';
      try {
        return JSON.stringify(d, null, 2);
      } catch {
        return String(d);
      }
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 11000;
}

.modal-content {
  background: #fff;
  width: 100vw;
  height: 100vh;
  max-width: none;
  max-height: none;
  border-radius: 0;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 32px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
}

.modal-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #1f2937;
}

.modal-close {
  background: transparent;
  border: 0;
  font-size: 28px;
  line-height: 1;
  color: #6b7280;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 8px;
  transition: background 0.2s ease, color 0.2s ease;
}

.modal-close:hover {
  background: #f3f4f6;
  color: #1f2937;
}

.modal-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 24px 32px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  padding: 16px 32px;
  border-top: 1px solid #e6e6e6;
  background: #fff;
}

.loader,
.empty {
  text-align: center;
  color: #6b7280;
  padding: 20px 0;
}

.history-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.history-item {
  padding: 12px 0;
  border-bottom: 1px solid #e6e6e6;
}

.history-item:last-child {
  border-bottom: 0;
}

.history-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.history-badge {
  display: inline-flex;
  align-items: center;
  background: #4F5BDF;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 50px;
}

.history-badge--archived {
  background: #6b7280;
}

.history-badge--restored {
  background: #10b981;
}

.history-badge--columns_updated,
.history-badge--appearance_updated,
.history-badge--updated {
  background: #f59e0b;
}

.history-badge--created {
  background: #10b981;
}

.history-user {
  font-size: 13px;
  color: #1f2937;
  font-weight: 500;
}

.history-time {
  font-size: 12px;
  color: #6b7280;
  margin-left: auto;
}

.history-toggle {
  margin-top: 6px;
  background: transparent;
  border: 0;
  color: #4F5BDF;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}

.history-toggle:hover {
  text-decoration: underline;
}

.history-details {
  list-style: none;
  padding: 8px 0 0;
  margin: 0;
}

.history-detail-row {
  background: #f9fafb;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  padding: 8px 10px;
  margin-top: 6px;
}

.history-detail-row pre {
  margin: 0;
  font-size: 11px;
  color: #4b5563;
  white-space: pre-wrap;
  word-break: break-word;
}

.lk-btn {
  padding: 8px 16px;
  border-radius: 10px;
  border: 0;
  background: #4F5BDF;
  color: #fff;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.lk-btn:hover {
  background: #3a45b2;
}
</style>
