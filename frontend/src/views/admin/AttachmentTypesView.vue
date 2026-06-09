<template>
  <AdminPageShell>
    <AttachmentsManagement />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import AttachmentsManagement from '@/components/AttachmentsManagement.vue';

export default {
  name: 'AttachmentTypesView',
  components: { AdminPageShell, AttachmentsManagement },
};
</script>
