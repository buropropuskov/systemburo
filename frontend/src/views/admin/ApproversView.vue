<template>
  <AdminPageShell>
    <ApplicationApprovers />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import ApplicationApprovers from '@/components/ApplicationApprovers.vue';

export default {
  name: 'ApproversView',
  components: { AdminPageShell, ApplicationApprovers },
};
</script>
