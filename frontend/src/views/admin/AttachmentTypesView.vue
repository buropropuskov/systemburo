<template>
  <section class="admin-page">
    <AttachmentsManagement />
  </section>
</template>

<script>
import AttachmentsManagement from '@/components/AttachmentsManagement.vue';

export default {
  name: 'AttachmentTypesView',
  components: { AttachmentsManagement },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
