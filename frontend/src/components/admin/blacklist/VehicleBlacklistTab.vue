<template>
  <div>
    <BlacklistTabBase
      ref="base"
      empty-noun="машины"
      :entity-icon="carIcon"
      :api-list="listVehicleBlacklist"
      :get-primary-text="primaryText"
      :get-detail-rows="detailRows"
      @count="$emit('count', $event)"
      @create="showCreate = true"
      @archive="askArchive"
      @restore="doRestore"
      @history="historyItem = $event"
    >
      <template #header-left>
        <slot name="header-left" />
      </template>
    </BlacklistTabBase>
    <BlacklistCreateModal
      :show="showCreate"
      type="vehicle"
      :create-fn="createVehicleBlacklist"
      @close="showCreate = false"
      @created="onCreated"
    />
    <VehicleBlacklistHistoryModal
      v-if="historyItem"
      :item="historyItem"
      :current-user-name="currentUserName"
      @close="historyItem = null"
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
import VehicleBlacklistHistoryModal from './VehicleBlacklistHistoryModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import {
  listVehicleBlacklist,
  createVehicleBlacklist,
  archiveVehicleBlacklist,
  restoreVehicleBlacklist,
} from '@/api/blacklist';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';
import carIcon from '@/assets/icons/car.png';

/**
 * Вкладка "Машины" чёрного списка (#443). Конфигурирует BlacklistTabBase под машины
 * и владеет действиями: создание (модалка), снятие (подтверждение), возврат.
 */
export default {
  name: 'VehicleBlacklistTab',
  components: { BlacklistTabBase, BlacklistCreateModal, VehicleBlacklistHistoryModal, ConfirmationModal },
  props: {
    currentUserName: { type: String, default: '' },
  },
  emits: ['count'],
  data() {
    return { showCreate: false, archiveItem: null, historyItem: null, carIcon };
  },
  computed: {
    archiveMessage() {
      return this.archiveItem
        ? `Снять «${this.primaryText(this.archiveItem)}» с чёрного списка? Совпадающие машины с активной заявкой снова станут активными.`
        : '';
    },
  },
  methods: {
    listVehicleBlacklist,
    createVehicleBlacklist,
    primaryText(item) {
      return [item.car_number, item.mark_name].filter(Boolean).join(' ');
    },
    detailRows(item) {
      return [
        { label: 'Номер', value: item.car_number },
        { label: 'Марка', value: item.mark_name },
        { label: 'Причина', value: item.reason, kind: 'reason' },
        { label: 'Добавлена', value: formatDateTime(item.created_at) },
      ];
    },
    async onCreated(name) {
      this.showCreate = false;
      useDeletionsStore().notify({ prefix: 'Машина ', bold: name, suffix: ' добавлена в чёрный список' });
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
        await archiveVehicleBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Машина ', bold: this.primaryText(item), suffix: ' снята с чёрного списка' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось снять: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async doRestore(item) {
      try {
        await restoreVehicleBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Машина ', bold: this.primaryText(item), suffix: ' возвращена в чёрный список' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось вернуть: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
  },
};
</script>
