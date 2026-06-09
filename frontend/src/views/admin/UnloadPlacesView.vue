<template>
  <section class="admin-page">
    <UnloadPlacesContainer />
  </section>
</template>

<script>
import UnloadPlacesContainer from '@/components/UnloadPlaces/UnloadPlacesContainer.vue';

export default {
  name: 'UnloadPlacesView',
  components: { UnloadPlacesContainer },
};
</script>

<style scoped>
.admin-page {
  padding: 16px;
  height: 100%;
}
</style>
