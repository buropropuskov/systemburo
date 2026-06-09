<template>
  <section class="admin-page">
    <OrganizationsManagement />
  </section>
</template>

<script>
import OrganizationsManagement from '@/components/OrganizationsManagement.vue';

export default {
  name: 'OrganizationsView',
  components: { OrganizationsManagement },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
