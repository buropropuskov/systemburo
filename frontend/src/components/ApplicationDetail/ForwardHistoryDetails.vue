<template>
  <div class="forward-detail">
    <div class="forward-label">Получатели</div>
    <div
      v-for="recipient in recipients"
      :key="recipient.user_id"
      class="forward-recipient"
    >
      <strong>{{ recipient.display_name }}</strong><span> — {{ forwardRecipientPurpose(recipient) }}</span>
    </div>
    <div
      v-if="!recipients.length"
      class="forward-legacy"
    >
      {{ FORWARD_RECIPIENTS_UNAVAILABLE }}
    </div>
    <div class="forward-materials">
      <span class="forward-label">Материалы: </span>{{ forwardMaterialsText(metadata) }}
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { forwardRecipients, forwardRecipientPurpose, forwardMaterialsText, FORWARD_RECIPIENTS_UNAVAILABLE } from '@/utils/applicationForwardHistory';

const props = defineProps({ metadata: { type: Object, default: null } });
const recipients = computed(() => forwardRecipients(props.metadata));
</script>

<style scoped>
.forward-detail {
    margin-top: 12px;
    padding: 12px 14px;
    background: var(--surface-2);
    border-radius: var(--radius-md);
    font-size: 13px;
    line-height: 1.6;
    overflow-wrap: anywhere;
}

.forward-label {
    font-weight: 600;
    color: var(--text-muted);
}

.forward-recipient {
    margin: 6px 0;
}

.forward-materials {
    margin-top: 10px;
}

.forward-legacy {
    color: var(--text-muted);
}
</style>
