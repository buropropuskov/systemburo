<template>
  <section class="admin-page">
    <NumberFormat />
  </section>
</template>

<script>
import NumberFormat from '@/components/NumberFormat.vue';

export default {
  name: 'NumberFormatsView',
  components: { NumberFormat },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
