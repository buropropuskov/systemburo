<template>
  <section class="admin-page">
    <CompaniesManagement />
  </section>
</template>

<script>
import CompaniesManagement from '@/components/CompaniesManagement.vue';

export default {
  name: 'CompaniesView',
  components: { CompaniesManagement },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
