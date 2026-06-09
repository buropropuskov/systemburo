<template>
  <section class="admin-page">
    <CitizenshipManagement />
  </section>
</template>

<script>
import CitizenshipManagement from '@/components/CitizenshipManagement.vue';

export default {
  name: 'CitizenshipView',
  components: { CitizenshipManagement },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
