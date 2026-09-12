<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="cars-history-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>
              История въездов и выездов<template v-if="tableTitle"> - {{ tableTitle }}</template>
            </h3>
            <div class="header-actions">
              <button
                class="export-btn"
                :disabled="historyRows.length === 0 || isExporting"
                @click="exportToExcel"
              >
                <AppIcon
                  v-if="!isExporting"
                  name="export"
                  class="export-icon"
                />
                <span v-if="!isExporting">Экспорт</span>
                <div
                  v-else
                  class="export-loader"
                />
              </button>
              <button
                class="close-btn"
                @click="requestClose"
              >
                ×
              </button>
            </div>
          </div>

          <div class="history-filters">
            <div class="filter-row">
              <div class="search-filter">
                <span class="filter-label">Поиск:</span>
                <input 
                  v-model="searchQuery" 
                  type="text" 
                  class="search-input" 
                  placeholder="Поиск по автомобилю, номеру, организации..."
                  @input="applyFilters"
                >
              </div>
              <div class="user-filter">
                <span class="filter-label">Пользователь:</span>
                <div
                  ref="userSelect"
                  class="custom-select"
                  @click="toggleUserDropdown"
                >
                  <div class="select-trigger">
                    <span class="selected-value">{{ selectedUserName }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ 'arrow-open': userDropdownOpen }"
                    />
                  </div>
                  <transition name="fade">
                    <div
                      v-if="userDropdownOpen"
                      class="select-dropdown"
                    >
                      <div 
                        class="select-option"
                        :class="{ 'selected': selectedUserId === null }"
                        @click="selectUser(null)"
                      >
                        Все пользователи
                      </div>
                      <div 
                        v-for="user in uniqueUsers" 
                        :key="user.id"
                        class="select-option"
                        :class="{ 'selected': selectedUserId === user.id }"
                        @click="selectUser(user.id)"
                      >
                        {{ user.name }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>
          
              <div class="car-filter">
                <span class="filter-label">Автомобиль:</span>
                <div
                  ref="carSelect"
                  class="custom-select"
                  @click="toggleCarDropdown"
                >
                  <div class="select-trigger">
                    <span class="selected-value">{{ selectedCarName }}</span>
                    <AppIcon
                      name="arrow"
                      class="select-arrow"
                      :class="{ 'arrow-open': carDropdownOpen }"
                    />
                  </div>
                  <transition name="fade">
                    <div
                      v-if="carDropdownOpen"
                      class="select-dropdown"
                    >
                      <div 
                        class="select-option"
                        :class="{ 'selected': selectedCarId === null }"
                        @click="selectCar(null)"
                      >
                        Все автомобили
                      </div>
                      <div 
                        v-for="car in cars" 
                        :key="car.id"
                        class="select-option"
                        :class="{ 'selected': selectedCarId === car.id }"
                        @click="selectCar(car.id)"
                      >
                        {{ formatCarName(car) }}
                      </div>
                    </div>
                  </transition>
                </div>
              </div>
          
              <div class="date-filter">
                <span class="filter-label">Период:</span>
                <input 
                  v-model="dateFrom" 
                  type="date" 
                  class="date-input"
                  @change="applyFilters({ immediate: true })"
                >
                <span class="date-separator">—</span>
                <input 
                  v-model="dateTo" 
                  type="date" 
                  class="date-input"
                  @change="applyFilters({ immediate: true })"
                >
              </div>
          
              <div class="sort-filter">
                <span class="filter-label">Сортировка:</span>
                <button
                  class="sort-btn"
                  @click="toggleSortOrder"
                >
                  <AppIcon
                    name="sort"
                    class="sort-icon"
                    :class="{ 'sort-asc': sortOrder === 'asc' }"
                  />
                  <span>{{ sortOrder === 'desc' ? 'Сначала новые' : 'Сначала старые' }}</span>
                </button>
              </div>
            </div>
          </div>

          <div
            ref="scrollContainer"
            class="modal-content"
            @scroll.passive="onScroll"
          >
            <div
              v-if="loading"
              class="history-loading"
            >
              <LoaderSpinner label="Загрузка истории…" />
            </div>
        
            <div
              v-else-if="historyRows.length === 0"
              class="history-empty"
            >
              История пуста
            </div>
        
            <div
              v-else
              class="history-timeline"
            >
              <template
                v-for="group in historyGroupedByDate"
                :key="group.date"
              >
                <div class="history-date-separator">
                  {{ group.date }}
                </div>
                <div
                  v-for="(item, i) in group.items"
                  :key="item.id"
                  class="history-item"
                >
                  <div
                    class="timeline-dot"
                    :class="getActionClass(item.action_type)"
                  />
                  <div
                    v-if="i < group.items.length - 1"
                    class="timeline-line"
                  />

                  <div class="history-content">
                    <div class="history-header">
                      <span class="car-info">{{ getCarInfo(item) }}</span>
                      <span class="user-name">{{ item.user_name || 'Система' }}</span>
                      <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                    </div>

                    <div class="action-text">
                      {{ getActionText(item) }}
                      <span
                        v-if="item.reverted"
                        class="reverted-badge"
                      >отменена</span>
                    </div>

                    <div
                      v-if="item.action_type === 'entry' || item.action_type === 'exit'"
                      class="action-comment"
                    >
                      {{ getActionComment(item) }}
                    </div>

                    <div
                      v-if="item.comment && item.action_type !== 'entry' && item.action_type !== 'exit'"
                      class="action-comment"
                    >
                      {{ item.comment }}
                    </div>

                    <div
                      v-if="item.table_name"
                      class="place-name"
                    >
                      {{ item.table_name }}
                    </div>
                  </div>
                </div>
              </template>

              <div class="history-footer">
                <span>Показано {{ historyRows.length }} из {{ total }}</span>
                <LoaderSpinner
                  v-if="loadingMore"
                  label="Загрузка ещё"
                />
                <button
                  v-else-if="hasMore"
                  class="load-more-btn"
                  @click="loadMore"
                >
                  Показать ещё
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { ref } from 'vue';
import {
  PASSAGE_EXPORT_LIMIT,
  PASSAGE_PAGE_SIZE,
  collectPassageRows,
  fetchPassageFilterOptions,
  fetchPassagePage,
} from '@/utils/passageJournal';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { buildPassageJournalBlob, saveJournalBlob } from '@/utils/passageJournalExport';
import { formatMoscow, formatMoscowDateTime } from '@/utils/serverTime';

export default {
  name: 'CarsTableHistoryModal',
  components: { LoaderSpinner, AppIcon },
  props: {
    cars: {
      type: Array,
      required: true
    },
    // Таблица проходной, из которой открыли историю. Без неё модалка показывает
    // историю всех таблиц - так она используется вне конкретной таблицы.
    tableId: {
      type: Number,
      default: null
    },
    // Отображаемое имя таблицы для заголовка.
    tableTitle: {
      type: String,
      default: ''
    },
    currentUserId: {
      type: Number,
      default: null
    },
    currentUserName: {
      type: String,
      default: ''
    }
  },
  emits: ['close'],
  setup(_, { emit }) {
    // Анимация закрытия: внутренний visible (enter по mounted, leave по requestClose),
    // emit('close') только ПОСЛЕ leave-перехода (@after-leave) - иначе родитель
    // размонтирует мгновенно и анимация не проиграется.
    const visible = ref(false);
    const requestClose = () => { visible.value = false; };
    const onAfterLeave = () => emit('close');
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);
    return { visible, requestClose, onAfterLeave, onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      loading: false,
      history: [],
      sortOrder: 'desc',
      searchQuery: '',
      selectedCarId: null,
      selectedUserId: null,
      dateFrom: '',
      dateTo: '',
      userDropdownOpen: false,
      carDropdownOpen: false,
      isExporting: false,
      carsMap: {},
      // Страница журнала и общее число строк по фильтру: журнал читается порциями,
      // а не целиком (#2469).
      page: 1,
      total: 0,
      loadingMore: false,
      filterUsers: [],
      searchTimer: null,
      // Порядковый номер запроса: ответ отставшей загрузки не должен затирать свежую.
      loadSeq: 0
    };
  },
  computed: {
    // Кто отмечал проходы - отдельный запрос к журналу, а не разбор загруженной
    // страницы: собранный из 50 строк список предлагал бы не тех, кто отмечал.
    uniqueUsers() {
      return this.filterUsers;
    },

    selectedUserName() {
      if (this.selectedUserId === null) return 'Все пользователи';
      const user = this.uniqueUsers.find(u => u.id === this.selectedUserId);
      return user ? user.name : 'Все пользователи';
    },

    selectedCarName() {
      if (this.selectedCarId === null) return 'Все автомобили';
      return this.getCarNameById(this.selectedCarId);
    },

    // Фильтры, поиск и порядок применяет сервер, поэтому показываем то, что пришло.
    historyRows() {
      return this.history;
    },

    // Загружено меньше, чем нашлось по фильтру - значит есть что подгружать.
    hasMore() {
      return this.history.length < this.total;
    },

    historyGroupedByDate() {
      const groups = [];
      const dateMap = new Map();
      for (const item of this.historyRows) {
        const dateKey = formatMoscow(new Date(item.created_at), { day: 'numeric', month: 'long', year: 'numeric' });
        if (!dateMap.has(dateKey)) {
          dateMap.set(dateKey, []);
          groups.push({ date: dateKey, items: dateMap.get(dateKey) });
        }
        dateMap.get(dateKey).push(item);
      }
      return groups;
    },



    formattedCurrentDateTime() {
      return formatMoscowDateTime();
    },

    currentUserDisplayName() {
      if (!this.currentUserName) return 'Пользователь';
      const parts = this.currentUserName.split(' ').filter(part => part && part !== 'null' && part !== 'undefined');
      return parts.length > 0 ? parts.join(' ') : 'Пользователь';
    }
  },
  mounted() {
    this.visible = true;
    this.loadHistory();
    this.loadFilterUsers();
    document.addEventListener('click', this.handleClickOutside);
    document.addEventListener('keydown', this.onKeydown);

    this.cars.forEach(car => {
      this.carsMap[car.id] = this.formatCarName(car);
    });
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    formatCarName(car) {
      if (car.car_number && car.car_number.toLowerCase().includes('по факту')) {
        return `По факту (${car.organization_name || car.organization || 'Не указана'})`;
      }
      return `${car.car_number || 'Без номера'} ${car.car_brand || ''} (${car.organization_name || car.organization || 'Не указана'})`.trim();
    },

    getCarNameById(carId) {
      if (!this.carsMap[carId]) {
        const car = this.cars.find(c => c.id == carId);
        if (car) {
          this.carsMap[carId] = this.formatCarName(car);
        } else {
          this.carsMap[carId] = `Автомобиль ID: ${carId}`;
        }
      }
      return this.carsMap[carId];
    },

    getCarInfo(item) {
      if (item.car_number || item.organization) {
        if (item.car_number && item.car_number.toLowerCase().includes('по факту')) {
          return `По факту (${item.organization || 'Не указана'})`;
        }
        return `${item.car_number || 'Без номера'} ${item.car_brand || ''} (${item.organization || 'Не указана'})`.trim();
      }
      return this.getCarNameById(item.car_id);
    },

    getActionText(item) {
      // GetCarsHistoryByTable (/cars/history/table/:id) не фильтрует action_type (урок
      // #1085), поэтому словарь обязан покрывать все действия машины, иначе deactivate/
      // create/прочие текут в журнал сырым английским кодом.
      if (item.action_type === 'entry') {
        return 'Отметил о прибытии';
      } else if (item.action_type === 'exit') {
        return 'Машина уехала';
      } else if (item.action_type === 'entry_revert') {
        return 'Отметка о прибытии отменена';
      } else if (item.action_type === 'exit_revert') {
        return 'Отметка об убытии отменена';
      } else if (item.action_type === 'delete') {
        return 'Удаление из таблицы';
      } else if (item.action_type === 'restore') {
        return 'Восстановление в таблице';
      } else if (item.action_type === 'purge') {
        return 'Безвозвратное удаление';
      } else if (item.action_type === 'added_to_table') {
        return 'Добавлен в таблицу проходной';
      } else if (item.action_type === 'moved_between_tables') {
        return 'Перенесён между таблицами';
      } else if (item.action_type === 'unbound_from_table') {
        return 'Снят с таблицы';
      } else if (item.action_type === 'create') {
        return 'Подана заявка на автомобиль';
      } else if (item.action_type === 'activate') {
        return 'Автомобиль введён в работу';
      } else if (item.action_type === 'deactivate') {
        return 'Автомобиль выведен из работы';
      } else if (item.action_type === 'blacklisted') {
        return 'Добавлен в чёрный список';
      } else if (item.action_type === 'unblacklisted') {
        return 'Снят с чёрного списка';
      } else if (item.action_type === 'blacklist_override') {
        return 'Пропущен несмотря на подозрение в обходе ЧС';
      } else if (item.action_type === 'blacklist_override_revoke') {
        return 'Отменено подтверждение пропуска (обход ЧС)';
      } else if (item.action_type === 'update') {
        return item.field_name ? `Изменено поле "${item.field_name}"` : 'Данные обновлены';
      }
      return item.action_type;
    },

    getActionComment(item) {
      if (item.action_type === 'entry') {
        return `Пользователь ${item.user_name || 'Система'} отметил о прибытии автомобиля ${item.car_number || ''} ${item.car_brand || ''} на территорию`.trim();
      } else if (item.action_type === 'exit') {
        return `Пользователь ${item.user_name || 'Система'} отметил об убытии автомобиля ${item.car_number || ''} ${item.car_brand || ''} с территории`.trim();
      }
      return item.comment || '';
    },

    journalPath() {
      return this.tableId ? `/cars/history/table/${this.tableId}` : '/cars/history/all';
    },

    journalFilters() {
      return {
        search: this.searchQuery,
        userId: this.selectedUserId,
        entityId: this.selectedCarId,
        dateFrom: this.dateFrom,
        dateTo: this.dateTo,
        order: this.sortOrder,
      };
    },

    /**
     * Читает страницу журнала. append дописывает её к уже показанным строкам,
     * иначе список начинается заново с первой страницы.
     *
     * Ответ отставшего запроса отбрасывается по номеру: пока грузилась страница с
     * прежним фильтром, человек успевает набрать в поиске ещё символ, и без этой
     * проверки список показывал бы результат старого запроса.
     */
    async loadHistory({ append = false } = {}) {
      const seq = ++this.loadSeq;
      if (append) {
        this.loadingMore = true;
      } else {
        this.loading = true;
        this.page = 1;
      }
      try {
        const result = await fetchPassagePage(this.journalPath(), this.journalFilters(), {
          page: this.page,
          perPage: PASSAGE_PAGE_SIZE,
        });
        if (seq !== this.loadSeq) return;
        this.history = append ? [...this.history, ...result.items] : result.items;
        this.total = result.total;
      } catch {
        if (seq !== this.loadSeq) return;
        // Ошибка обработана здесь целиком: человек видит тост, список остаётся прежним.
        // Наружу её бросать некому - загрузку начинают mounted и обработчики фильтров,
        // и необработанное отклонение только шумело бы в консоли.
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'историю проходов', type: 'error' });
        // Неудавшаяся подгрузка возвращает счётчик назад, иначе следующая попытка
        // перескочит страницу и в списке появится дыра.
        if (append) this.page = Math.max(1, this.page - 1);
      } finally {
        if (seq === this.loadSeq) {
          this.loading = false;
          this.loadingMore = false;
        }
      }
    },

    async loadMore() {
      if (this.loadingMore || this.loading || !this.hasMore) return;
      this.page += 1;
      await this.loadHistory({ append: true });
    },

    // Подгрузка на подходе к концу списка: 200 пикселей запаса, чтобы порция успела
    // прийти до того, как человек доскроллит до пустоты.
    onScroll(event) {
      const el = event.target;
      if (el.scrollHeight - el.scrollTop - el.clientHeight < 200) this.loadMore();
    },

    async loadFilterUsers() {
      try {
        const options = await fetchPassageFilterOptions('/cars/history/filter-options', this.tableId);
        this.filterUsers = options.users.map(user => ({ id: user.id, name: user.name || 'Система' }));
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'список пользователей фильтра', type: 'error' });
      }
    },

    getActionClass(actionType) {
      // Зелёная точка - машина появляется/остаётся в таблице (прибытие, восстановление,
      // добавление/перенос, ввод в работу, снятие с ЧС). Остальное (уехала, удаление,
      // вывод из работы по истечении срока, добавление в ЧС) - красная по умолчанию.
      if (['entry', 'restore', 'added_to_table', 'moved_between_tables', 'activate', 'create', 'unblacklisted', 'blacklist_override'].includes(actionType)) return 'dot-entry';
      return 'dot-exit';
    },

    formatDateTime(dateTimeString) {
      if (!dateTimeString) return '';
      return formatMoscowDateTime(new Date(dateTimeString));
    },

    toggleSortOrder() {
      this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
      this.applyFilters({ immediate: true });
    },

    toggleUserDropdown() {
      this.userDropdownOpen = !this.userDropdownOpen;
      this.carDropdownOpen = false;
    },

    toggleCarDropdown() {
      this.carDropdownOpen = !this.carDropdownOpen;
      this.userDropdownOpen = false;
    },

    selectUser(userId) {
      this.selectedUserId = userId;
      this.userDropdownOpen = false;
      this.applyFilters({ immediate: true });
    },

    selectCar(carId) {
      this.selectedCarId = carId;
      this.carDropdownOpen = false;
      this.applyFilters({ immediate: true });
    },

    // Поиск перезагружает журнал с задержкой: без неё каждый символ уходил бы
    // отдельным запросом. Остальные фильтры применяются сразу.
    applyFilters({ immediate = false } = {}) {
      clearTimeout(this.searchTimer);
      if (immediate) {
        this.loadHistory();
        return;
      }
      this.searchTimer = setTimeout(() => this.loadHistory(), 300);
    },

    handleClickOutside(event) {
      // refs, не this.$el.querySelectorAll: при <Teleport> $el - это якорный комментарий
      // без querySelector, и дропдауны фильтров не закрывались бы по клику снаружи.
      const selects = [this.$refs.userSelect, this.$refs.carSelect];
      const clickedInside = selects.some(
        (select) => select && select.contains(event.target),
      );
      if (!clickedInside) {
        this.userDropdownOpen = false;
        this.carDropdownOpen = false;
      }
    },

    exportRow(item) {
      return [
        this.formatDateTime(item.created_at),
        this.getCarInfo(item),
        item.user_name || 'Система',
        this.getActionText(item),
        this.getActionComment(item),
        item.table_name || '',
      ];
    },

    /**
     * Выгружает журнал по текущему фильтру, а не то, что успело загрузиться в список:
     * с подгрузкой страницами «экспорт видимого» отдавал бы первые 50 строк.
     */
    async exportToExcel() {
      if (this.historyRows.length === 0) return;

      this.isExporting = true;
      try {
        const { rows, total, truncated } = await collectPassageRows(
          this.journalPath(),
          this.journalFilters(),
          { entityKey: 'car_id' },
        );
        const blob = await buildPassageJournalBlob({
          sheetName: 'Istoriya_viezdov',
          headers: ['Дата и время', 'Автомобиль', 'Пользователь', 'Действие', 'Комментарий', 'Место'],
          rows: rows.map(item => this.exportRow(item)),
          columnWidths: [25, 50, 40, 30, 60, 30],
          author: this.currentUserDisplayName,
          formedAt: this.formattedCurrentDateTime,
          // Обрезанная выгрузка обязана сказать об этом в самом файле: по нему считают
          // итоги за период, и молчание превратило бы часть журнала в «всё, что было».
          note: truncated ? `${rows.length} из ${total} (предел выгрузки ${PASSAGE_EXPORT_LIMIT})` : '',
        });
        saveJournalBlob(blob, `Istoriya_viezdov_${this.formattedCurrentDateTime.replace(/[.:,]/g, '-')}.xlsx`);

        if (truncated) {
          useDeletionsStore().notify({
            prefix: `В файл попали первые ${rows.length} строк из ${total}. `,
            bold: 'Сузьте период или фильтр',
            type: 'warning',
          });
        }
      } catch {
        useDeletionsStore().notify({ bold: 'Ошибка при экспорте в Excel', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },

    onKeydown(e) {
      if (e.key === 'Escape') this.requestClose();
    },
  }
};
</script>

<style scoped>
.history-date-separator {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  padding: 8px 0 4px;
  margin-bottom: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  letter-spacing: 0.02em;
}

.place-name {
  font-size: 11px;
  color: var(--accent-text);
  margin-top: 2px;
  font-weight: 500;
  font-style: italic;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 13000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

/* Анимация открытия/закрытия (паттерн BaseModal): overlay fade + контент scale */
.modal-fade-enter-active {
  transition: opacity 0.25s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .cars-history-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .cars-history-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes modal-scale-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

.cars-history-modal {
  background: var(--surface);
  border-radius: 30px;
  width: 900px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px var(--shadow-drop);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.export-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 6px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
}

.export-btn:hover:not(:disabled) {
  background: var(--surface-2);
  border-color: var(--accent);
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.export-icon {
  width: 14px;
  height: 14px;
}

.export-loader {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border);
  border-top: 2px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--text-muted);
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: var(--surface-2);
  color: var(--text);
}

.history-filters {
  padding: 15px 20px;
  border-bottom: 1px solid var(--border);
  background-color: var(--surface-2);
  flex-shrink: 0;
}

.filter-row {
  display: flex;
  gap: 15px;
  align-items: center;
  flex-wrap: wrap;
}

.search-filter,
.user-filter,
.car-filter,
.date-filter,
.sort-filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
}

.search-input {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  width: 200px;
  height: 32px;
  transition: all 0.2s ease;
}

.search-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.custom-select {
  position: relative;
  width: 200px;
  cursor: pointer;
}

.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  transition: all 0.2s ease;
  height: 32px;
}

.select-trigger:hover {
  border-color: var(--accent);
  background: var(--surface-2);
}

.selected-value {
  font-size: 12px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.select-arrow {
  width: 8px;
  height: 8px;
  transition: transform 0.2s ease;
}

.select-arrow.arrow-open {
  transform: rotate(90deg);
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.select-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 300px;
  overflow-y: auto;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  box-shadow: 0 4px 12px var(--shadow-drop);
  z-index: 1000;
}

.select-dropdown::-webkit-scrollbar {
  width: 0px;
}

.select-option {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  border-bottom: 1px solid var(--border);
}

.select-option:last-child {
  border-bottom: none;
}

.select-option:hover {
  background-color: var(--surface-2);
}

.select-option.selected {
  background-color: var(--accent-tint);
  font-weight: 500;
}

.date-input {
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 12px;
  width: 120px;
  height: 32px;
}

.date-separator {
  color: var(--text-muted);
  font-size: 12px;
}

.sort-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 32px;
  width: 150px;
}

.sort-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.sort-icon {
  color: var(--text-muted);
  width: 14px;
  height: 14px;
  transition: transform 0.2s ease;
}

.sort-icon.sort-asc {
  transform: rotate(180deg);
}

.modal-content {
  padding: 20px;
  overflow-y: auto;
  flex: 1;
}

.history-loading,
.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--text-muted);
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid var(--surface-2);
  border-top: 3px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.history-timeline {
  position: relative;
  padding-left: 20px;
}

.history-item {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  position: relative;
}

.history-item:last-child {
  margin-bottom: 0;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
}

.timeline-line {
  position: absolute;
  left: 4px;
  top: 18px;
  width: 2px;
  height: calc(100% + 2px);
  background: var(--border);
}

.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }

.history-content {
  flex: 1;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
  flex-wrap: wrap;
  gap: 5px;
}

.car-info {
  font-weight: 600;
  color: var(--accent-text);
  font-size: 13px;
}

.user-name {
  font-weight: 500;
  color: var(--text);
  font-size: 13px;
}

.action-time {
  color: var(--text-muted);
  font-size: 11px;
}

.action-text {
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 4px;
  padding-left: 6px;
  border-left: 2px solid var(--border);
}

@media (max-width: 768px) {
  .filter-row {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .search-filter,
  .user-filter,
  .car-filter,
  .date-filter,
  .sort-filter {
    width: 100%;
  }
  
  .custom-select,
  .search-input,
  .date-input,
  .sort-btn {
    width: 100%;
  }
  
  .date-input {
    width: calc(50% - 20px);
  }
}

</style>