<template>
  <div>
    <BlacklistTabBase
      ref="base"
      empty-noun="людей"
      entity-icon="user"
      :api-list="listPersonBlacklist"
      :get-primary-text="primaryText"
      :get-detail-rows="detailRows"
      :lookup-card="lookupPerson"
      :bulk-archive-fn="bulkArchivePersonBlacklist"
      :bulk-restore-fn="bulkRestorePersonBlacklist"
      testid-prefix="person-bl"
      tab-key="persons"
      cascade-noun-plural="сотрудники"
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
      type="person"
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
      type="person"
      :create-fn="createPersonBlacklist"
      @close="showCreate = false"
      @created="onCreated"
    />
    <PersonBlacklistHistoryModal
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
    <EmployeeDetailsModal
      :show="showDetails"
      :employee="detailsEmployee"
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
import PersonBlacklistHistoryModal from './PersonBlacklistHistoryModal.vue';
import AddToBlacklistModal from './AddToBlacklistModal.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import EmployeeDetailsModal from '@/components/CreateApplication/EmployeeDetailsModal.vue';
import {
  listPersonBlacklist,
  createPersonBlacklist,
  updatePersonBlacklist,
  archivePersonBlacklist,
  restorePersonBlacklist,
  personBlacklistImpact,
  purgePersonBlacklist,
  bulkArchivePersonBlacklist,
  bulkRestorePersonBlacklist,
} from '@/api/blacklist';
import { lookupUniqueEmployee } from '@/api/employees';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Вкладка "Люди" чёрного списка (#443). Конфигурирует BlacklistTabBase под людей
 * и владеет действиями: создание (модалка), снятие (подтверждение), возврат.
 */
export default {
  name: 'PersonBlacklistTab',
  components: { BlacklistTabBase, BlacklistCreateModal, BlacklistImpactModal, PersonBlacklistHistoryModal, AddToBlacklistModal, ConfirmationModal, EmployeeDetailsModal },
  props: {
    currentUserName: { type: String, default: '' },
  },
  emits: ['count'],
  data() {
    return {
      showCreate: false, archiveItem: null, showHistory: false, detailsEmployee: null, showDetails: false,
      editItem: null, savingEdit: false, editError: '', purgeItem: null,
      restoreItem: null, restoreSubject: '', restoreImpact: { matches: 0, tables: [], rows: [] },
    };
  },
  computed: {
    archiveMessage() {
      return this.archiveItem
        ? `Убрать «${this.primaryText(this.archiveItem)}» из чёрного списка? Совпадающие сотрудники с активной заявкой снова станут активными.`
        : '';
    },
    purgeMessage() {
      return this.purgeItem
        ? `Удалить «${this.primaryText(this.purgeItem)}» из архива навсегда? Запись исчезнет, но событие останется в общей истории чёрного списка.`
        : '';
    },
  },
  methods: {
    listPersonBlacklist,
    createPersonBlacklist,
    bulkArchivePersonBlacklist,
    bulkRestorePersonBlacklist,
    primaryText(item) {
      return [item.last_name, item.first_name, item.middle_name].filter(Boolean).join(' ');
    },
    detailRows(item) {
      return [
        { label: 'Фамилия', value: item.last_name },
        { label: 'Имя', value: item.first_name },
        { label: 'Отчество', value: item.middle_name },
        { label: 'Причина', value: item.reason, kind: 'reason' },
        { label: 'Добавлен', value: formatDateTime(item.created_at) },
      ];
    },
    // Лукап сотрудника в реестре по ФИО записи ЧС -> объект для EmployeeDetailsModal
    // (как EmployeeView.openEmployeeDetails). null, если в реестре нет (кнопка disabled).
    async lookupPerson(item) {
      const emp = await lookupUniqueEmployee({
        last_name: item.last_name,
        first_name: item.first_name,
        middle_name: item.middle_name || '',
      });
      if (!emp) return null;
      return {
        id: emp.id,
        last_name: emp.last_name,
        first_name: emp.first_name,
        middle_name: emp.middle_name,
        position: emp.position,
        citizenshipName: emp.citizenship_name,
        passport_series_number: emp.passport_series_number,
        patent_number: emp.patent_number,
        other_permission: emp.other_permission,
        organization: emp.organization_name || null,
        company: emp.company_name || null,
        target_tables: [],
      };
    },
    openCard(employee) {
      if (!employee) return;
      this.detailsEmployee = employee;
      this.showDetails = true;
    },
    async onCreated(name) {
      this.showCreate = false;
      useDeletionsStore().notify({ prefix: 'Человек ', bold: name, suffix: ' добавлен в чёрный список' });
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
        const updated = await updatePersonBlacklist(item.id, payload);
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
        await archivePersonBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Человек ', bold: this.primaryText(item), suffix: ' убран из чёрного списка' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось убрать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    /**
     * Возврат из архива деактивирует совпадающих работников так же, как первичное
     * внесение, поэтому и предупреждение то же - см. VehicleBlacklistTab.
     */
    async doRestore(item) {
      try {
        const impact = await personBlacklistImpact({
          lastName: item.last_name,
          firstName: item.first_name,
          middleName: item.middle_name || '',
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

    confirmRestore() {
      const item = this.restoreItem;
      this.restoreItem = null;
      if (item) this.persistRestore(item);
    },

    async persistRestore(item) {
      try {
        await restorePersonBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Человек ', bold: this.primaryText(item), suffix: ' возвращён в чёрный список' });
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
        await purgePersonBlacklist(item.id);
        useDeletionsStore().notify({ prefix: 'Человек ', bold: this.primaryText(item), suffix: ' удалён навсегда' });
        await this.$refs.base.fetchData();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось удалить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
  },
};
</script>
