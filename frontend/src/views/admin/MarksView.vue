<template>
  <AdminPageShell>
    <MarksManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import MarksManagement from '@/components/MarksManagement.vue';

export default {
  name: 'MarksView',
  components: { AdminPageShell, MarksManagement },
};
</script>
