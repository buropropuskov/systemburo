<template>
    <div class="attachment-header-section">
      <ApplicationAttachmentHeading :attachment="attachment" />
      <ApplicationChangeAttention v-if="removalChange" :change="removalChange" :active="activeKey === removalChange.key" :busy="busy" text="Состав заявки изменён" @acknowledge="emit('acknowledge', $event)" @focus-change="emit('focus-change', $event)">
        <Badge variant="neutral" size="sm">Состав изменён</Badge>
      </ApplicationChangeAttention>

      <!-- Даты действия -->
      <div
        v-if="attachment.entry_date_from || attachment.entry_date_to"
        class="date-range"
      >
        <span class="date-label">Срок действия:</span>
        <span class="date-value">
          {{ dateRange }}
        </span>
      </div>

      <!-- Время действия -->
      <div
        v-if="attachment.entry_time_from || attachment.entry_time_to"
        class="time-range"
      >
        <span class="time-label">Время:</span>
        <span class="time-value">
          {{ timeRange }}
        </span>
      </div>
    </div>

</template>
<script setup>
import { computed } from 'vue';
import ApplicationAttachmentHeading from './ApplicationAttachmentHeading.vue';
import ApplicationChangeAttention from './ApplicationChangeAttention.vue';
import Badge from '@/components/ui/Badge.vue';
const props = defineProps({ attachment: { type: Object, required: true }, changes: { type: Array, default: () => [] }, dateRange: String, timeRange: String, activeKey: String, busy: Boolean });
const emit = defineEmits(['acknowledge', 'focus-change']);
const removalChange = computed(() => props.changes.find(change => change.field === 'removed') || null);
</script>
<style scoped>
.attachment-header-section {
    padding: 15px;
    border-bottom: 1px solid var(--color-border);
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px 24px;
}

.date-range, .time-range {
    display: flex;
    flex-direction: column;
    gap: 0px;
    font-size: 14px;
}

.date-range:last-child, .time-range:last-child {
    margin-bottom: 0;
}

.date-label, .time-label {
    color: var(--text-muted);
    font-weight: 400;
    min-width: 110px;
    font-size: 14px;
}

.date-value, .time-value {
    color: var(--text);
    font-weight: 400;
    font-size: 15px;
}


</style>
