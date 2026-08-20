<template>
  <div
    class="log-details-section"
    :class="{ 'with-details': log }"
  >
    <div
      v-if="log"
      class="log-details-content"
    >
      <div class="details-header">
        <h3 class="details-title">
          Детали запроса
        </h3>
        <button
          class="close-details-btn"
          @click="$emit('close')"
        >
          &times;
        </button>
      </div>

      <div class="details-body">
        <div class="details-grid">
          <div class="detail-group">
            <label class="detail-label">ID запроса:</label>
            <span class="detail-value">{{ log.id }}</span>
          </div>

          <div class="detail-group">
            <label class="detail-label">Время:</label>
            <span class="detail-value">{{ formatFullDate(log.created_at) }}</span>
          </div>

          <div class="detail-group">
            <label class="detail-label">Метод:</label>
            <RequestLogBadge
              kind="method"
              :value="log.method"
              variant="detail"
            />
          </div>

          <div class="detail-group">
            <label class="detail-label">URL:</label>
            <span class="detail-value path-value">
              {{ log.url }}
            </span>
          </div>

          <div class="detail-group">
            <label class="detail-label">Статус ответа:</label>
            <RequestLogBadge
              kind="status"
              :value="log.response_status"
              variant="detail"
            />
          </div>

          <div class="detail-group">
            <label class="detail-label">Время выполнения:</label>
            <span class="detail-value">{{ formatDuration(log) }}</span>
          </div>

          <div class="detail-group">
            <label class="detail-label">Пользователь:</label>
            <span class="detail-value">
              {{ log.username || 'Аноним' }}
              <span
                v-if="log.user_id"
                class="user-id"
              >(ID: {{ log.user_id }})</span>
            </span>
          </div>

          <div
            v-if="log.headers"
            class="detail-group"
          >
            <label class="detail-label">Заголовки:</label>
            <pre class="detail-value code-block">{{ formatJson(log.headers) }}</pre>
          </div>

          <div
            v-if="log.request_body"
            class="detail-group"
          >
            <label class="detail-label">Тело запроса:</label>
            <pre class="detail-value code-block request-body">{{ formatJson(log.request_body) }}</pre>
          </div>

          <div
            v-if="log.response_body"
            class="detail-group"
          >
            <label class="detail-label">Тело ответа:</label>
            <pre class="detail-value code-block response-body">{{ formatJson(log.response_body) }}</pre>
          </div>
        </div>
      </div>
    </div>
    <div
      v-else
      class="no-selection-message"
    >
      <p>Выберите запрос для просмотра деталей</p>
    </div>
  </div>
</template>

<script setup>
import RequestLogBadge from './RequestLogBadge.vue';
import { formatDuration, formatFullDate, formatJson } from '@/utils/requestLogsFormat';

/** Карточка выбранного обращения. Пустой `log` показывает подсказку выбора. */
defineProps({
  log: { type: Object, default: null },
});

defineEmits(['close']);
</script>

<style scoped>
.log-details-section {
  width: 35%;
  overflow-y: auto;
  background: var(--surface-2);
  border-left: 1px solid var(--border);
}

.log-details-content {
  padding: 20px;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.1em;
  font-weight: 600;
}

.close-details-btn {
  background: none;
  border: none;
  font-size: 1.5em;
  cursor: pointer;
  color: var(--text-muted);
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
}

.close-details-btn:hover {
  background-color: var(--border);
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.details-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.8em;
  color: var(--text-muted);
  font-weight: 500;
}

.detail-value {
  font-size: 0.9em;
  color: var(--text);
  word-break: break-all;
}

.path-value {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background: var(--surface-2);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  font-size: 0.85em;
}

.code-block {
  background: var(--surface-2);
  padding: 8px;
  border-radius: var(--radius-sm);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.85em;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.request-body {
  background: var(--info-bg);
  border-left: 3px solid var(--info);
}

.response-body {
  background: var(--success-bg);
  border-left: 3px solid var(--success);
}

.user-id {
  font-size: 10px;
  color: var(--text-muted);
  margin-left: 4px;
}

.no-selection-message {
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

@media (max-width: 1200px) {
  .log-details-section {
    width: 100% !important;
  }
}

@media (max-width: 768px) {
  .log-details-content {
    padding: 16px;
  }

  .code-block {
    font-size: 0.75em;
  }
}
</style>
