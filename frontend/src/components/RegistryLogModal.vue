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
        Кто и когда заводил, правил и удалял записи. Удалённые видны только здесь.
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
        v-if="!loading && !error && filteredItems.length"
        class="reglog__head"
        aria-hidden="true"
      >
        <span>Когда</span>
        <span>Кто действовал</span>
        <span>{{ entity === 'cars' ? 'Машина' : 'Работник' }}</span>
        <span>Что произошло</span>
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
          <span class="reglog__who">
            {{ actorName(item) }}
            <span class="reglog__who-role">кто</span>
          </span>
          <span class="reglog__subject">
            {{ item.subject || 'запись не определена' }}
            <span class="reglog__subject-role">{{ entity === 'cars' ? 'машина' : 'работник' }}</span>
          </span>
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
import { formatDateTime } from '@/utils/datetime';

const ACTION_LABELS = {
  create: 'Создание',
  data_changed: 'Правка',
  delete: 'Удаление',
};

// Имена полей приходят из базы как есть (position, first_name, organization_id) - в
// журнале для человека они читаются как мусор, поэтому переводим по словарю.
const FIELD_LABELS = {
  last_name: 'фамилия',
  first_name: 'имя',
  middle_name: 'отчество',
  position: 'должность',
  passport_series_number: 'паспортные данные',
  patent_number: 'номер патента',
  other_permission: 'иное разрешение на работы',
  citizenship_id: 'гражданство',
  organization_id: 'организация',
  company_id: 'компания',
  user_id: 'владелец записи',
  pd_consent_at: 'согласие на обработку персональных данных',
  number: 'номер',
  mark: 'марка',
  format_id: 'формат номера',
};

// Поля-привязки хранят идентификаторы: печатать «было «10», стало «пусто»» значит
// показывать человеку номер строки чужой таблицы. Для них пишем сам факт правки.
const REFERENCE_FIELDS = new Set(['citizenship_id', 'organization_id', 'company_id', 'user_id', 'format_id']);

/** Первая буква заглавной: словарь полей хранит их со строчной («должность»). */
function capitalize(text) {
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : text;
}

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
        return `${this.describe(item)} ${this.actorName(item)} ${item.subject || ''}`.toLowerCase().includes(needle);
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
    /**
     * Текст события. Объект (кто именно правился) стоит отдельной колонкой, поэтому
     * здесь только действие - иначе строка читалась бы «Мякотных Сергей | Сотрудник
     * Иванов удалён из реестра», и было бы непонятно, кто из двоих кого удалил.
     * comment используем только как запас для событий, записанных до появления снимка.
     */
    describe(item) {
      if (item.action_type === 'create') return 'Заведена в реестр';
      if (item.action_type === 'delete') return 'Удалена из реестра';
      if (item.action_type === 'data_changed' && item.field_name) {
        const field = FIELD_LABELS[item.field_name] || item.field_name;
        if (item.field_name === 'pd_consent_at') {
          return item.new_value ? 'Подтверждено согласие на обработку персональных данных' : `Изменено: ${field}`;
        }
        if (REFERENCE_FIELDS.has(item.field_name)) {
          return `Изменено: ${field}`;
        }
        const from = item.old_value || 'пусто';
        const to = item.new_value || 'пусто';
        return `${capitalize(field)}: «${from}» → «${to}»`;
      }
      return item.comment || ACTION_LABELS[item.action_type] || item.action_type;
    },
    formatMoment: formatDateTime,
  },
};
</script>

<style scoped>
/* Базовая модалка не даёт внутренних отступов (base-modal__body: padding 0) - их задаёт
   содержимое. Без них текст и строки липли к самым краям окна, налезая на скругления. */
.reglog {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 4px 20px 20px;
    max-height: 60vh;
    min-width: 0;
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
    width: 170px;
    flex-shrink: 0;
}

.reglog__search {
    flex: 1;
    min-width: 0;
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

/* minmax(0, 1fr) в последней колонке: с обычным 1fr длинная строка события раздувает
   сетку, и список выезжает за правый край окна. */
.reglog__head,
.reglog__row {
    display: grid;
    grid-template-columns: 112px 150px 170px minmax(0, 1fr);
    gap: 12px;
    align-items: baseline;
    padding: 8px 12px;
    border-radius: var(--radius-md);
    background: var(--surface-2);
    font-size: 13px;
    min-width: 0;
}

/* Удаление выделяем мягким фоном, а не левым бордюром: на скруглённой строке он торчал
   отдельной красной чёрточкой. */
.reglog__row--delete {
    background: var(--danger-bg);
}

.reglog__row--delete .reglog__what {
    color: var(--danger-text);
}

/* Шапка колонок: без неё непонятно, кто в строке действовал, а кто объект правки. */
.reglog__head {
    padding: 0 12px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--text-muted);
}

.reglog__when {
    color: var(--text-muted);
    font-size: 12px;
    white-space: nowrap;
}

.reglog__subject {
    min-width: 0;
    overflow-wrap: anywhere;
}

/* Подпись роли под значением: страховка на случай, если колонки уедут (узкий экран,
   перенос) - тогда всё равно видно, где действующий, а где объект. */
.reglog__who-role,
.reglog__subject-role {
    display: none;
}

.reglog__who {
    font-weight: 500;
    min-width: 0;
    overflow-wrap: anywhere;
}

.reglog__what {
    min-width: 0;
    overflow-wrap: anywhere;
}

@media (max-width: 768px) {
    .reglog {
        padding: 4px 14px 16px;
    }

    .reglog__filters {
        flex-direction: column;
        align-items: stretch;
    }

    .reglog__filter {
        width: 100%;
    }

    .reglog__head {
        display: none;
    }

    .reglog__row {
        grid-template-columns: 1fr;
        gap: 2px;
    }

    .reglog__who-role,
    .reglog__subject-role {
        display: inline;
        margin-left: 6px;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.03em;
        color: var(--text-muted);
    }

    .reglog__when {
        white-space: normal;
    }
}
</style>
