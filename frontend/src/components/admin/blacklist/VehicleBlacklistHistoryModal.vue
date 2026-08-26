<template>
  <BlacklistHistoryModalBase
    :show="show"
    title="История чёрного списка машин"
    entity-label="mashiny"
    :load-fn="loadFn"
    :action-texts="actionTexts"
    :field-labels="fieldLabels"
    :entity-label-fn="entityLabelFn"
    :comment-exclude-keys="commentExcludeKeys"
    :current-user-name="currentUserName"
    @close="$emit('close')"
  />
</template>

<script>
import BlacklistHistoryModalBase from './BlacklistHistoryModalBase.vue';
import { getAllVehicleBlacklistHistory } from '@/api/blacklist';

const ACTION_TEXTS = {
  created: 'Добавлена в чёрный список',
  archived: 'Снята с чёрного списка',
  restored: 'Возвращена в чёрный список',
  updated: 'Изменена причина',
  purged: 'Удалена навсегда',
};

const FIELD_LABELS = {
  reason: 'Причина',
  reason_old: 'Было',
  reason_new: 'Стало',
  cars_deactivated: 'Деактивировано машин',
  cars_reactivated: 'Возвращено машин в оборот',
};

// Лейбл машины и причина-диф рендерятся отдельными блоками - не дублируем их в комментарии.
const COMMENT_EXCLUDE_KEYS = ['car_number', 'mark_name', 'reason_old', 'reason_new'];

/**
 * Модалка общего журнала ЧС машин (#443): все события всех записей, включая физически
 * удалённые (лейбл машины берётся из details, а не из самой записи). Тонкая обёртка над
 * BlacklistHistoryModalBase.
 */
export default {
  name: 'VehicleBlacklistHistoryModal',
  components: { BlacklistHistoryModalBase },
  props: {
    show: { type: Boolean, default: false },
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
    commentExcludeKeys() {
      return COMMENT_EXCLUDE_KEYS;
    },
  },
  methods: {
    loadFn() {
      return getAllVehicleBlacklistHistory();
    },
    entityLabelFn(historyItem) {
      const d = historyItem.details || {};
      return [d.car_number, d.mark_name].filter(Boolean).join(' ');
    },
  },
};
</script>
