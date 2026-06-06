<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      data-testid="system-table-history-modal"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <div
        class="system-table-history-modal"
        @mousedown.stop
      >
        <div class="modal-header">
          <h3>История таблицы «{{ table.display_name }}»</h3>
          <button
            class="close-btn"
            aria-label="Закрыть"
            @click="close"
          >
            ×
          </button>
        </div>

        <div class="modal-content">
          <div
            v-if="loading"
            class="history-loading"
          >
            <LoaderSpinner label="Загрузка истории..." />
          </div>

          <div
            v-else-if="!history.length"
            class="history-empty"
          >
            История пуста
          </div>

          <div
            v-else
            class="history-timeline"
          >
            <div
              v-for="(item, index) in history"
              :key="item.id"
              class="history-item"
            >
              <div
                class="timeline-dot"
                :class="getActionClass(item.action_type)"
              />
              <div
                v-if="index < history.length - 1"
                class="timeline-line"
              />

              <div class="history-content">
                <div class="history-header">
                  <span class="user-name">{{ item.user_name || 'Система' }}</span>
                  <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                </div>

                <div class="action-text">
                  {{ getActionText(item) }}
                </div>

                <div
                  v-if="getActionDetails(item)"
                  class="action-comment"
                >
                  {{ getActionDetails(item) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useOverlayClose } from '@/composables/useOverlayClose';
import LoaderSpinner from './ui/LoaderSpinner.vue';

const FIELD_LABELS = {
  display_name: 'наименование',
  table_type: 'тип таблицы',
  show_fact_table: 'отображение таблицы по факту',
  fact_table_hint: 'подсказка',
  instruction: 'инструкция',
  map_link: 'ссылка на карту',
  status: 'статус',
  status_comment: 'комментарий статуса',
  location_description: 'описание местоположения',
};

const TYPE_LABELS = {
  cars: 'Машины',
  people: 'Люди',
};

export default {
  name: 'SystemTableHistoryModal',
  components: { LoaderSpinner },
  props: {
    table: { type: Object, required: true },
  },
  emits: ['close'],
  setup(_, { emit }) {
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
    return { onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      loading: false,
      history: [],
    };
  },
  mounted() {
    this.loadHistory();
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.close();
    },
    close() {
      this.$emit('close');
    },

    async loadHistory() {
      this.loading = true;
      try {
        const response = await apiRequest(`/system-tables/${this.table.id}/history`);
        if (response.ok) {
          const data = await response.json();
          this.history = Array.isArray(data) ? data : [];
        }
      } catch (error) {
        console.error('Error loading system table history:', error);
      } finally {
        this.loading = false;
      }
    },

    getActionClass(actionType) {
      const classes = {
        created: 'dot-create',
        updated: 'dot-update',
        archived: 'dot-archive',
        restored: 'dot-restore',
        columns_updated: 'dot-update',
        appearance_updated: 'dot-update',
      };
      return classes[actionType] || 'dot-default';
    },

    getActionText(item) {
      const texts = {
        created: 'Таблица создана',
        updated: 'Данные таблицы изменены',
        archived: 'Таблица архивирована',
        restored: 'Таблица восстановлена из архива',
        columns_updated: 'Изменены столбцы',
        appearance_updated: 'Изменено оформление',
      };
      return texts[item.action_type] || item.action_type;
    },

    getActionDetails(item) {
      const d = item.details;
      if (!d || typeof d !== 'object') return '';

      switch (item.action_type) {
        case 'created': {
          const parts = [];
          if (d.display_name) parts.push(`Наименование: "${d.display_name}"`);
          if (d.name) parts.push(`Системное имя: "${d.name}"`);
          if (d.table_type) parts.push(`Тип: ${TYPE_LABELS[d.table_type] || d.table_type}`);
          return parts.join(' / ');
        }
        case 'updated': {
          const keys = Object.keys(d).filter(k => k !== 'is_active');
          if (!keys.length) return '';
          const parts = keys.map(k => {
            const label = FIELD_LABELS[k] || k;
            let val = d[k];
            if (val === null || val === undefined || val === '') return `${label}: -`;
            if (typeof val === 'boolean') val = val ? 'да' : 'нет';
            if (typeof val === 'string' && val.length > 60) val = `${val.slice(0, 60)}...`;
            return `${label}: ${val}`;
          });
          return parts.join(' / ');
        }
        case 'columns_updated': {
          const variant = d.variant === 'fact' ? 'По факту' : 'Основная таблица';
          const count = Array.isArray(d.fields) ? d.fields.length : 0;
          return `${variant}, столбцов: ${count}`;
        }
        case 'appearance_updated':
          return '';
        case 'archived':
        case 'restored':
          return '';
        default:
          return '';
      }
    },

    formatDateTime(s) {
      if (!s) return '';
      const d = new Date(s);
      return d.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }).replace(',', '');
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 12000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.system-table-history-modal {
  background: white;
  border-radius: 30px;
  width: 900px;
  max-width: 95%;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from {
    transform: translateY(20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #333;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: #a2a2a2;
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: #f5f5f5;
  color: #333;
}

.modal-content {
  padding: 20px 25px;
  overflow-y: auto;
  flex: 1;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  color: #6b7280;
  font-size: 14px;
}

.history-timeline {
  position: relative;
  padding-left: 32px;
}

.history-item {
  position: relative;
  padding-bottom: 22px;
}

.history-item:last-child {
  padding-bottom: 0;
}

.timeline-dot {
  position: absolute;
  left: -28px;
  top: 4px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #6b7280;
  box-shadow: 0 0 0 3px #fff, 0 0 0 4px #e6e6e6;
  z-index: 1;
}

.timeline-dot.dot-create,
.timeline-dot.dot-restore {
  background: #10b981;
}

.timeline-dot.dot-update {
  background: #f59e0b;
}

.timeline-dot.dot-archive {
  background: #6b7280;
}

.timeline-dot.dot-default {
  background: #9ca3af;
}

.timeline-line {
  position: absolute;
  left: -22px;
  top: 18px;
  bottom: -4px;
  width: 2px;
  background: #e6e6e6;
}

.history-content {
  background: #fafafa;
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  padding: 12px 14px;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 6px;
}

.user-name {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
}

.action-time {
  font-size: 12px;
  color: #a2a2a2;
  white-space: nowrap;
}

.action-text {
  font-size: 14px;
  color: #1f2937;
  font-weight: 500;
  line-height: 1.4;
}

.action-comment {
  margin-top: 6px;
  font-size: 12px;
  color: #4b5563;
  line-height: 1.4;
  word-break: break-word;
}
</style>
