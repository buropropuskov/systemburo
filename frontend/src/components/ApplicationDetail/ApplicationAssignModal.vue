<template>
  <BaseModal
    :show="show"
    :title="title"
    width="560px"
    content-class="application-assign-modal"
    @close="$emit('close')"
  >
    <div
      class="assign"
      data-testid="application-assign-modal"
    >
      <p class="assign__summary">
        {{ summary }}
      </p>

      <div
        v-if="loading"
        class="assign__state"
      >
        Загрузка...
      </div>

      <div
        v-else-if="!options.length"
        class="assign__state"
        data-testid="application-assign-empty"
      >
        {{ kind === 'tables' ? 'Нет доступных постов' : 'Нет доступных мест разгрузки' }}
      </div>

      <TargetTablesGrid
        v-else-if="kind === 'tables'"
        v-model="selected"
        :tables="options"
        multiple
      />

      <div
        v-else
        class="assign__grid"
      >
        <div
          v-for="place in options"
          :key="place.id"
          class="assign__item"
          :class="{
            'assign__item--active': selected.includes(place.id) && place.status === 'active',
            'assign__item--inactive': place.status !== 'active'
          }"
          data-testid="application-assign-place"
          @click="togglePlace(place)"
        >
          {{ place.name }}
        </div>
      </div>

      <p
        v-if="willClearAll"
        class="assign__warning"
        data-testid="application-assign-warning"
      >
        Ничего не выбрано: {{ kind === 'tables' ? 'элемент не попадёт ни в одну таблицу проходной' : 'у машины не останется мест разгрузки' }}.
      </p>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="application-assign-cancel"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="application-assign-apply"
        :disabled="submitting"
        @click="$emit('apply', [...selected])"
      >
        {{ submitting ? 'Сохранение...' : 'Сохранить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue';
import { apiRequest } from '@/api/client';

/**
 * Выбор постов или мест разгрузки при доназначении элементам заявки (#1393).
 *
 * Показывает текущий набор выбранным и отдаёт родителю итоговый список - тот шлёт
 * его режимом replace, поэтому одно окно закрывает и добавление, и снятие.
 * Для постов переиспользует TargetTablesGrid (тот же грид, что в форме заявки и в
 * групповых операциях таблиц проходной), для мест разгрузки - плитки по образцу
 * блока «Места разгрузки» VehicleForm.
 */
export default {
  name: 'ApplicationAssignModal',
  components: { BaseModal, TargetTablesGrid },
  props: {
    show: { type: Boolean, default: false },
    // 'tables' - посты проезда/прохода, 'places' - места разгрузки
    kind: {
      type: String,
      required: true,
      validator: (v) => ['tables', 'places'].includes(v),
    },
    // 'cars' | 'people' - от типа зависят и посты, и формулировки
    elementType: {
      type: String,
      required: true,
      validator: (v) => ['cars', 'people'].includes(v),
    },
    // Что уже назначено: набор показывается выбранным
    currentIds: { type: Array, default: () => [] },
    // Сколько элементов затронет операция (1 - одна строка, N - «назначить всем»)
    targetCount: { type: Number, default: 1 },
    submitting: { type: Boolean, default: false },
  },
  emits: ['close', 'apply'],
  data() {
    return {
      selected: [],
      allTables: [],
      allPlaces: [],
      loading: false,
    };
  },
  computed: {
    title() {
      if (this.kind === 'places') return 'Места разгрузки';
      return this.elementType === 'cars' ? 'Посты проезда' : 'Места прохода';
    },

    summary() {
      if (this.targetCount > 1) {
        const word = this.elementType === 'cars' ? 'машинам' : 'сотрудникам';
        return `Выбор применится к ${this.targetCount} ${word}.`;
      }
      return 'Отметьте нужное: снятые отметки уберут привязку.';
    },

    /**
     * /system-tables отдаёт { table: {...}, fields, ... } - разворачиваем t.table,
     * иначе table_type undefined и фильтр всегда пуст.
     */
    options() {
      if (this.kind === 'places') {
        // отключённое место, уже назначенное машине, показываем - иначе оно
        // молча уедет в запрос и там будет непонятно, что происходит
        return this.allPlaces.filter(
          (p) => p.is_active !== false || this.currentIds.includes(p.id),
        );
      }
      return this.allTables
        .map((t) => t.table || t)
        .filter((tbl) => tbl.table_type === this.elementType)
        .filter((tbl) => tbl.is_active !== false || this.currentIds.includes(tbl.id))
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

    willClearAll() {
      return this.selected.length === 0 && this.currentIds.length > 0;
    },
  },
  watch: {
    show: {
      immediate: true,
      handler(val) {
        if (!val) return;
        this.selected = [...this.currentIds];
        this.load();
      },
    },
  },
  methods: {
    async load() {
      this.loading = true;
      try {
        if (this.kind === 'places') {
          const res = await apiRequest('/unload-places');
          if (res.ok) {
            const data = await res.json();
            this.allPlaces = Array.isArray(data) ? data : [];
          }
        } else {
          const res = await apiRequest('/system-tables');
          if (res.ok) {
            const data = await res.json();
            this.allTables = Array.isArray(data) ? data : [];
          }
        }
      } finally {
        this.loading = false;
      }
    },

    togglePlace(place) {
      if (place.status !== 'active') return;
      const index = this.selected.indexOf(place.id);
      if (index === -1) this.selected.push(place.id);
      else this.selected.splice(index, 1);
    },
  },
};
</script>

<style scoped>
.assign {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.assign__summary {
  margin: 0;
  font-size: 13.5px;
  color: var(--color-text-muted);
}

.assign__state {
  padding: 24px 0;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 14px;
}

/* Плитки мест - по образцу блока «Места разгрузки» формы машины */
.assign__grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.assign__item {
  padding: 8px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #fff;
  font-size: 13.5px;
  color: var(--color-text);
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.assign__item:hover {
  border-color: var(--color-primary);
  background: var(--color-bg);
}

.assign__item--active {
  border-color: var(--color-primary);
  background: var(--color-primary-tint);
  color: var(--color-primary);
  font-weight: 600;
}

.assign__item--inactive {
  opacity: 0.5;
  cursor: not-allowed;
}

.assign__warning {
  margin: 0;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  background: rgba(255, 193, 7, 0.14);
  color: #8a6d00;
  font-size: 13px;
}
</style>
