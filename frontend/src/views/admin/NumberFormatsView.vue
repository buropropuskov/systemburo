<template>
  <AdminPageShell>
    <NumberFormat />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import NumberFormat from '@/components/NumberFormat.vue';

export default {
  name: 'NumberFormatsView',
  components: { AdminPageShell, NumberFormat },
};
</script>
