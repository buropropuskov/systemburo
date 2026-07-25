<template>
  <BaseModal
    :show="show"
    :title="modalTitle"
    width="520px"
    content-class="table-bulk-target-modal"
    @close="$emit('close')"
  >
    <div
      class="tbt"
      data-testid="table-bulk-target-modal"
    >
      <p class="tbt__summary">
        Операция применится к <b>{{ selectedCount }}</b> {{ entityWord }}.
      </p>

      <div
        v-if="loading"
        class="tbt__state"
      >
        Загрузка таблиц...
      </div>
      <div
        v-else-if="!availableTables.length"
        class="tbt__state"
        data-testid="table-bulk-target-empty"
      >
        Нет доступных таблиц
      </div>
      <TargetTablesGrid
        v-else
        v-model="tableIds"
        :tables="availableTables"
        multiple
      />
    </div>

    <template #actions>
      <button
        class="lk-button lk-button--ghost"
        data-testid="table-bulk-target-cancel"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="lk-button lk-button--primary"
        data-testid="table-bulk-target-apply"
        :disabled="!canApply"
        @click="onApply"
      >
        {{ submitting ? 'Применение...' : applyLabel }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue';
import { apiRequest } from '@/api/client';

/**
 * Модалка групповых операций "Перенести"/"Добавить в таблицу" над выделенными
 * строками таблицы проходной (#1194, срез S4). Переиспользует TargetTablesGrid
 * (тот же грид плиток, что в VehicleForm/EmployeeForm) со списком системных таблиц
 * ТОГО ЖЕ table_type, исключая текущую таблицу.
 *
 * Собирает выбор и эмитит `apply` с массивом id целевых таблиц - батч-запрос,
 * разбор BulkOpResult и обновление списка делает родитель (CarsTable/PeopleTable),
 * по образцу BulkOperationsModal (directories/BulkOperationsModal.vue).
 */
export default {
  name: 'TableBulkTargetModal',
  components: { BaseModal, TargetTablesGrid },
  props: {
    show: { type: Boolean, default: false },
    // 'move' - перенести (снять с текущей, привязать к выбранным),
    // 'add' - добавить в выбранные, не снимая с текущей.
    mode: {
      type: String,
      required: true,
      validator: (v) => ['move', 'add'].includes(v),
    },
    // Тип целевых таблиц - совпадает с типом строк, над которыми выполняется операция.
    entityType: {
      type: String,
      required: true,
      validator: (v) => ['cars', 'people'].includes(v),
    },
    // ID текущей таблицы (проп tableId вызывающей CarsTable/PeopleTable) - исключается
    // из списка выбора (перенос/добавление в саму себя бессмысленны).
    excludeTableId: { type: Number, default: null },
    selectedCount: { type: Number, default: 0 },
    submitting: { type: Boolean, default: false },
  },
  emits: ['close', 'apply'],
  data() {
    return {
      tableIds: [],
      allTables: [],
      loading: false,
    };
  },
  computed: {
    modalTitle() {
      return this.mode === 'move' ? 'Перенести в таблицу' : 'Добавить в таблицу';
    },
    entityWord() {
      return this.entityType === 'cars' ? 'машинам' : 'сотрудникам';
    },
    applyLabel() {
      const verb = this.mode === 'move' ? 'Перенести' : 'Добавить';
      return `${verb} (${this.selectedCount})`;
    },
    // /system-tables (GetAll) отдаёт SystemTableWithDetails с double-wrap { table: {...},
    // fields, ... } - разворачиваем t.table (как VehicleForm/SelectTables), иначе t.table_type
    // undefined и фильтр всегда пуст ("Нет доступных таблиц"). TargetTablesGrid ждёт { table: {...} }.
    availableTables() {
      return this.allTables
        .map((t) => t.table || t)
        .filter((tbl) => tbl.table_type === this.entityType && tbl.id !== this.excludeTableId)
        .map((tbl) => ({
          table: {
            id: tbl.id,
            name: tbl.name,
            display_name: tbl.display_name,
            table_type: tbl.table_type,
            status: tbl.status || 'active',
            status_comment: tbl.status_comment,
          },
        }));
    },
    canApply() {
      return !this.submitting && this.tableIds.length > 0;
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.tableIds = [];
        this.fetchTables();
      }
    },
  },
  methods: {
    async fetchTables() {
      this.loading = true;
      try {
        const response = await apiRequest('/system-tables', {});
        if (response.ok) {
          this.allTables = await response.json();
        }
      } catch (error) {
        console.error('Ошибка при загрузке системных таблиц:', error);
      } finally {
        this.loading = false;
      }
    },
    onApply() {
      if (!this.canApply) return;
      this.$emit('apply', [...this.tableIds]);
    },
  },
};
</script>

<style scoped>
.tbt {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.tbt__summary {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}

.tbt__summary b {
  color: var(--accent-text);
}

.tbt__state {
  font-size: 13px;
  color: var(--text-muted);
  text-align: center;
  padding: 20px 0;
}
</style>
