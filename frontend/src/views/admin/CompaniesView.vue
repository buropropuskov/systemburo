<template>
  <AdminPageShell>
    <CompaniesManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import CompaniesManagement from '@/components/CompaniesManagement.vue';

export default {
  name: 'CompaniesView',
  components: { AdminPageShell, CompaniesManagement },
};
</script>
