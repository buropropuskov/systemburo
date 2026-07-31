<template>
  <AdminPageShell>
    <FileArchiveManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import FileArchiveManagement from '@/components/admin/FileArchive/FileArchiveManagement.vue';

export default {
  name: 'FileArchiveView',
  components: { AdminPageShell, FileArchiveManagement },
};
</script>
