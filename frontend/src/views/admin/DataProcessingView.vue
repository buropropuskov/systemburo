<template>
  <AdminPageShell>
    <DataProcessingSettings />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import DataProcessingSettings from '@/components/admin/DataProcessingSettings.vue';

/**
 * Страница /admin/data-processing: хостит раздел обработки данных отдельно от
 * настроек системы. Настройки - это параметры работы (загрузки, пагинация,
 * уведомления), а здесь механизм согласия со своими данными о людях.
 */
export default {
  name: 'DataProcessingView',
  components: { AdminPageShell, DataProcessingSettings },
};
</script>
