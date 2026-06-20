<template>
  <AdminPageShell>
    <NewsManagementComponent />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import NewsManagementComponent from '@/components/NewsManagement.vue';

export default {
  name: 'NewsManagementView',
  components: { AdminPageShell, NewsManagementComponent },
};
</script>
