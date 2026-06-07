<template>
  <BlacklistTabBase
    search-placeholder="Поиск по номеру или марке..."
    empty-noun="машины"
    :api-list="listVehicleBlacklist"
    :get-primary-text="primaryText"
    :get-detail-rows="detailRows"
    @count="$emit('count', $event)"
  />
</template>

<script>
import BlacklistTabBase from './BlacklistTabBase.vue';
import { listVehicleBlacklist } from '@/api/blacklist';
import { formatDateTime } from '@/utils/datetime';

/**
 * Вкладка "Машины" чёрного списка (#443). Тонкая обёртка над BlacklistTabBase:
 * задаёт API-метод и отображение строки/деталей машины (номер + марка).
 */
export default {
  name: 'VehicleBlacklistTab',
  components: { BlacklistTabBase },
  emits: ['count'],
  methods: {
    listVehicleBlacklist,
    primaryText(item) {
      return [item.car_number, item.mark_name].filter(Boolean).join(' ');
    },
    detailRows(item) {
      return [
        { label: 'Номер', value: item.car_number },
        { label: 'Марка', value: item.mark_name },
        { label: 'Причина', value: item.reason },
        { label: 'Добавлена', value: formatDateTime(item.created_at) },
      ];
    },
  },
};
</script>
