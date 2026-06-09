<template>
  <section class="admin-page">
    <MarksManagement />
  </section>
</template>

<script>
import MarksManagement from '@/components/MarksManagement.vue';

export default {
  name: 'MarksView',
  components: { MarksManagement },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
