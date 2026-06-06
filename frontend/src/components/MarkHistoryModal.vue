<template>
  <Teleport to="body">
  <div class="modal-overlay" data-testid="mark-history-modal" @click.self="$emit('close')">
    <div class="modal-content">
      <h3 class="modal-title">История марки «{{ mark.name }}»</h3>
      <div v-if="isLoading" class="loader">Загрузка...</div>
      <div v-else-if="!history.length" class="empty">Записей нет</div>
      <ul v-else class="history-list">
        <li
          v-for="h in history"
          :key="h.id"
          class="history-item"
        >
          <div class="history-action">{{ actionLabel(h.action_type) }}</div>
          <div v-if="h.old_value || h.new_value" class="history-values">
            <span v-if="h.old_value" class="old">{{ h.old_value }}</span>
            <span v-if="h.old_value && h.new_value" class="arrow">→</span>
            <span v-if="h.new_value" class="new">{{ h.new_value }}</span>
          </div>
          <div class="history-meta">
            {{ formatDate(h.created_at) }}
          </div>
        </li>
      </ul>
      <div class="modal-actions">
        <button class="lk-btn" @click="$emit('close')">Закрыть</button>
      </div>
    </div>
  </div>
  </Teleport>
</template>

<script>
import { getMarkHistory } from '@/api/marks';

const ACTION_LABELS = {
  created: 'Создана',
  renamed: 'Переименована',
  archived: 'Архивирована',
  restored: 'Восстановлена',
};

export default {
  name: 'MarkHistoryModal',
  props: {
    mark: { type: Object, required: true },
  },
  emits: ['close'],
  data() {
    return { history: [], isLoading: false };
  },
  mounted() {
    this.load();
  },
  methods: {
    async load() {
      this.isLoading = true;
      try {
        const data = await getMarkHistory(this.mark.id);
        this.history = Array.isArray(data) ? data : [];
      } finally {
        this.isLoading = false;
      }
    },
    actionLabel(t) {
      return ACTION_LABELS[t] || t;
    },
    formatDate(s) {
      if (!s) return '';
      const d = new Date(s);
      return d.toLocaleString('ru-RU');
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 480px;
  max-width: 92vw;
  max-height: 80vh;
  overflow-y: auto;
}

.modal-title {
  margin: 0 0 16px;
  font-size: 18px;
}

.loader,
.empty {
  text-align: center;
  color: #888;
  padding: 20px 0;
}

.history-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.history-item {
  padding: 10px 0;
  border-bottom: 1px solid var(--color-border);
}

.history-item:last-child {
  border-bottom: 0;
}

.history-action {
  font-weight: 600;
  margin-bottom: 4px;
}

.history-values {
  font-size: 13px;
  margin-bottom: 4px;
}

.history-values .old {
  color: #888;
  text-decoration: line-through;
}

.history-values .arrow {
  margin: 0 6px;
  color: #888;
}

.history-values .new {
  color: var(--color-primary);
}

.history-meta {
  font-size: 12px;
  color: #888;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.lk-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 0;
  background: var(--color-primary);
  color: #fff;
  cursor: pointer;
}
</style>
