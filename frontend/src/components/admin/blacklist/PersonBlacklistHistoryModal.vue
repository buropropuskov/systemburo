<template>
  <BlacklistHistoryModalBase
    :title="title"
    :entity-label="entityLabel"
    :load-fn="loadFn"
    :action-texts="actionTexts"
    :field-labels="fieldLabels"
    :current-user-name="currentUserName"
    @close="$emit('close')"
  />
</template>

<script>
import BlacklistHistoryModalBase from './BlacklistHistoryModalBase.vue';
import { getPersonBlacklistHistory } from '@/api/blacklist';

const ACTION_TEXTS = {
  created: 'Добавлен в чёрный список',
  archived: 'Снят с чёрного списка',
  restored: 'Возвращён в чёрный список',
};

const FIELD_LABELS = {
  reason: 'Причина',
  employees_deactivated: 'Деактивировано сотрудников',
  employees_reactivated: 'Возвращено сотрудников',
};

/**
 * Модалка истории записи ЧС людей (#443). Тонкая обёртка над
 * BlacklistHistoryModalBase: задаёт заголовок, словари действий/полей и загрузчик.
 */
export default {
  name: 'PersonBlacklistHistoryModal',
  components: { BlacklistHistoryModalBase },
  props: {
    item: { type: Object, required: true },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  computed: {
    actionTexts() {
      return ACTION_TEXTS;
    },
    fieldLabels() {
      return FIELD_LABELS;
    },
    entityLabel() {
      return [this.item.last_name, this.item.first_name, this.item.middle_name].filter(Boolean).join(' ');
    },
    title() {
      return `История человека «${this.entityLabel}»`;
    },
  },
  methods: {
    loadFn() {
      return getPersonBlacklistHistory(this.item.id);
    },
  },
};
</script>
