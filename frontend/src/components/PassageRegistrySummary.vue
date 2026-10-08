<template>
  <header>
    <PassageTableTools :kind="kind" source="admin_summary" />
    <slot />
  </header>
</template>
<script setup>
import PassageTableTools from './PassageTableTools.vue';
defineProps({ kind: { type: String, required: true } });
</script>
