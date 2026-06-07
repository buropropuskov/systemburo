<template>
  <div>
    <BlacklistTabBase
      ref="base"
      search-placeholder="Поиск по ФИО..."
      empty-noun="людей"
      :api-list="listPersonBlacklist"
      :get-primary-text="primaryText"
      :get-detail-rows="detailRows"
      @count="$emit('count', $event)"
      @create="showCreate = true"
      @archive="askArchive"
      @restore="doRestore"
    />
    <BlacklistCreateModal
      :show="showCreate"
      type="person"
      :create-fn="createPersonBlacklist"
      @close="showCreate = false"
      @created="onCreated"
    />
    <ConfirmationModal
      :show="!!archiveItem"
      title="Снятие с чёрного списка"
      :message="archiveMessage"
      confirm-text="Снять"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#dc3545', borderColor: '#dc3545' }"
      @confirm="doArchive"
      @cancel="archiveItem = null"
    />
  </div>
</template>

<script>
import BlacklistTabBase from './BlacklistTabBase.vue';
import BlacklistCreateModal from './BlacklistCreateModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import {
  listPersonBlacklist,
  createPersonBlacklist,
  archivePersonBlacklist,
  restorePersonBlacklist,
} from '@/api/blacklist';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Вкладка "Люди" чёрного списка (#443). Конфигурирует BlacklistTabBase под людей
 * и владеет действиями: создание (модалка), снятие (подтверждение), возврат.
 */
export default {
  name: 'PersonBlacklistTab',
  components: { BlacklistTabBase, BlacklistCreateModal, ConfirmationModal },
  emits: ['count'],
  data() {
    return { showCreate: false, archiveItem: null };
  },
  computed: {
    archiveMessage() {
      return this.archiveItem
        ? `Снять «${this.primaryText(this.archiveItem)}» с чёрного списка? Совпадающие сотрудники с активной заявкой снова станут активными.`
        : '';
    },
  },
  methods: {
    listPersonBlacklist,
    createPersonBlacklist,
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
    async onCreated(name) {
      this.showCreate = false;
      useDeletionsStore().notify({ prefix: 'Человек ', bold: name, suffix: ' добавлен в чёрный список' });
      await this.$refs.base.fetchData();
    },
    askArchive(item) {
      this.archiveItem = item;
    },
    async doArchive() {
      const item = this.archiveItem;
      this.archiveItem = null;
      if (!item) return;
      try {
        await archivePersonBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Человек ', bold: this.primaryText(item), suffix: ' снят с чёрного списка' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось снять: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async doRestore(item) {
      try {
        await restorePersonBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Человек ', bold: this.primaryText(item), suffix: ' возвращён в чёрный список' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось вернуть: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
  },
};
</script>
