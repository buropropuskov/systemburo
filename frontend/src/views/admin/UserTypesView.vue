<template>
  <section class="admin-page">
    <UserTypes />
  </section>
</template>

<script>
import UserTypes from '@/components/UserTypes.vue';

export default {
  name: 'UserTypesView',
  components: { UserTypes },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
