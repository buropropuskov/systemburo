<template>
  <div>
    <BlacklistTabBase
      ref="base"
      empty-noun="людей"
      :entity-icon="userIcon"
      :api-list="listPersonBlacklist"
      :get-primary-text="primaryText"
      :get-detail-rows="detailRows"
      :lookup-card="lookupPerson"
      @count="$emit('count', $event)"
      @create="showCreate = true"
      @archive="askArchive"
      @restore="doRestore"
      @history="historyItem = $event"
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
      v-if="historyItem"
      :item="historyItem"
      :current-user-name="currentUserName"
      @close="historyItem = null"
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
    <EmployeeDetailsModal
      :show="showDetails"
      :employee="detailsEmployee"
      source="blacklist"
      :current-user-name="currentUserName"
      @close="showDetails = false"
    />
  </div>
</template>

<script>
import BlacklistTabBase from './BlacklistTabBase.vue';
import BlacklistCreateModal from './BlacklistCreateModal.vue';
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
} from '@/api/blacklist';
import { lookupUniqueEmployee } from '@/api/employees';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';
import userIcon from '@/assets/icons/user.png';

/**
 * Вкладка "Люди" чёрного списка (#443). Конфигурирует BlacklistTabBase под людей
 * и владеет действиями: создание (модалка), снятие (подтверждение), возврат.
 */
export default {
  name: 'PersonBlacklistTab',
  components: { BlacklistTabBase, BlacklistCreateModal, PersonBlacklistHistoryModal, AddToBlacklistModal, ConfirmationModal, EmployeeDetailsModal },
  props: {
    currentUserName: { type: String, default: '' },
  },
  emits: ['count'],
  data() {
    return {
      showCreate: false, archiveItem: null, historyItem: null, userIcon, detailsEmployee: null, showDetails: false,
      editItem: null, savingEdit: false, editError: '',
    };
  },
  computed: {
    archiveMessage() {
      return this.archiveItem
        ? `Убрать «${this.primaryText(this.archiveItem)}» из чёрного списка? Совпадающие сотрудники с активной заявкой снова станут активными.`
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
    async saveEdit(reason) {
      const item = this.editItem;
      if (!item || this.savingEdit) return;
      this.savingEdit = true;
      this.editError = '';
      try {
        await updatePersonBlacklist(item.id, { reason });
        useDeletionsStore().notify({ prefix: 'Причина обновлена для ', bold: this.primaryText(item) });
        this.editItem = null;
        await this.$refs.base.fetchData();
      } catch (e) {
        this.editError = e?.message || 'Не удалось сохранить причину';
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
