<template>
  <AdminPageShell>
    <CitizenshipManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import CitizenshipManagement from '@/components/CitizenshipManagement.vue';

export default {
  name: 'CitizenshipView',
  components: { AdminPageShell, CitizenshipManagement },
};
</script>
