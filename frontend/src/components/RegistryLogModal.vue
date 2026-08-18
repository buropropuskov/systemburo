<template>
  <BaseModal
    :show="show"
    :title="title"
    width="760px"
    radius="30px"
    content-testid="registry-log-modal"
    @close="$emit('close')"
  >
    <div class="reglog">
      <p class="reglog__hint">
        Журнал показывает, кто и когда заводил, правил и удалял записи. Удалённые записи
        остаются только здесь: самой строки в реестре больше нет.
      </p>

      <div class="reglog__filters">
        <BaseDropdown
          v-model="actionFilter"
          :options="actionOptions"
          label-key="label"
          value-key="value"
          class="reglog__filter"
          data-testid="registry-log-action"
        />
        <input
          v-model="search"
          class="lk-input reglog__search"
          type="text"
          placeholder="Поиск по записи или работнику"
          data-testid="registry-log-search"
        >
      </div>

      <div
        v-if="loading"
        class="reglog__state"
      >
        Загрузка журнала…
      </div>
      <div
        v-else-if="error"
        class="reglog__state reglog__state--error"
      >
        {{ error }}
      </div>
      <div
        v-else-if="!filteredItems.length"
        class="reglog__state"
        data-testid="registry-log-empty"
      >
        {{ items.length ? 'По условиям отбора событий нет' : 'Журнал пуст' }}
      </div>
      <ul
        v-else
        class="reglog__list"
        data-testid="registry-log-list"
      >
        <li
          v-for="item in filteredItems"
          :key="item.id"
          class="reglog__row"
          :class="{ 'reglog__row--delete': item.action_type === 'delete' }"
        >
          <span class="reglog__when">{{ formatMoment(item.created_at) }}</span>
          <span class="reglog__who">{{ actorName(item) }}</span>
          <span class="reglog__what">{{ describe(item) }}</span>
        </li>
      </ul>
    </div>
  </BaseModal>
</template>

<script>
/**
 * Журнал реестра сотрудников или машин: создания, правки полей и удаления с автором и
 * временем. Заведён ради удалённых записей - у исчезнувшей строки ни карточки, ни
 * истории по её номеру нет, и вопрос «кем и когда удалена» иначе остаётся без ответа.
 *
 * Endpoint отдаёт журнал только администратору, поэтому кнопку показываем по тому же
 * признаку (can_manage_all из ownership-info), а не по отдельной проверке роли.
 */
import { apiRequest } from '@/api/client';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

const ACTION_LABELS = {
  create: 'Создание',
  data_changed: 'Правка',
  delete: 'Удаление',
};

export default {
  name: 'RegistryLogModal',
  components: { BaseModal, BaseDropdown },
  props: {
    show: {
      type: Boolean,
      required: true,
    },
    // employees | cars - от этого зависят endpoint и заголовок.
    entity: {
      type: String,
      required: true,
    },
  },
  emits: ['close'],
  data() {
    return {
      items: [],
      loading: false,
      error: '',
      actionFilter: 'all',
      search: '',
    };
  },
  computed: {
    title() {
      return this.entity === 'cars' ? 'Журнал реестра автомобилей' : 'Журнал реестра сотрудников';
    },
    actionOptions() {
      return [
        { value: 'all', label: 'Все события' },
        { value: 'create', label: 'Создание' },
        { value: 'data_changed', label: 'Правка' },
        { value: 'delete', label: 'Удаление' },
      ];
    },
    filteredItems() {
      const needle = this.search.trim().toLowerCase();
      return this.items.filter((item) => {
        if (this.actionFilter !== 'all' && item.action_type !== this.actionFilter) return false;
        if (!needle) return true;
        return `${this.describe(item)} ${this.actorName(item)}`.toLowerCase().includes(needle);
      });
    },
  },
  watch: {
    // immediate: окно может быть смонтировано уже открытым (и так его монтируют тесты);
    // без этого журнал остался бы пустым до повторного открытия.
    show: {
      immediate: true,
      handler(opened) {
        if (opened) this.load();
      },
    },
  },
  methods: {
    async load() {
      this.loading = true;
      this.error = '';
      try {
        const path = this.entity === 'cars' ? '/unique-cars/history' : '/unique-employees/history';
        const response = await apiRequest(`${path}?limit=500`);
        if (!response.ok) throw new Error(`Журнал не загрузился (${response.status})`);
        const data = await response.json();
        this.items = Array.isArray(data) ? data : [];
      } catch (e) {
        this.error = e.message || 'Журнал не загрузился';
        this.items = [];
      } finally {
        this.loading = false;
      }
    },
    actorName(item) {
      const parts = [item.user_last_name, item.user_first_name].filter(Boolean);
      if (parts.length) return parts.join(' ');
      return item.username || 'Система';
    },
    describe(item) {
      if (item.comment) return item.comment;
      if (item.action_type === 'data_changed' && item.field_name) {
        const from = item.old_value || 'пусто';
        const to = item.new_value || 'пусто';
        return `${ACTION_LABELS.data_changed}: ${item.field_name}, было «${from}», стало «${to}»`;
      }
      return ACTION_LABELS[item.action_type] || item.action_type;
    },
    formatMoment(value) {
      if (!value) return '';
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) return '';
      return date.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
      });
    },
  },
};
</script>

<style scoped>
.reglog {
    display: flex;
    flex-direction: column;
    gap: 12px;
    max-height: 60vh;
}

.reglog__hint {
    margin: 0;
    font-size: 12px;
    color: var(--text-muted);
}

.reglog__filters {
    display: flex;
    gap: 10px;
    align-items: center;
}

.reglog__filter {
    width: 180px;
    flex-shrink: 0;
}

.reglog__search {
    flex: 1;
}

.reglog__state {
    padding: 24px 0;
    text-align: center;
    font-size: 13px;
    color: var(--text-muted);
}

.reglog__state--error {
    color: var(--danger-text);
}

.reglog__list {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.reglog__row {
    display: grid;
    grid-template-columns: 130px 160px 1fr;
    gap: 10px;
    align-items: baseline;
    padding: 8px 10px;
    border-radius: var(--radius-md);
    background: var(--surface-2);
    font-size: 13px;
}

.reglog__row--delete {
    border-left: 3px solid var(--danger);
}

.reglog__when {
    color: var(--text-muted);
    font-size: 12px;
}

.reglog__who {
    font-weight: 500;
}

@media (max-width: 768px) {
    .reglog__filters {
        flex-direction: column;
        align-items: stretch;
    }

    .reglog__filter {
        width: 100%;
    }

    .reglog__row {
        grid-template-columns: 1fr;
        gap: 2px;
    }
}
</style>
