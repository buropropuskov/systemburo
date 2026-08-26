<template>
  <div>
    <BlacklistTabBase
      ref="base"
      empty-noun="машины"
      entity-icon="car"
      :api-list="listVehicleBlacklist"
      :get-primary-text="primaryText"
      :get-detail-rows="detailRows"
      :lookup-card="lookupCar"
      :bulk-archive-fn="bulkArchiveVehicleBlacklist"
      :bulk-restore-fn="bulkRestoreVehicleBlacklist"
      testid-prefix="vehicle-bl"
      tab-key="vehicles"
      cascade-noun-plural="машины"
      @count="$emit('count', $event)"
      @create="showCreate = true"
      @archive="askArchive"
      @restore="doRestore"
      @purge="askPurge"
      @history-all="showHistory = true"
      @open-card="openCard"
      @edit="onEdit"
    >
      <template #header-left>
        <slot name="header-left" />
      </template>
    </BlacklistTabBase>
    <AddToBlacklistModal
      :show="!!editItem"
      type="vehicle"
      mode="edit"
      :entity-label="editItem ? primaryText(editItem) : ''"
      :initial-reason="editItem ? editItem.reason : ''"
      :initial-entity="editItem || {}"
      :saving="savingEdit"
      :error="editError"
      :z-index="1000"
      @close="editItem = null"
      @confirm="saveEdit"
    />
    <BlacklistCreateModal
      :show="showCreate"
      type="vehicle"
      :create-fn="createVehicleBlacklist"
      @close="showCreate = false"
      @created="onCreated"
    />
    <VehicleBlacklistHistoryModal
      :show="showHistory"
      :current-user-name="currentUserName"
      @close="showHistory = false"
    />
    <ConfirmationModal
      :show="!!archiveItem"
      title="Убрать из чёрного списка"
      :message="archiveMessage"
      confirm-text="Убрать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#dc3545', borderColor: '#dc3545' }"
      @confirm="doArchive"
      @cancel="archiveItem = null"
    />
    <ConfirmationModal
      :show="!!purgeItem"
      title="Удалить запись навсегда"
      :message="purgeMessage"
      confirm-text="Удалить навсегда"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#dc3545', borderColor: '#dc3545' }"
      @confirm="doPurge"
      @cancel="purgeItem = null"
    />
    <VehicleDetailsModal
      :show="showDetails"
      :vehicle="detailsVehicle"
      :show-car-features="true"
      source="blacklist"
      :current-user-name="currentUserName"
      @close="showDetails = false"
    />
    <BlacklistImpactModal
      :show="!!restoreItem"
      :subject="restoreSubject"
      :impact="restoreImpact"
      confirm-label="Вернуть в чёрный список"
      @confirm="confirmRestore"
      @close="restoreItem = null"
    />
  </div>
</template>

<script>
import BlacklistTabBase from './BlacklistTabBase.vue';
import BlacklistCreateModal from './BlacklistCreateModal.vue';
import BlacklistImpactModal from './BlacklistImpactModal.vue';
import VehicleBlacklistHistoryModal from './VehicleBlacklistHistoryModal.vue';
import AddToBlacklistModal from './AddToBlacklistModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import VehicleDetailsModal from '@/components/CreateApplication/VehicleDetailsModal.vue';
import {
  listVehicleBlacklist,
  createVehicleBlacklist,
  updateVehicleBlacklist,
  archiveVehicleBlacklist,
  restoreVehicleBlacklist,
  purgeVehicleBlacklist,
  bulkArchiveVehicleBlacklist,
  bulkRestoreVehicleBlacklist,
  vehicleBlacklistImpact,
} from '@/api/blacklist';
import { lookupUniqueCar } from '@/api/cars';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Вкладка "Машины" чёрного списка (#443). Конфигурирует BlacklistTabBase под машины
 * и владеет действиями: создание (модалка), снятие (подтверждение), возврат.
 */
export default {
  name: 'VehicleBlacklistTab',
  components: { BlacklistTabBase, BlacklistCreateModal, BlacklistImpactModal, VehicleBlacklistHistoryModal, AddToBlacklistModal, ConfirmationModal, VehicleDetailsModal },
  props: {
    currentUserName: { type: String, default: '' },
  },
  emits: ['count'],
  data() {
    return {
      showCreate: false, archiveItem: null, showHistory: false, detailsVehicle: null, showDetails: false,
      editItem: null, savingEdit: false, editError: '', purgeItem: null,
      restoreItem: null, restoreSubject: '', restoreImpact: { matches: 0, tables: [], rows: [] },
    };
  },
  computed: {
    archiveMessage() {
      return this.archiveItem
        ? `Убрать «${this.primaryText(this.archiveItem)}» из чёрного списка? Совпадающие машины с активной заявкой снова станут активными.`
        : '';
    },
    purgeMessage() {
      return this.purgeItem
        ? `Удалить «${this.primaryText(this.purgeItem)}» из архива навсегда? Запись исчезнет, но событие останется в общей истории чёрного списка.`
        : '';
    },
  },
  methods: {
    listVehicleBlacklist,
    createVehicleBlacklist,
    bulkArchiveVehicleBlacklist,
    bulkRestoreVehicleBlacklist,
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
    // Лукап машины в реестре по номеру+марке записи ЧС -> объект для VehicleDetailsModal
    // (как CarsView.openCarDetails). null, если машины в реестре нет (кнопка останется disabled).
    async lookupCar(item) {
      const car = await lookupUniqueCar({ number: item.car_number, mark: item.mark_name });
      if (!car) return null;
      return {
        id: car.id,
        plateNumber: car.number,
        mark: car.mark,
        formatId: car.format_id || null,
        organization: car.organization_name || null,
        organizationId: car.organization_id || null,
        company: car.company_name || null,
        companyId: car.company_id || null,
        isExisting: true,
        unloadPlaces: [],
        isActive: car.status,
      };
    },
    openCard(vehicle) {
      if (!vehicle) return;
      this.detailsVehicle = vehicle;
      this.showDetails = true;
    },
    async onCreated(name) {
      this.showCreate = false;
      useDeletionsStore().notify({ prefix: 'Машина ', bold: name, suffix: ' добавлена в чёрный список' });
      await this.$refs.base.fetchData();
    },
    onEdit(item) {
      this.editError = '';
      this.editItem = item;
    },
    async saveEdit(payload) {
      const item = this.editItem;
      if (!item || this.savingEdit) return;
      this.savingEdit = true;
      this.editError = '';
      try {
        const updated = await updateVehicleBlacklist(item.id, payload);
        useDeletionsStore().notify({ prefix: 'Запись обновлена: ', bold: this.primaryText(updated || item) });
        this.editItem = null;
        await this.$refs.base.fetchData();
      } catch (e) {
        this.editError = e?.message || 'Не удалось сохранить';
      } finally {
        this.savingEdit = false;
      }
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
        useDeletionsStore().notify({ prefix: 'Машина ', bold: this.primaryText(item), suffix: ' убрана из чёрного списка' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось убрать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    /**
     * Возврат из архива деактивирует совпадающие машины так же, как первичное
     * внесение, поэтому и предупреждение то же: сначала показываем, где запись
     * сейчас есть. Сбой предпросмотра возврату не мешает - он вспомогательный.
     */
    async doRestore(item) {
      try {
        const impact = await vehicleBlacklistImpact({
          carNumber: item.car_number,
          markId: item.mark_id,
        });
        if (impact && impact.matches > 0) {
          this.restoreItem = item;
          this.restoreImpact = impact;
          this.restoreSubject = this.primaryText(item);
          return;
        }
      } catch (e) {
        console.warn('Предпросмотр последствий возврата не удался, продолжаем', e);
      }
      await this.persistRestore(item);
    },

    /** Подтверждение из окна последствий возврата. */
    confirmRestore() {
      const item = this.restoreItem;
      this.restoreItem = null;
      if (item) this.persistRestore(item);
    },

    async persistRestore(item) {
      try {
        await restoreVehicleBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Машина ', bold: this.primaryText(item), suffix: ' возвращена в чёрный список' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось вернуть: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    askPurge(item) {
      this.purgeItem = item;
    },
    async doPurge() {
      const item = this.purgeItem;
      this.purgeItem = null;
      if (!item) return;
      try {
        await purgeVehicleBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Машина ', bold: this.primaryText(item), suffix: ' удалена навсегда' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось удалить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
  },
};
</script>
