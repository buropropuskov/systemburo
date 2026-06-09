<template>
  <section class="admin-page">
    <ApplicationApprovers />
  </section>
</template>

<script>
import ApplicationApprovers from '@/components/ApplicationApprovers.vue';

export default {
  name: 'ApproversView',
  components: { ApplicationApprovers },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
