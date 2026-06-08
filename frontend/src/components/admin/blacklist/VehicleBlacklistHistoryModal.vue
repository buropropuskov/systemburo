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
import { getVehicleBlacklistHistory } from '@/api/blacklist';

const ACTION_TEXTS = {
  created: 'Добавлена в чёрный список',
  archived: 'Снята с чёрного списка',
  restored: 'Возвращена в чёрный список',
  updated: 'Изменена причина',
};

const FIELD_LABELS = {
  reason: 'Причина',
  reason_old: 'Было',
  reason_new: 'Стало',
  cars_deactivated: 'Деактивировано машин',
  cars_reactivated: 'Возвращено машин в оборот',
};

/**
 * Модалка истории записи ЧС машин (#443). Тонкая обёртка над
 * BlacklistHistoryModalBase: задаёт заголовок, словари действий/полей и загрузчик.
 */
export default {
  name: 'VehicleBlacklistHistoryModal',
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
      return [this.item.car_number, this.item.mark_name].filter(Boolean).join(' ');
    },
    title() {
      return `История машины «${this.entityLabel}»`;
    },
  },
  methods: {
    loadFn() {
      return getVehicleBlacklistHistory(this.item.id);
    },
  },
};
</script>
