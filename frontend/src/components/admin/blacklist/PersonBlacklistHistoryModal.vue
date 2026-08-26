<template>
  <BlacklistHistoryModalBase
    :show="show"
    title="История чёрного списка людей"
    entity-label="lyudi"
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
import { getAllPersonBlacklistHistory } from '@/api/blacklist';

const ACTION_TEXTS = {
  created: 'Добавлен в чёрный список',
  archived: 'Снят с чёрного списка',
  restored: 'Возвращён в чёрный список',
  updated: 'Изменена причина',
  purged: 'Удалён навсегда',
};

const FIELD_LABELS = {
  reason: 'Причина',
  reason_old: 'Было',
  reason_new: 'Стало',
  employees_deactivated: 'Деактивировано сотрудников',
  employees_reactivated: 'Возвращено сотрудников',
};

// ФИО и причина-диф рендерятся отдельными блоками - не дублируем их в комментарии.
const COMMENT_EXCLUDE_KEYS = ['full_name', 'reason_old', 'reason_new'];

/**
 * Модалка общего журнала ЧС людей (#443): все события всех записей, включая физически
 * удалённые (ФИО берётся из details, а не из самой записи). Тонкая обёртка над
 * BlacklistHistoryModalBase.
 */
export default {
  name: 'PersonBlacklistHistoryModal',
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
      return getAllPersonBlacklistHistory();
    },
    entityLabelFn(historyItem) {
      const d = historyItem.details || {};
      return d.full_name || '';
    },
  },
};
</script>
