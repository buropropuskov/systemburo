<template>
  <div
    class="info-grid"
    data-testid="ob-detail-basic"
  >
    <div class="info-row">
      <span class="info-label">Организация / Отдел:</span>
      <span class="info-value">{{ application.organization_name }}</span>
    </div>
    <div
      v-if="application.company_name"
      class="info-row"
    >
      <span class="info-label">Компания:</span>
      <span class="info-value">{{ application.company_name }}</span>
    </div>
    <div class="info-row">
      <span class="info-label">Отправитель:</span>
      <span class="info-value sender-value">
        <span>{{ application.sender_full_name || application.sender_name }}</span>
        <Badge
          v-if="application.sender_is_important"
          variant="info"
          size="sm"
          class="sender-important-tag"
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          ><polygon points="12 2 15 8.6 22 9.3 16.8 14 18.3 21 12 17.3 5.7 21 7.2 14 2 9.3 9 8.6" /></svg>
          Важный
        </Badge>
      </span>
    </div>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';

/**
 * «Основная информация» карточки заявки: чья заявка и кто её подал.
 *
 * Вынесена из ApplicationDetail - его шаблон давно за порогом lint:size, а туру
 * понадобился якорь на этот блок. Заодно блок перестал тонуть в тысяче строк
 * карточки.
 */
export default {
  name: 'ApplicationBasicInfo',
  components: { Badge },
  props: {
    application: { type: Object, required: true },
  },
};
</script>

<style scoped>
.info-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.info-row {
    display: flex;
    gap: 8px;
    font-size: 13px;
    line-height: 1.4;
}

.info-label {
    color: var(--text-muted);
    flex-shrink: 0;
}

.info-value {
    color: var(--text);
    overflow-wrap: anywhere;
}

.sender-value {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
}

.sender-important-tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}
</style>
