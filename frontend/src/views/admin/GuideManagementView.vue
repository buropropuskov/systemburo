<template>
  <AdminPageShell>
    <GuideManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import GuideManagement from '@/components/GuideManagement.vue';

export default {
  name: 'GuideManagementView',
  components: { AdminPageShell, GuideManagement },
};
</script>
