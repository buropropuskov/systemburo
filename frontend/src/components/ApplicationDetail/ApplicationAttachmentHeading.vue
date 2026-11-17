<template>
  <div class="attachment-title-row">
    <h4 class="attachment-title">
      <span
        class="attachment-name"
        :title="attachment.attachment_display_name"
      >{{ attachment.attachment_display_name }}</span>
      <span
        v-if="timePeriod"
        class="attachment-period"
        :class="{ 'attachment-period--mixed': timePeriod === 'mixed' }"
        role="img"
        :aria-label="periodLabels[timePeriod]"
        :title="periodLabels[timePeriod]"
      >
        <svg
          v-if="timePeriod !== 'night'"
          class="attachment-period-sun"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          aria-hidden="true"
        >
          <circle
            cx="12"
            cy="12"
            r="4"
          />
          <path d="M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2" />
        </svg>
        <span
          v-if="timePeriod === 'mixed'"
          class="attachment-period-divider"
          aria-hidden="true"
        />
        <svg
          v-if="timePeriod !== 'day'"
          class="attachment-period-moon"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M20.3 15.2A8.5 8.5 0 0 1 8.8 3.7 8.6 8.6 0 1 0 20.3 15.2Z" />
        </svg>
      </span>
    </h4>
    <div
      v-if="attachment.roof_access || attachment.free_parking"
      class="attachment-tags"
    >
      <Badge
        v-if="attachment.roof_access"
        variant="primary"
        size="lg"
      >
        Крыша
      </Badge>
      <Badge
        v-if="attachment.free_parking"
        variant="warning"
        size="lg"
      >
        Парковка
      </Badge>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import Badge from '@/components/ui/Badge.vue'
import { getAttachmentTimePeriod } from '@/utils/attachmentTimePeriod'

const props = defineProps({
  attachment: { type: Object, required: true },
})

const timePeriod = computed(() => getAttachmentTimePeriod(props.attachment.entry_time_from, props.attachment.entry_time_to))
const periodLabels = {
  day: 'Время допуска: день',
  night: 'Время допуска: ночь',
  mixed: 'Время допуска: день и ночь',
}
</script>

<style scoped>
.attachment-title-row {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.attachment-title {
  font-size: 18px;
  color: var(--accent-text);
  font-weight: 700;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.attachment-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.attachment-period {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  gap: 3px;
  width: 30px;
  height: 22px;
  color: var(--text-muted);
  border: 1px solid var(--color-border);
  border-radius: 999px;
}

.attachment-period--mixed {
  width: 52px;
}

.attachment-period svg {
  width: 16px;
  height: 16px;
}

.attachment-period-sun {
  color: var(--warning-text);
}

.attachment-period-moon {
  color: var(--accent-text);
}

.attachment-period-divider {
  width: 1px;
  height: 10px;
  background: currentColor;
  transform: rotate(24deg);
}

.attachment-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}
</style>
