<template>
  <div class="col date-col" data-label="Действует до">
    {{ formatDateRu(item.entry_date_to) }}
    <PassageStateIndicator :passage-state="item.passage_state" :expired="passageExpired(item)" />
  </div>
</template>
<script setup>
import PassageStateIndicator from './PassageStateIndicator.vue';
import { formatDateRu } from '@/utils/datetime';
import { passageExpired } from '@/utils/passageProjection';
defineProps({ item: { type: Object, required: true } });
</script>
