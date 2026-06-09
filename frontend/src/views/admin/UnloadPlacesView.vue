<template>
  <AdminPageShell>
    <UnloadPlacesContainer />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import UnloadPlacesContainer from '@/components/UnloadPlaces/UnloadPlacesContainer.vue';

export default {
  name: 'UnloadPlacesView',
  components: { AdminPageShell, UnloadPlacesContainer },
};
</script>
