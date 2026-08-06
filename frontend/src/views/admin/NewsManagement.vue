<template>
  <AdminPageShell>
    <NewsManagementComponent data-testid="ob-admin-news" />
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
