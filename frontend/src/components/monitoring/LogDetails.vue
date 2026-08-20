<template>
  <BaseModal
    :show="Boolean(log)"
    title="Детали запроса"
    width="640px"
    radius="30px"
    content-class="log-details-modal"
    @close="$emit('close')"
  >
    <dl
      v-if="log"
      class="details-grid"
    >
      <div class="detail-group">
        <dt class="detail-label">
          ID запроса
        </dt>
        <dd class="detail-value">
          {{ log.id }}
        </dd>
      </div>

      <div class="detail-group">
        <dt class="detail-label">
          Время
        </dt>
        <dd class="detail-value">
          {{ formatFullDate(log.created_at) }}
        </dd>
      </div>

      <div class="detail-group">
        <dt class="detail-label">
          Метод
        </dt>
        <dd class="detail-value">
          <RequestLogBadge
            kind="method"
            :value="log.method"
            variant="detail"
          />
        </dd>
      </div>

      <div class="detail-group">
        <dt class="detail-label">
          Статус ответа
        </dt>
        <dd class="detail-value">
          <RequestLogBadge
            kind="status"
            :value="log.response_status"
            variant="detail"
          />
        </dd>
      </div>

      <div class="detail-group">
        <dt class="detail-label">
          Время выполнения
        </dt>
        <dd class="detail-value">
          {{ formatDuration(log) }}
        </dd>
      </div>

      <div class="detail-group">
        <dt class="detail-label">
          Пользователь
        </dt>
        <dd class="detail-value">
          {{ log.username || 'Аноним' }}
          <span
            v-if="log.user_id"
            class="user-id"
          >(ID: {{ log.user_id }})</span>
        </dd>
      </div>

      <div class="detail-group detail-group--wide">
        <dt class="detail-label">
          URL
        </dt>
        <dd class="detail-value path-value">
          {{ log.url }}
        </dd>
      </div>
    </dl>
  </BaseModal>
</template>

<script setup>
import BaseModal from '@/components/ui/BaseModal.vue';
import RequestLogBadge from './RequestLogBadge.vue';
import { formatDuration, formatFullDate } from '@/utils/requestLogsFormat';

/** Окно выбранного обращения. Пустой `log` держит окно закрытым. */
defineProps({
  log: { type: Object, default: null },
});

defineEmits(['close']);
</script>

<style scoped>
.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin: 0;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.detail-group--wide {
  grid-column: 1 / -1;
}

.detail-label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.detail-value {
  margin: 0;
  font-size: 14px;
  color: var(--text);
  word-break: break-word;
}

.path-value {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  background: var(--surface-2);
  padding: 8px 10px;
  border-radius: var(--radius-md);
  font-size: 13px;
  word-break: break-all;
}

.user-id {
  font-size: 11px;
  color: var(--text-muted);
  margin-left: 4px;
}

@media (max-width: 768px) {
  .details-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;
  }
}
</style>
