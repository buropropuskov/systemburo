<template>
  <BlacklistTabBase
    search-placeholder="Поиск по ФИО..."
    empty-noun="людей"
    :api-list="listPersonBlacklist"
    :get-primary-text="primaryText"
    :get-detail-rows="detailRows"
    @count="$emit('count', $event)"
  />
</template>

<script>
import BlacklistTabBase from './BlacklistTabBase.vue';
import { listPersonBlacklist } from '@/api/blacklist';
import { formatDateTime } from '@/utils/datetime';

/**
 * Вкладка "Люди" чёрного списка (#443). Тонкая обёртка над BlacklistTabBase:
 * задаёт API-метод и отображение строки/деталей человека (ФИО).
 */
export default {
  name: 'PersonBlacklistTab',
  components: { BlacklistTabBase },
  emits: ['count'],
  methods: {
    listPersonBlacklist,
    primaryText(item) {
      return [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ');
    },
    detailRows(item) {
      return [
        { label: 'Фамилия', value: item.last_name },
        { label: 'Имя', value: item.first_name },
        { label: 'Отчество', value: item.middle_name },
        { label: 'Причина', value: item.reason },
        { label: 'Добавлен', value: formatDateTime(item.created_at) },
      ];
    },
  },
};
</script>
