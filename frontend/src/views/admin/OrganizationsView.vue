<template>
  <AdminPageShell>
    <OrganizationsManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import OrganizationsManagement from '@/components/OrganizationsManagement.vue';

export default {
  name: 'OrganizationsView',
  components: { AdminPageShell, OrganizationsManagement },
};
</script>
