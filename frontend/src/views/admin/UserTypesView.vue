<template>
  <AdminPageShell>
    <UserTypes />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import UserTypes from '@/components/UserTypes.vue';

export default {
  name: 'UserTypesView',
  components: { AdminPageShell, UserTypes },
};
</script>
