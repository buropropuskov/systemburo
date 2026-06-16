<template>
  <AdminPageShell>
    <DocumentsManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import DocumentsManagement from '@/components/DocumentsManagement.vue';

export default {
  name: 'DocumentsView',
  components: { AdminPageShell, DocumentsManagement },
};
</script>
