<template>
  <div
    class="tables"
    :class="{ 'tables--fab': showFabBar }"
  >
    <div class="tables__sticky-head">
      <div
        ref="headerRef"
        class="tables__header"
      >
        <h1
          ref="titleRef"
          class="tables__title"
        >
          Таблица <span class="table-name">{{ tableDisplayName }}</span>
        </h1>
        <button
          class="tables__instruction"
          :class="{ 'tables__instruction--compact': instructionCompact }"
          data-testid="ob-table-instruction"
          :title="instructionCompact ? 'Инструкция' : null"
          :aria-label="instructionCompact ? 'Инструкция' : null"
          @click="openInstruction"
        >
          <AppIcon
            name="instruction"
            class="tables__icon tables__icon--accent"
          />
          <p
            v-if="!instructionCompact"
            class="instruction__text"
          >
            Инструкция
          </p>
        </button>
      </div>

      <!-- Модальное окно с инструкцией -->
      <Teleport to="body">
        <transition name="instruction-modal">
          <div
            v-if="showInstruction"
            class="modal-overlay"
            @mousedown="onOverlayMousedown"
            @mouseup="onOverlayMouseup"
          >
            <div
              class="instruction-modal-large"
              @mousedown.stop
            >
              <div class="modal-header">
                <h3>Инструкция по использованию таблицы <span class="blue">{{ tableDisplayName }}</span></h3>
                <button
                  class="modal-close"
                  @click="closeInstruction"
                >
                  ×
                </button>
              </div>
              <div class="instruction-content">
                <div
                  v-if="tableInstruction"
                  class="text-constructor-content"
                  v-html="sanitizedInstruction"
                />
                <div
                  v-else
                  class="no-instruction"
                >
                  <div class="no-instruction-icon">
                    📝
                  </div>
                  <h4>Инструкция не добавлена</h4>
                  <p>Для этой таблицы пока не создана и не написана инструкция.</p>
                  <p>Обратитесь к Бюро пропусков для добавления инструкции.</p>
                </div>
              </div>
            </div>
          </div>
        </transition>
      </Teleport>

      <div class="tables__filters">
        <div class="filters__fields">
          <!-- Десктоп: поле поиска видно всегда. На мобилке (#1097 S7) его заменяет
               иконка-тоггл справа, а поле раскрывается оверлеем поверх ряда - механика
               взята у Центра и кабинета (эталон §9.3), не изобретена заново. -->
          <div
            v-if="!isNarrow"
            class="field search"
          >
            <input
              v-model="searchQuery"
              placeholder="Поиск.."
              type="text"
              class="field__input search"
              @input="applyFilters"
            >
            <AppIcon
              name="search"
              class="tables__icon"
            />
          </div>

          <!-- Десктоп: вторичные фильтры инлайн в строке (как было). Видимость каждого
               фильтра решает directoryFilters, а не v-if в шаблоне - иначе десктопный ряд
               и мобильный лист со временем разъедутся по составу. data-testid у обеих
               ветвей общий (`table-sheet-*`): ветки взаимоисключающие по isNarrow, дублей
               в DOM нет, а переименование стоило бы правки спек без выгоды. -->
          <template v-if="!isNarrow">
            <div
              v-for="filter in directoryFilters"
              :key="filter.field"
              class="filters__control"
            >
              <BaseDropdown
                :model-value="filter.values"
                :options="filter.options"
                :placeholder="filter.allLabel"
                :summary-label="filter.summaryLabel"
                :search-keys="filter.searchKeys"
                :data-testid="filter.testid"
                multiple
                searchable
                teleport
                @update:model-value="ids => setMultiFilter(filter.field, ids)"
              />
            </div>

            <!-- Новый DateFilter -->
            <DateFilter
              ref="dateFilter"
              :mode="'range'"
              :selected-date="selectedDate"
              :date-range-start="dateRangeStart"
              :date-range-end="dateRangeEnd"
              @update:selected-date="updateSelectedDate"
              @update:date-range-start="updateDateRangeStart"
              @update:date-range-end="updateDateRangeEnd"
              @apply="applyDateFilters"
              @clear="clearDate"
            />
          </template>

          <!-- Мобилка: вторичные фильтры свёрнуты в кнопку «Фильтр», поиск - в иконку. -->
          <template v-else>
            <FilterButton
              :active="secondaryFiltersActive"
              data-testid="table-filter-btn"
              @click="showFilterSheet = true"
            />

            <button
              type="button"
              class="search-icon-btn"
              :class="{ 'search-icon-btn--active': showMobileSearch || !!searchQuery.trim() }"
              aria-label="Поиск по таблице"
              data-testid="table-search-icon"
              @click="toggleMobileSearch"
            >
              <AppIcon
                name="search"
                class="search-icon-btn__img"
              />
            </button>

            <!-- Действия таблицы (Версии/Отчёт/Экспорт/Корзина) свёрнуты в лист снизу:
                 четыре иконки без подписей занимали вторую строку шапки и не читались.
                 В строке остаются поиск, «Фильтр» и это «⋯» (мокап mobile-ux.html,
                 экран «Проходная»). -->
            <button
              v-if="hasSheetActions"
              type="button"
              class="more-icon-btn"
              aria-label="Действия с таблицей"
              data-testid="table-more-btn"
              @click="showActionsSheet = true"
            >
              &#8943;
            </button>

            <!-- Поле раскрывается ВЛЕВО оверлеем поверх ряда через clip-path: ряд не
                 переставляется, кнопка «Фильтр» не уезжает, reflow нет. Иконка справа -
                 тоггл, крестик внутри поля - очистить и закрыть (зеркало Центра).
                 Правый вырез считается от числа кнопок справа: без «⋯» это 46px
                 (36 иконка + 10 зазор ряда), с ним - 92px. Значение приходит
                 переменной, чтобы правило осталось в CSS, а не ушло в inline. -->
            <Transition name="table-search">
              <div
                v-if="showMobileSearch"
                class="tables__search-overlay"
                :style="{ '--search-overlay-right': hasSheetActions ? '92px' : '46px' }"
              >
                <div class="field search">
                  <input
                    ref="mobileSearchInput"
                    v-model="searchQuery"
                    placeholder="Поиск.."
                    type="text"
                    class="field__input search"
                    data-testid="table-input-search"
                    @input="applyFilters"
                  >
                  <button
                    v-if="searchQuery.trim()"
                    type="button"
                    class="tables__search-clear"
                    aria-label="Очистить поиск"
                    @click="clearMobileSearch"
                  >
                    &times;
                  </button>
                </div>
              </div>
            </Transition>
          </template>
        </div>
        <!-- Десктоп: действия таблицы инлайн в шапке. На мобилке ряд расходится по
             двум местам - главное действие уходит в нижнюю панель, остальные в лист
             «⋯». v-if, а не скрытая копия: два узла с одним data-testid ломают
             онбординг-тур и E2E (эталон §2.2). -->
        <div
          v-if="!isNarrow"
          class="filters__options"
        >
          <button
            v-if="canManualAdd"
            class="options__manual-add"
            data-testid="manual-add-button"
            @click="showManualAdd = true"
          >
            <span class="options__manual-add-icon">+</span>
            <span class="options__text">Добавить вручную</span>
          </button>
          <RouterLink
            v-if="canVersions"
            :to="`/table/${$route.params.tableName}/versions`"
            class="options__versions-link"
            title="Версии состояния таблицы"
            data-testid="table-versions-link"
            aria-label="Версии состояния таблицы"
          >
            <AppIcon
              name="recent-changes"
              class="options__icon"
            />
            <span class="options__text">Версии</span>
          </RouterLink>
          <RouterLink
            v-if="canTrash"
            :to="`/table/${$route.params.tableName}/trash`"
            class="options__trash-link"
            title="Корзина таблицы"
            data-testid="table-trash-link"
            aria-label="Корзина таблицы"
          >
            <AppIcon
              name="trashcan"
              class="options__icon"
            />
            <span class="options__text">Корзина</span>
          </RouterLink>
          <button
            v-if="canExport"
            class="options__export"
            @click="handleExport"
          >
            <AppIcon
              name="export"
              class="tables__icon"
            />
            <p class="options__text">
              Экспорт
            </p>
          </button>
          <button
            v-if="canReport"
            class="options__export"
            data-testid="pass-report-button"
            @click="showPassReport = true"
          >
            <AppIcon
              name="stats"
              class="tables__icon"
            />
            <p class="options__text">
              Отчёт
            </p>
          </button>
        </div>
      </div>
    </div>

    <!-- Мобилка: те же действия полными подписями в листе снизу. Механика окна
         (крестик, затемнение, Escape, свайп вниз, блокировка фона) - из BaseModal,
         пропы 1:1 с FilterSheet рядом, чтобы два листа одного экрана не разъехались
         по геометрии. Своя обёртка, а не FilterSheet: у того нижняя панель занята
         кнопкой сброса фильтров, здесь она не нужна. -->
    <BaseModal
      v-if="isNarrow && hasSheetActions"
      :show="showActionsSheet"
      title="Действия с таблицей"
      width="600px"
      radius="30px"
      sheet-swipe
      @close="showActionsSheet = false"
    >
      <div class="actions-sheet__list">
        <RouterLink
          v-if="canVersions"
          :to="`/table/${$route.params.tableName}/versions`"
          class="actions-sheet__item"
          data-testid="table-versions-link"
          @click="showActionsSheet = false"
        >
          <AppIcon
            name="recent-changes"
            class="actions-sheet__icon"
          />
          Версии таблицы
        </RouterLink>
        <!-- «История» переехала сюда из шапки таблицы: там на телефоне один ряд в
             48px, и три группы контролов в него не влезали. Модалка живёт в самой
             таблице, поэтому зовём её метод по ref. -->
        <button
          v-if="canTableHistory"
          type="button"
          class="actions-sheet__item"
          data-testid="table-history-action"
          @click="runSheetAction(openTableHistory)"
        >
          <AppIcon
            name="clipboard"
            class="actions-sheet__icon"
          />
          История изменений
        </button>
        <button
          v-if="canReport"
          type="button"
          class="actions-sheet__item"
          data-testid="pass-report-button"
          @click="runSheetAction(() => { showPassReport = true; })"
        >
          <AppIcon
            name="stats"
            class="actions-sheet__icon"
          />
          {{ tableType === 'cars' ? 'Отчёт по проездам' : 'Отчёт по проходам' }}
        </button>
        <button
          v-if="canExport"
          type="button"
          class="actions-sheet__item"
          data-testid="table-export-action"
          @click="runSheetAction(handleExport)"
        >
          <AppIcon
            name="export"
            class="actions-sheet__icon"
          />
          Экспорт в Excel
        </button>
        <RouterLink
          v-if="canTrash"
          :to="`/table/${$route.params.tableName}/trash`"
          class="actions-sheet__item actions-sheet__item--danger"
          data-testid="table-trash-link"
          @click="showActionsSheet = false"
        >
          <AppIcon
            name="trashcan"
            class="actions-sheet__icon"
          />
          Корзина
        </RouterLink>
      </div>
    </BaseModal>

    <!-- Мобилка: вторичные фильтры в bottom-sheet. -->
    <FilterSheet
      v-if="isNarrow"
      :show="showFilterSheet"
      :has-active-filters="hasActiveFilters"
      @close="showFilterSheet = false"
      @reset="clearFilters"
    >
      <div
        v-for="filter in directoryFilters"
        :key="filter.field"
        class="filter-section"
      >
        <span class="filter-label">{{ filter.summaryLabel }}</span>
        <BaseDropdown
          :model-value="filter.values"
          :options="filter.options"
          :placeholder="filter.allLabel"
          :summary-label="filter.summaryLabel"
          :search-keys="filter.searchKeys"
          :data-testid="filter.testid"
          multiple
          searchable
          teleport
          @update:model-value="ids => setMultiFilter(filter.field, ids)"
        />
      </div>
      <div class="filter-section">
        <span class="filter-label">Период</span>
        <DateFilter
          ref="dateFilter"
          :mode="'range'"
          :selected-date="selectedDate"
          :date-range-start="dateRangeStart"
          :date-range-end="dateRangeEnd"
          data-testid="table-sheet-date"
          @update:selected-date="updateSelectedDate"
          @update:date-range-start="updateDateRangeStart"
          @update:date-range-end="updateDateRangeEnd"
          @apply="applyDateFilters"
          @clear="clearDate"
        />
      </div>
    </FilterSheet>

    <!-- Мобилка: содержимое едет вместе с документом (владелец: "скроллится вся
         страница, кроме шапки") - своей прокрутки у блока нет, .tables__sticky-head
         выше липнет к верху. Таблица «по факту» и основная таблица идут одним
         списком - не два раздельных скролла по компоненту. -->
    <div class="tables__content">
      <div
        v-if="showFactTable"
        class="fact-section"
      >
        <FactTable
          v-if="currentUserId"
          ref="factTable"
          :table-type="tableType"
          :table-id="tableData?.table?.id"
          :table-data="tableData"
          :search-query="searchQuery"
          :selected-organization-ids="selectedOrganizationIds"
          :selected-company-ids="selectedCompanyIds"
          :selected-unloading-place-ids="selectedUnloadingPlaceIds"
          :date-range-start="dateRangeStart"
          :date-range-end="dateRangeEnd"
          :selected-date="selectedDate"
          :current-user-id="currentUserId"
          :current-user-name="currentUserName"
          :grid="gridMode"
          @refresh-data="refreshData"
          @open-application="handleOpenApplication"
        />
        <!-- Подсказка на синем фоне. На мобилке (#1097 S6) сворачивается: там она
             разворачивается во всю ширину между таблицей «по факту» и основной,
             отодвигая последнюю вниз на экран и больше.
             Состояние помним per-table в localStorage - как режим «Сетка» рядом.
             Переключатель только на узком экране: на десктопе подсказка стоит
             сбоку от таблицы и место не отнимает (эталон §9.3 - прятать за кнопку
             только там, где мало места). -->
        <div
          v-if="tableFactHint"
          class="fact-hint-card"
          :class="{ 'fact-hint-card--collapsed': isNarrow && !hintExpanded }"
        >
          <button
            v-if="isNarrow"
            class="fact-hint-card__toggle"
            type="button"
            :aria-expanded="hintExpanded ? 'true' : 'false'"
            data-testid="fact-hint-toggle"
            @click="toggleHint"
          >
            <span class="fact-hint-card__toggle-text">Подсказка</span>
            <!-- Вектор вместо arrow.png: растровая стрелка нативно 30px и на
                 2x-экране в 14px мылилась. Рисуем сами - 16px, штрих 2.2,
                 currentColor (следует за цветом подсказки в любой теме). -->
            <svg
              class="fact-hint-card__chevron"
              width="16"
              height="16"
              viewBox="0 0 16 16"
              fill="none"
              aria-hidden="true"
            >
              <path
                d="M6 3l5 5-5 5"
                stroke="currentColor"
                stroke-width="2.2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <div class="fact-hint-card__collapse">
            <div class="fact-hint-card__collapse-inner">
              <div
                class="text-constructor-content hint-content"
                v-html="sanitizedHint"
              />
            </div>
          </div>
        </div>
      </div>
            
      <!-- Основная таблица - разные компоненты для разных типов -->
      <CarsTable
        v-if="tableType === 'cars' && currentUserId"
        ref="carsTable"
        v-model:grid="gridMode"
        :table-name="tableSystemName"
        :table-title="tableDisplayName"
        :table-id="tableData?.table?.id"
        :search-query="searchQuery"
        :selected-organization-ids="selectedOrganizationIds"
        :selected-company-ids="selectedCompanyIds"
        :selected-unloading-place-ids="selectedUnloadingPlaceIds"
        :date-range-start="dateRangeStart"
        :date-range-end="dateRangeEnd"
        :selected-date="selectedDate"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        @refresh-data="refreshData"
        @open-application="handleOpenApplication"
      />

      <PeopleTable
        v-if="tableType === 'people'"
        ref="peopleTable"
        v-model:grid="gridMode"
        :table-name="tableSystemName"
        :search-query="searchQuery"
        :selected-organization-ids="selectedOrganizationIds"
        :selected-company-ids="selectedCompanyIds"
        :date-range-start="dateRangeStart"
        :date-range-end="dateRangeEnd"
        :selected-date="selectedDate"
        :current-user-id="currentUserId"
        :current-user-name="currentUserName"
        @refresh-data="refreshData"
        @open-application="handleOpenApplication"
      />
    </div>

    <!-- Мобилка: главное действие экрана прижато к низу, под большой палец, и
         продублировано «⋯» - как в мокапе. Панель fixed, поэтому контент получает
         нижний отступ (.tables--fab), иначе она накрывает последнюю карточку.

         data-bottom-action-bar - общий признак «снизу закреплена панель»: по нему
         ScrollTopButton уводит свою кнопку выше, иначе она стоит в этом же углу.
         Атрибут, а не переменная от страницы: любой следующий экран с такой панелью
         получает поведение, просто поставив его на свою. -->
    <div
      v-if="showFabBar"
      class="tables__fab-bar"
      data-bottom-action-bar
    >
      <button
        type="button"
        class="tables__fab-main"
        data-testid="manual-add-button"
        @click="showManualAdd = true"
      >
        Добавить вручную
      </button>
      <button
        v-if="hasSheetActions"
        type="button"
        class="tables__fab-more"
        aria-label="Действия с таблицей"
        data-testid="table-more-bar-btn"
        @click="showActionsSheet = true"
      >
        &#8943;
      </button>
    </div>

    <ApplicationDetail
      v-if="showApplicationDetail"
      :application="selectedApplication"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :mode="'center'"
      @close="closeApplicationDetail"
      @application-changed="handleApplicationChanged"
    />

    <TableExportModal
      :show="showExportModal"
      @close="showExportModal = false"
      @export="handleExportChoice"
    />

    <ManualAddModal
      :show="showManualAdd"
      :mode="tableType || 'cars'"
      :table-id="tableData?.table?.id"
      :table-name="tableDisplayName"
      @close="showManualAdd = false"
      @added="onManualAdded"
    />

    <PassReportModal
      :show="showPassReport"
      :table-id="tableData?.table?.id"
      :table-type="tableType"
      :table-display-name="tableDisplayName"
      :current-user-name="currentUserName"
      @close="showPassReport = false"
    />
  </div>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { apiRequest } from '@/api/client'
import { sanitizeHtml } from '@/utils/sanitize';
import { useOverlayClose } from '@/composables/useOverlayClose';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import DateFilter from './DateFilter.vue';
import FactTable from './FactTable.vue';
import CarsTable from './CarsTable.vue';
import PeopleTable from './PeopleTable.vue';
import ApplicationDetail from './ApplicationDetail/ApplicationDetail.vue';
import TableExportModal from './TableExportModal.vue';
import ManualAddModal from './ManualAddModal.vue';
import PassReportModal from './PassReportModal.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import FilterButton from '@/components/ui/FilterButton.vue';
import FilterSheet from '@/components/ui/FilterSheet.vue';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { usePermissionsStore } from '@/stores/permissions';
import { useOnboardingStore } from '@/stores/onboarding';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'TablesComponent',
    components: {
        AppIcon,
        BaseDropdown,
        DateFilter,
        FactTable,
        CarsTable,
        PeopleTable,
        ApplicationDetail,
        TableExportModal,
        ManualAddModal,
        PassReportModal,
        BaseModal,
        FilterButton,
        FilterSheet,
    },
    emits: ['refresh-data'],
    setup() {
        const showInstruction = ref(false);
        const openInstruction = () => { showInstruction.value = true; };
        const closeInstruction = () => { showInstruction.value = false; };
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(closeInstruction);

        const onKeydown = (e) => {
            if (e.key === 'Escape' && showInstruction.value) closeInstruction();
        };

        watch(showInstruction, (open) => {
            setBodyScrollLock(this, open);
        });

        onMounted(() => document.addEventListener('keydown', onKeydown));
        onBeforeUnmount(() => {
            document.removeEventListener('keydown', onKeydown);
            releaseBodyScrollLock(this);
        });

        const permissionsStore = usePermissionsStore();

        // Мобилка: вторичные фильтры сворачиваются в кнопку «Фильтр» + FilterSheet,
        // поиск - в иконку с оверлеем. Десктоп не трогаем - фильтры инлайн
        // (эпик mobile-filter-collapse).
        //
        // Порог 767.98, а не дефолтные 768 (#1097 S7): страница живёт поверх
        // card-правил responsive-tables.css, которые срабатывают на 767.98 (эталон §1.2).
        // На ровно 768 таблицы оставались десктопными, а шапка страницы уже уезжала в
        // мобильный режим - экран собирался гибридом (свёрнутые фильтры над обычной
        // таблицей). CSS-медиа этого компонента переведены на тот же порог.
        const { isNarrow } = useNarrowScreen(767.98);

        const onboardingStore = useOnboardingStore();
        return { showInstruction, openInstruction, closeInstruction, onOverlayMousedown, onOverlayMouseup, permissionsStore, onboardingStore, isNarrow };
    },
    data() {
        return {
            tableData: null,
            searchQuery: '',
            // Мультивыбор справочников (#1398): пустой массив - фильтр выключен.
            selectedOrganizationIds: [],
            selectedCompanyIds: [],
            selectedUnloadingPlaceIds: [],

            organizations: [],
            companies: [],
            // Справочник мест разгрузки грузит страница: дропдаун фильтра
            // получает готовые опции пропом и сам ничего не запрашивает.
            unloadPlaces: [],

            selectedDate: null,
            dateRangeStart: null,
            dateRangeEnd: null,
            
            currentUserId: null,
            currentUserName: '',

            showApplicationDetail: false,
            selectedApplication: null,
            showExportModal: false,
            showManualAdd: false,
            showPassReport: false,
            // Окно отчёта открыл тур - только такое он и закрывает за собой.
            passReportOpenedByTour: false,

            // Мобилка: открыт ли bottom-sheet со вторичными фильтрами.
            showFilterSheet: false,

            // Мобилка: открыт ли лист действий таблицы («⋯»).
            showActionsSheet: false,

            // Мобилка (#1097 S7): раскрыт ли оверлей поиска над рядом фильтров.
            showMobileSearch: false,

            // Режим "Сетка" (#1289): один тумблер страницы на обе таблицы
            // (по факту + основная). Состояние своё у каждой таблицы.
            gridMode: false,

            // Мобилка: кнопка «Инструкция» сжата до иконки, когда заголовок+инструкция
            // не влезают в одну строку (measureHeader). Десктоп всегда с текстом.
            instructionCompact: false,

            // Мобилка (#1097 S6): развёрнута ли подсказка таблицы «по факту». По
            // умолчанию да - до первого сворачивания экран выглядит как раньше.
            hintExpanded: true,
        };
    },
    computed: {
        tableSystemName() {
            return this.tableData?.table?.name || '';
        },
        
        tableDisplayName() {
            return this.tableData?.table?.display_name || 'Загрузка...';
        },
        
        tableType() {
            return this.tableData?.table?.table_type || '';
        },
        
        // Только машины: у сотрудников тумблера "по факту" нет (#2019).
        showFactTable() {
            return this.tableData?.table?.table_type === 'cars' && !!this.tableData?.table?.show_fact_table;
        },
        
        tableFactHint() {
            return this.tableData?.table?.fact_table_hint || '';
        },
        
        tableInstruction() {
            return this.tableData?.table?.instruction || '';
        },
        
        showOrganizationFilter() {
            return this.tableType === 'cars' || this.tableType === 'people';
        },
        
        showUnloadingFilter() {
            return this.tableType === 'cars';
        },

        // Кнопка "Добавить вручную" (#1049): для cars - право entity.cars.manual_add,
        // для people - entity.employees.manual_add (super/admin проходят авто). Ключ
        // зеркалит BE-гейт роутов /cars/manual и /employees/manual (RequirePermissionV2).
        canManualAdd() {
            if (this.tableType === 'cars') return this.can('entity.cars.manual_add');
            if (this.tableType === 'people') return this.can('entity.employees.manual_add');
            return false;
        },

        // Права на действия таблицы - одним местом на обе раскладки: десктопный ряд
        // и мобильный лист иначе разъедутся по составу при следующей правке.
        canVersions() {
            return this.can(`table.${this.$route.params.tableName}.versions`);
        },

        canTrash() {
            return this.can(`table.${this.$route.params.tableName}.trash`);
        },

        canExport() {
            return this.can(`table.${this.$route.params.tableName}.export`);
        },

        canReport() {
            return this.can(`table.${this.$route.params.tableName}.report`);
        },

        // История таблицы. На десктопе её открывает кнопка в шапке самой таблицы, на
        // телефоне шапка - один ряд в 48px, и кнопка уехала в лист «⋯» (волна 6).
        canTableHistory() {
            return this.can(`table.${this.$route.params.tableName}.history`);
        },

        // Есть ли что показывать в листе «⋯»: без единого доступного действия кнопка
        // открывала бы пустое окно.
        hasSheetActions() {
            return this.canVersions || this.canTrash || this.canExport || this.canReport
                || (this.isNarrow && this.canTableHistory);
        },

        // Нижняя панель существует ради главного действия; «⋯» в ней - дубль кнопки
        // из шапки, ради одного его панель не поднимаем.
        showFabBar() {
            return this.isNarrow && this.canManualAdd;
        },

        sanitizedHint() {
            return this.sanitizeHtml(this.tableFactHint);
        },
        
        sanitizedInstruction() {
            return this.sanitizeHtml(this.tableInstruction);
        },

        // Конфиг мультивыборных фильтров-справочников (#1398), зеркало
        // ApplicationsCenter.directoryFilters. Один источник для десктопного ряда и
        // мобильного листа: состав задаётся здесь, поэтому две ветви шаблона не могут
        // разойтись. summaryLabel идёт и в счётчик кнопки («Организация: 3»), и в
        // подпись секции листа.
        directoryFilters() {
            const filters = [];
            if (this.showOrganizationFilter) {
                filters.push(
                    {
                        field: 'selectedOrganizationIds',
                        values: this.selectedOrganizationIds,
                        options: this.organizations,
                        allLabel: 'Все организации',
                        summaryLabel: 'Организация',
                        testid: 'table-sheet-org',
                    },
                    {
                        field: 'selectedCompanyIds',
                        values: this.selectedCompanyIds,
                        options: this.companies,
                        allLabel: 'Все компании',
                        summaryLabel: 'Компания',
                        testid: 'table-sheet-company',
                    },
                );
            }
            if (this.showUnloadingFilter) {
                filters.push({
                    field: 'selectedUnloadingPlaceIds',
                    values: this.selectedUnloadingPlaceIds,
                    options: this.unloadPlaces,
                    allLabel: 'Все места разгрузки',
                    summaryLabel: 'Место разгрузки',
                    // Код площадки живёт в description: без этого поиск по коду,
                    // работавший до переезда на BaseDropdown, молча сузился бы.
                    searchKeys: ['name', 'description'],
                    testid: 'table-sheet-place',
                });
            }
            return filters;
        },

        // Вторичные фильтры (без поиска) - для точки-индикатора на кнопке «Фильтр»
        // на мобилке: поиск виден отдельно, точка отражает только свёрнутые фильтры.
        secondaryFiltersActive() {
            return this.selectedOrganizationIds.length > 0 ||
                   this.selectedCompanyIds.length > 0 ||
                   this.selectedUnloadingPlaceIds.length > 0 ||
                   !!this.selectedDate ||
                   !!(this.dateRangeStart && this.dateRangeEnd);
        },

        hasActiveFilters() {
            return !!this.searchQuery.trim() || this.secondaryFiltersActive;
        }
    },
    watch: {
        /**
         * Онбординг просит показать отчёт по проходам: открываем окно по сигналу и
         * закрываем, когда сигнал гаснет. Окно, открытое человеком, не трогаем.
         */
        'onboardingStore.revealOpen'(target) {
            if (target === 'pass-report') {
                if (this.showPassReport) return;
                this.passReportOpenedByTour = true;
                this.showPassReport = true;
                return;
            }
            if (!this.passReportOpenedByTour) return;
            this.passReportOpenedByTour = false;
            this.showPassReport = false;
        },

        '$route.params.tableName': {
            handler() {
                this.fetchTableData();
                this.clearFilters();
                this.loadGridMode();
                this.loadHintExpanded();
            },
            immediate: true
        },
        gridMode(value) {
            this.saveGridMode(value);
        },
        // Имя таблицы или переход десктоп<->мобилка меняют, влезает ли инструкция
        // с текстом в строку заголовка - пересчитываем.
        tableDisplayName() {
            this.measureHeader();
        },
        isNarrow(value) {
            // Возврат на десктоп - гасим мобильное раскрытие поиска, иначе оверлей
            // остаётся смонтированным поверх инлайн-ряда фильтров (#1097 S7).
            // Лист действий гасим по той же причине: его кнопки на десктопе снова
            // в шапке, а окно осталось бы висеть.
            if (!value) {
                this.showMobileSearch = false;
                this.showActionsSheet = false;
            }
            this.measureHeader();
        }
    },
    async mounted() {
        window.addEventListener('resize', this.measureHeader);
        this.measureHeader();
        await this.fetchCurrentUser();  // Ждём загрузки пользователя
        this.fetchTableData();          // потом таблицы
    },
    beforeUnmount() {
        window.removeEventListener('resize', this.measureHeader);
    },
    methods: {
        /**
         * На мобилке заголовок «Таблица X» и кнопка «Инструкция» стоят в одну строку.
         * Если полный заголовок не помещается рядом с инструкцией-с-текстом (обрезается
         * ellipsis), сжимаем инструкцию до иконки, отдавая место заголовку. На десктопе
         * инструкция всегда с текстом.
         */
        measureHeader() {
            if (!this.isNarrow) {
                this.instructionCompact = false;
                return;
            }
            // Мерим в развёрнутом состоянии (текст виден), решение принимаем в nextTick.
            this.instructionCompact = false;
            this.$nextTick(() => {
                const title = this.$refs.titleRef;
                if (!title || !this.isNarrow) return;
                this.instructionCompact = title.scrollWidth > title.clientWidth + 1;
            });
        },

        // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
        // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },

        // Ключ от имени таблицы в роуте, а не от tableData.table.id: id приезжает
        // асинхронно, тумблер успел бы моргнуть до загрузки настроек таблицы.
        gridStorageKey() {
            return `grid-mode:${this.$route.params.tableName || 'default'}`;
        },

        loadGridMode() {
            try {
                this.gridMode = localStorage.getItem(this.gridStorageKey()) === '1';
            } catch {
                this.gridMode = false;
            }
        },

        saveGridMode(value) {
            try {
                localStorage.setItem(this.gridStorageKey(), value ? '1' : '0');
            } catch {
                // localStorage недоступен (приватный режим) - режим просто не запомнится.
            }
        },

        // Подсказка «по факту» (#1097 S6). Ключ от имени таблицы в роуте - по тем же
        // причинам, что у режима «Сетка» выше.
        hintStorageKey() {
            return `fact-hint-open:${this.$route.params.tableName || 'default'}`;
        },

        // Развёрнуто по умолчанию: отсутствующий ключ читается как '1', свёрнутым
        // блок становится только после явного действия пользователя.
        loadHintExpanded() {
            try {
                this.hintExpanded = localStorage.getItem(this.hintStorageKey()) !== '0';
            } catch {
                this.hintExpanded = true;
            }
        },

        toggleHint() {
            this.hintExpanded = !this.hintExpanded;
            try {
                localStorage.setItem(this.hintStorageKey(), this.hintExpanded ? '1' : '0');
            } catch {
                // localStorage недоступен (приватный режим) - состояние не запомнится.
            }
        },
        /**
         * Действие из листа «⋯»: сначала закрываем лист, потом выполняем. Иначе
         * окно экспорта/отчёта открывается ПОД листом - оба лежат на базовом слое
         * модалок, и поздний оказывается не сверху, а рядом.
         *
         * @param {Function} action
         */
        runSheetAction(action) {
            this.showActionsSheet = false;
            action();
        },

        /**
         * Открывает историю той таблицы, что сейчас на экране: у машин и у людей она
         * своя - со своей моделью, фильтрами и выгрузкой, - и живёт внутри компонента
         * таблицы. Рендерится ровно один из двух, поэтому и ref всегда один.
         */
        openTableHistory() {
            const cars = this.$refs.carsTable;
            if (cars) {
                cars.openCarsTableHistory();
                return;
            }
            const people = this.$refs.peopleTable;
            if (people) people.openEmployeesHistory();
        },

        handleApplicationUpdate() {
            this.refreshData();
        },

        async fetchCurrentUser() {
            try {
                const response = await apiRequest("/users/me", {
                });

                if (response.ok) {
                    const userData = await response.json();
                    this.currentUserId = userData.id;

                    const nameParts = [userData.last_name, userData.first_name, userData.middle_name].filter(Boolean);
                    this.currentUserName = nameParts.join(' ') || userData.username;
                }
            } catch (error) {
                console.error("Ошибка при загрузке данных пользователя:", error);
            }
        },

        async refreshData() {
            await this.fetchTableData();
            this.$emit('refresh-data');
        },

        // Ручные записи появились в текущей и fact-таблице (#1049) - перегружаем строки
        // напрямую (real-time #840 продублирует по target-таблице, здесь для мгновенного отклика).
        onManualAdded() {
            this.$refs.carsTable?.loadData?.();
            this.$refs.peopleTable?.loadData?.();
            this.$refs.factTable?.loadData?.();
        },
        
        sanitizeHtml(content) {
            return sanitizeHtml(content);
        },

        async fetchTableData() {
            const tableName = this.$route.params.tableName;
            if (!tableName) return;
            
            try {
                const response = await apiRequest(`/system-tables/name/${tableName}`, {
                });
                
                if (response.ok) {
                    const data = await response.json();
                    this.tableData = data;

                    await this.fetchOrganizationsForTable();
                    await this.fetchCompaniesForTable();
                    if (this.showUnloadingFilter) await this.fetchUnloadPlacesForTable();
                } else {
                    console.error('Table not found');
                    this.$router.push('/404');
                }
            } catch (error) {
                console.error("Error fetching table data:", error);
            }
        },

        async fetchOrganizationsForTable() {
            try {
                const response = await apiRequest("/organizations", {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    this.organizations = data;
                } else {
                    console.error("Ошибка при загрузке организаций");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке организаций:", error);
            }
        },

        async fetchCompaniesForTable() {
            try {
                const response = await apiRequest("/companies", {
                    method: "GET",
                });

                if (response.ok) {
                    this.companies = await response.json();
                } else {
                    console.error("Ошибка при загрузке компаний");
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке компаний:", error);
            }
        },

        /**
         * Справочник мест разгрузки для фильтра (#1398). Архивные записи отсеиваем,
         * как в Центре: фильтровать по выведенной из работы площадке смысла нет.
         *
         * Зовётся из fetchTableData, а не из mounted: справочник нужен только когда
         * известен тип таблицы (cars), а fetchTableData срабатывает ещё и на смену
         * таблицы в роуте.
         */
        async fetchUnloadPlacesForTable() {
            try {
                const response = await apiRequest("/unload-places", { method: "GET" });
                if (!response.ok) {
                    console.error("Ошибка при загрузке мест разгрузки");
                    return;
                }
                const data = await response.json();
                this.unloadPlaces = (Array.isArray(data) ? data : []).filter(p => p.is_active !== false);
            } catch (error) {
                console.error("Ошибка сети при загрузке мест разгрузки:", error);
            }
        },

        // Единая точка для всех мультивыборных фильтров (#1398): записать массив
        // и применить. Зеркало ApplicationsCenter.setMultiFilter.
        setMultiFilter(field, values) {
            this[field] = Array.isArray(values) ? values : [];
            this.applyFilters();
        },

        updateSelectedDate(date) {
            this.selectedDate = date;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
        },
        
        updateDateRangeStart(date) {
            this.dateRangeStart = date;
            this.selectedDate = null;
        },
        
        updateDateRangeEnd(date) {
            this.dateRangeEnd = date;
            this.selectedDate = null;
        },
        
        applyDateFilters() {
            this.applyFilters();
        },
        
        clearDate() {
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;
            this.applyFilters();
        },
        
        applyFilters() {
            // Фильтры применяются автоматически через props в дочерних компонентах
        },

        /**
         * Мобилка (#1097 S7): тоггл оверлея поиска. Фокус переносим в поле только при
         * раскрытии по клику - автофокуса при загрузке страницы нет (он на мобилке
         * выбрасывает клавиатуру поверх списка).
         */
        toggleMobileSearch() {
            this.showMobileSearch = !this.showMobileSearch;
            if (this.showMobileSearch) {
                this.$nextTick(() => {
                    this.$refs.mobileSearchInput?.focus();
                });
            }
        },

        clearMobileSearch() {
            this.searchQuery = '';
            this.showMobileSearch = false;
            this.applyFilters();
        },
        
        clearFilters() {
            this.searchQuery = '';

            // Прямой сброс state - работает и когда sheet закрыт (на мобилке вторичные
            // фильтры размонтированы, refs недоступны, а clearFilters зовётся ещё и при
            // смене таблицы). Дочерние следят за пропсами и подхватят сброс.
            // unloadPlaces НЕ чистим: это справочник опций, а не выбор.
            this.selectedOrganizationIds = [];
            this.selectedCompanyIds = [];
            this.selectedUnloadingPlaceIds = [];
            this.selectedDate = null;
            this.dateRangeStart = null;
            this.dateRangeEnd = null;

            // BaseDropdown полностью управляется пропом modelValue, метода reset() у него
            // нет - сброса состояния выше достаточно. У календаря своё выделение.
            this.$refs.dateFilter?.clearSelection?.();
        },

        async handleOpenApplication(applicationId) {
            try {
                const response = await apiRequest(`/applications/${applicationId}/details`, {});
                if (response.ok) {
                    const appData = await response.json();
                    this.selectedApplication = appData;
                    this.showApplicationDetail = true;
                } else {
                    console.error('Не удалось загрузить заявку');
                }
            } catch (error) {
                console.error('Ошибка загрузки заявки:', error);
            }
        },

        closeApplicationDetail() {
            this.showApplicationDetail = false;
            this.selectedApplication = null;
        },

        handleApplicationChanged() {
            this.refreshData();
        },

        handleExport() {
            if (this.showFactTable) {
                // Показываем модалку выбора только когда есть факт-таблица
                this.showExportModal = true;
            } else {
                // Только основная таблица — экспортируем сразу без диалога
                this.exportMainTable();
            }
        },

        async handleExportChoice(choice) {
            if (choice === 'both' || choice === 'fact') {
                const factRef = this.$refs.factTable;
                if (factRef && typeof factRef.exportToExcel === 'function') {
                    await factRef.exportToExcel();
                }
            }
            if (choice === 'both' || choice === 'main') {
                await this.exportMainTable();
            }
        },

        async exportMainTable() {
            const mainRef = this.$refs.carsTable || this.$refs.peopleTable;
            if (mainRef && typeof mainRef.exportToExcel === 'function') {
                await mainRef.exportToExcel();
            }
        },
    }
}
</script>

<style scoped>
/* Все стили остаются без изменений */
/* Отступ страницы - общий токен (#1097 S7, эталон §1.3): 20px базово, 16 на <=1024,
   12 на <=768, 10 на <=480. Хардкод 20px забирал на 320px по 20px с каждой стороны,
   и ряд действий из среза 6 собирался впритык. */
.tables {
    padding: var(--gutter);
    position: relative;
}

/* Панель главного действия прижата к низу экрана (position: fixed), поэтому её
   высоту резервирует контент - иначе последняя карточка списка уезжает под неё.
   64px = 8 сверху + кнопка 44 + 12 снизу. */
.tables--fab {
    padding-bottom: calc(var(--gutter) + 64px + env(safe-area-inset-bottom, 0px));
}

.tables__fab-bar {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    /* Выше содержимого страницы, ниже шапки (100) и всех окон (от 1000). */
    z-index: 90;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px var(--gutter) calc(12px + env(safe-area-inset-bottom, 0px));
    background: var(--surface);
    border-top: 1px solid var(--border);
}

.tables__fab-main {
    flex: 1 1 auto;
    min-width: 0;
    height: 44px;
    border: none;
    border-radius: var(--radius-pill);
    background: var(--accent);
    color: var(--accent-contrast);
    font-size: 15px;
    font-weight: 700;
    cursor: pointer;
}

.tables__fab-more {
    flex: 0 0 auto;
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius-pill);
    background: var(--surface);
    color: var(--text);
    font-size: 18px;
    line-height: 1;
    cursor: pointer;
}

/* Лист действий: строки 48px с полными подписями - ради них действия и уехали из
   шапки, где от них оставались иконки 36x36 без текста. */
.actions-sheet__list {
    display: flex;
    flex-direction: column;
    padding: 4px 0 8px;
}

.actions-sheet__item {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    min-height: 48px;
    padding: 0 20px;
    border: none;
    background: transparent;
    color: var(--text);
    font: inherit;
    font-size: 15px;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
}

.actions-sheet__item:active {
    background: var(--surface-2);
}

.actions-sheet__item--danger {
    color: var(--danger-text);
}

.actions-sheet__icon {
    color: var(--text);
    width: 20px;
    height: 20px;
    flex-shrink: 0;
}

.tables__title {
    font-size: 18px;
    font-weight: bold;
    color: var(--text);
}

.table-name {
    color: var(--accent-text);
}

.tables__header {
    display: flex;
    gap: 10px;
    padding-bottom: 15px;
    align-items: center;
    flex-wrap: wrap;
}

.tables__header-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-left: auto;
    flex-wrap: wrap;
    justify-content: flex-end;
}

.tables__instruction {
    width: fit-content;
    font-size: 14px;
    font-weight: 500;
    color: var(--accent-text);
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    border-radius: 50px;
    background: var(--surface);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    height: 25px;
    transition: all 0.2s ease;
}

.tables__instruction:hover {
    background-color: var(--surface-2);
    border-color: var(--accent);
}

.tables__icon {
    width: 15px;
    height: 15px;
    color: var(--text);
    /* Глиф рисуется на 15px: общая обводка 1.7 при поле 24 даёт на экране 1.06px -
       волосок против прежнего залитого растра. */
    stroke-width: 2.2;
}

/* Значок руководства был фирменного синего и остаётся им: он ведёт к инструкции,
   а не к данным таблицы. */
.tables__icon--accent {
    color: var(--accent-text);
}

.tables__filters {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 15px;
    border-bottom: 1px solid var(--border);
    gap: clamp(6px, 0.8vw, 10px);
    min-width: 0;
    flex-wrap: nowrap;
}

/* clamp на gap/padding/height/font-size - адаптивное масштабирование без
   media-queries. Фильтры всегда в одну строку: на узком экране становятся
   мельче, но не переносятся и не уезжают. */
.filters__fields {
    display: flex;
    align-items: center;
    gap: clamp(6px, 0.8vw, 10px);
    position: relative;
    min-width: 0;
    flex-wrap: nowrap;
    flex-shrink: 1;
}


.field {
    width: clamp(120px, 14vw, 200px);
    height: clamp(28px, 3vw, 35px);
    background-color: var(--surface);
    border-radius: 15px;
    border: 1px solid var(--border);
    padding: 0 clamp(6px, 0.8vw, 10px);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: clamp(4px, 0.6vw, 10px);
    position: relative;
    cursor: pointer;
    flex-shrink: 1;
    min-width: 0;
}

/* Обёртка дропдауна-фильтра (#1398). У .base-dropdown своей ширины нет, поэтому без
   обёртки ряд дёргался бы при каждой смене подписи («Все организации» -> «Организация: 3»).
   Размеры один в один с .field: ряд сжимается в лад с полем поиска, как до переезда на
   BaseDropdown. min-width:0 обязателен - ряд nowrap, и несжимаемый минимум выдавливает
   дропдауны поверх правой группы кнопок (замер: перекрытие 265px на 769px при 120px). */
.filters__control {
    width: clamp(120px, 14vw, 200px);
    min-width: 0;
    flex-shrink: 1;
}

/* Кнопка дропдауна под контракт ряда: те же clamp-высота, радиус 15px и размер шрифта,
   что у .field и календаря. :deep обязателен - кнопка живёт внутри дочернего компонента
   и хэша этого файла не несёт. min-height:0 гасит собственные 30px BaseDropdown, иначе
   они перебивают низ clamp. */
.filters__control :deep(.base-dropdown__button) {
    height: clamp(28px, 3vw, 35px);
    min-height: 0;
    border-radius: 15px;
    padding: 0 clamp(6px, 0.8vw, 10px);
    gap: clamp(4px, 0.6vw, 10px);
}

.filters__control :deep(.base-dropdown__text) {
    font-size: clamp(11px, 1.1vw, 14px);
}

/* Мобильный лист: своего бокса дропдаун в слоте FilterSheet не получает - задаём
   высоту 40px как у календаря рядом. Правило висит на .filter-section (элемент
   слота, несёт хэш этого файла и стоит в реальной DOM-цепочке под teleport'ом
   BaseModal); на .filter-sheet оно было бы мёртвым.
   Вне @media намеренно: секция рендерится только при isNarrow, и брейкпоинт CSS не
   должен расходиться с JS-условием. */
.filter-section :deep(.base-dropdown__button) {
    height: 40px;
    min-height: 0;
    border-radius: var(--radius-md);
    padding: 0 12px;
}

.filter-section :deep(.base-dropdown__text) {
    font-size: 14px;
}

.field__input {
    outline: none;
    border: none;
    background-color: transparent;
    font-size: clamp(11px, 1.1vw, 14px);
    width: 100%;
    min-width: 0;
    cursor: pointer;
}

.search {
    cursor: text;
}

.filters__options {
    display: flex;
    align-items: center;
    gap: clamp(6px, 1vw, 15px);
    justify-content: flex-end;
    flex-wrap: nowrap;
    flex-shrink: 0;
}

.options__icon {
    width: clamp(14px, 1.6vw, 20px);
    height: clamp(14px, 1.6vw, 20px);
    cursor: pointer;
    flex-shrink: 0;
    color: var(--text);
    stroke-width: 2;
}

.options__versions-link,
.options__trash-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    flex-shrink: 0;
    text-decoration: none;
    color: var(--text);
}

/* Десктоп: версии/корзина - только иконка (места хватает, подпись избыточна).
   На мобилке (см. @media) подпись показывается, и все действия становятся
   одинаковыми кнопками с текстом. */
.options__versions-link .options__text,
.options__trash-link .options__text {
    display: none;
}

.options__export {
    width: clamp(70px, 8vw, 100px);
    height: clamp(22px, 2.4vw, 25px);
    background: var(--surface);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    gap: 5px;
    font-size: clamp(10px, 1vw, 13px);
}

.options__export:hover {
    background: var(--surface-2);
}

.options__manual-add {
    height: clamp(22px, 2.4vw, 25px);
    padding: 0 clamp(8px, 1vw, 14px);
    background: var(--surface);
    color: var(--text);
    border: 1px solid var(--border);
    outline: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    gap: 5px;
    font-size: clamp(10px, 1vw, 13px);
    flex-shrink: 0;
    white-space: nowrap;
}

.options__manual-add:hover {
    background: var(--surface-2);
}

.options__manual-add-icon {
    font-size: clamp(13px, 1.3vw, 16px);
    line-height: 1;
    font-weight: 600;
}

.options__text {
    font-weight: 500;
}

.tables__content {
    margin-top: 15px;
    display: flex;
    flex-direction: column;
    gap: 20px;
}

/* Fact Section with Hint */
.fact-section {
    display: flex;
    gap: 20px;
}

.fact-hint-card {
    flex: 0 0 35%;
    /* Карточка не задаёт высоту секции: её текст лежит в absolute-слое, поэтому в
       поток идёт только рамка, а высоту строки диктует таблица «по факту». Длинная
       подсказка прокручивается внутри, а не вытягивает блок выше таблицы. */
    position: relative;
    overflow: hidden;
    /* Через токены: в светлых темах акцентная плашка, в тёмных - тёмная карточка
       в тон темы (сплошной синий блок на тёмном фоне читался плохо). Тени нет:
       крупные карточки страниц отделяет рамка, тень оставлена окнам и всплывающему. */
    background-color: var(--hint-card-bg);
    border: 1px solid var(--hint-card-border);
    border-radius: 30px;
    /* Отступы поджаты: высота карточки равна таблице «по факту» (у той жёсткие
       222px), и на прежних 20px текущая подсказка не помещалась на десяток
       пикселей - последняя строка уходила в прокрутку. */
    padding: 14px 16px;
    display: flex;
    gap: 15px;
    align-items: flex-start;
}

.hint-content {
    position: absolute;
    inset: 14px 16px;
    overflow-y: auto;
    scrollbar-width: thin;
    /* Плотнее строка - в ту же высоту входит больше текста. Прокрутка остаётся
       на случай подсказки длиннее таблицы. */
    line-height: 1.35;
    color: var(--hint-card-text);
}

.reset-filters-btn {
    padding: 6px 12px;
    border: 1px solid var(--border);
    background: var(--danger-bg);
    border-radius: 15px;
    cursor: pointer;
    font-size: 12px;
    transition: all 0.2s;
    height: 35px;
    color: var(--danger-text);
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 5px;
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.reset-filters-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 22%, var(--surface));
}

.reset-filters-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    background: var(--surface-2);
    color: var(--text-muted);
    border-color: var(--border);
}

.reset-filters-btn:disabled .options__icon {
    filter: grayscale(100%);
}

/* Text Constructor Content Styles */
.text-constructor-content :deep(*) {
    line-height: 150%;
    overflow-wrap: break-word;
}

.text-constructor-content :deep(img) {
    max-width: 100%;
    border-radius: 8px;
}

.text-constructor-content :deep(img:not([height])) {
    height: auto;
}

.text-constructor-content :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.text-constructor-content :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.text-constructor-content :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }
.text-constructor-content::after { content: ''; display: block; clear: both; }

.text-constructor-content :deep(strong) {
    font-weight: 600 !important;
}

.text-constructor-content :deep(em) {
    font-style: italic !important;
}

.text-constructor-content :deep(u) {
    text-decoration: underline !important;
}

.text-constructor-content :deep(ul),
.text-constructor-content :deep(ol) {
    padding-left: 20px !important;
}

.text-constructor-content :deep(li) {
    line-height: 150% !important;
}

.text-constructor-content :deep(h1) {
    font-size: 20px !important;
    font-weight: 700 !important;
    margin: 0 0 12px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(h2) {
    font-size: 18px !important;
    font-weight: 600 !important;
    margin: 0 0 10px 0 !important;
    line-height: 1.3 !important;
}

.text-constructor-content :deep(h3) {
    font-size: 16px !important;
    font-weight: 500 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.4 !important;
}

.text-constructor-content :deep(.black-text) { color: #000 !important; }
.text-constructor-content :deep(.red-text) { color: #FF0000 !important; }
.text-constructor-content :deep(.green-text) { color: #079D1D !important; }
.text-constructor-content :deep(.blue-text) { color: var(--accent-text) !important; }

.text-constructor-content :deep(.font-size-10) { font-size: 10px !important; }
.text-constructor-content :deep(.font-size-12) { font-size: 12px !important; }
.text-constructor-content :deep(.font-size-14) { font-size: 14px !important; }
.text-constructor-content :deep(.font-size-16) { font-size: 16px !important; }
.text-constructor-content :deep(.font-size-18) { font-size: 18px !important; }
.text-constructor-content :deep(.font-size-20) { font-size: 20px !important; }

.text-constructor-content :deep(.font-weight-300) { font-weight: 300 !important; }
.text-constructor-content :deep(.font-weight-400) { font-weight: 400 !important; }
.text-constructor-content :deep(.font-weight-500) { font-weight: 500 !important; }
.text-constructor-content :deep(.font-weight-600) { font-weight: 600 !important; }
.text-constructor-content :deep(.font-weight-900) { font-weight: 900 !important; }

.text-constructor-content :deep(.heading-h1) { 
    font-size: 24px !important;
    font-weight: 700 !important;
    margin: 0 0 8px 0 !important;
    line-height: 1.2 !important;
}

.text-constructor-content :deep(.heading-h2) {
    font-size: 20px !important;
    font-weight: 600 !important;
    margin: 8px 0 6px 0 !important;
    line-height: 1.3 !important;
}

.text-constructor-content :deep(.text-align-left) { text-align: left !important; }
.text-constructor-content :deep(.text-align-center) { text-align: center !important; }
.text-constructor-content :deep(.text-align-right) { text-align: right !important; }

/* Hint specific styles */
.hint-content :deep(*) {
    color: var(--hint-card-text) !important;
}

/* Плотнее межстрочный интервал, чем общий 150% текстового конструктора: карточка
   ограничена высотой таблицы «по факту», и на 150% текущая подсказка не помещалась.
   Оба класса на одном элементе - специфичность выше общего правила. */
.hint-content.text-constructor-content :deep(*) {
    line-height: 1.35;
}

.hint-content :deep(.black-text) { color: var(--hint-card-text) !important; }
.hint-content :deep(.red-text) { color: #FFB3B3 !important; }
.hint-content :deep(.green-text) { color: #B3FFC6 !important; }
.hint-content :deep(.blue-text) { color: #B3C6FF !important; }

.hint-content :deep(.text-align-left) { text-align: left !important; }
.hint-content :deep(.text-align-center) { text-align: center !important; }
.hint-content :deep(.text-align-right) { text-align: right !important; }

/* Instruction Modal */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 20px;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.instruction-modal-large {
    background: var(--surface);
    border-radius: 30px;
    padding: 0;
    max-width: 700px;
    width: 100%;
    max-height: calc(var(--app-vh, 1vh) * 80);
    box-shadow: 0 10px 30px var(--shadow-drop);
    display: flex;
    flex-direction: column;
}

.instruction-modal-enter-active,
.instruction-modal-leave-active {
    transition: opacity 0.2s ease;
}

.instruction-modal-enter-active .instruction-modal-large,
.instruction-modal-leave-active .instruction-modal-large {
    transition: transform 0.2s ease-out, opacity 0.2s ease-out;
}

.instruction-modal-enter-from,
.instruction-modal-leave-to {
    opacity: 0;
}

.instruction-modal-enter-from .instruction-modal-large,
.instruction-modal-leave-to .instruction-modal-large {
    transform: translateY(20px);
    opacity: 0;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
}

.modal-header h3 {
    margin: 0;
    font-size: 1.2em;
    font-weight: 600;
    color: var(--text);
}

.modal-close {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s ease;
}

.modal-close:hover {
    color: var(--text);
}

.instruction-content {
    padding: 20px;
    overflow-y: auto;
    flex: 1;
}

.instruction-content .text-constructor-content :deep(h1) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(h2) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(h3) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(p) {
    color: var(--text) !important;
}

.instruction-content .text-constructor-content :deep(li) {
    color: var(--text) !important;
}

.no-instruction {
    text-align: center;
    padding: 40px 20px;
    color: var(--text-muted);
}

.no-instruction-icon {
    font-size: 48px;
    margin-bottom: 16px;
    opacity: 0.5;
}

.no-instruction h4 {
    margin: 0 0 12px 0;
    color: var(--text);
    font-size: 1.2em;
}

.no-instruction p {
    margin: 0 0 8px 0;
    line-height: 1.5;
}

.blue {
    color: var(--accent-text);
}

/* 767.98, а не 768 (#1097 S7): страница стоит поверх card-правил
   responsive-tables.css с тем же порогом, и JS-гейт isNarrow переведён туда же.
   На ровно 768 (портретный iPad) правила расходились, и экран собирался гибридом -
   эталон §1.2. */
@media (max-width: 767.98px) {
    /* Модель прокрутки телефона (четвёртый круг замечаний владельца): "скроллится
       вся страница, кроме шапки" - панель фиксированной высоты с внутренней
       прокруткой (волна 13) владелец забраковал вместе с той, что была до неё
       (документ короче вьюпорта на первой загрузке). Возврат к прокрутке
       документа: `.tables` в обычном потоке, своей высоты и overflow не задаёт -
       скроллит страница целиком, как остальные списки (эталон §9, Центр и
       кабинет). Заголовок, инструкция, фильтры и поиск закреплены общим блоком
       `.tables__sticky-head` (см. ниже) под app bar (TheHeader, sticky top:0,
       --mobile-header-height), тот же приём, что у `.center__header`. */
    .tables__sticky-head {
        position: sticky;
        top: var(--mobile-header-height, 55px);
        z-index: 20;
        /* Непрозрачный фон обязателен - без него текст, уехавший под шапку при
           скролле, просвечивал бы сквозь неё. Токен страницы (не --surface):
           у `.tables` своего фона нет, страница и так того же цвета. */
        background: var(--bg);
        /* Отступ страницы уводим ВНУТРЬ липкого блока. Снаружи он оставлял щель
           между шапкой приложения и блоком, и блок сперва проезжал её и только
           потом прилипал - это и читается как «уехало на пять пикселей». Заодно
           фон дотягивается до краёв экрана, а не обрывается по отступу. */
        margin: calc(var(--gutter) * -1) calc(var(--gutter) * -1) 0;
        padding: var(--gutter) var(--gutter) 12px;
        border-bottom: 1px solid var(--border);
    }

    .fact-section {
        flex-direction: column;
    }

    /* Вертикальные отступы карточки на телефоне отдаём содержимому: в свёрнутом
       виде она вся состоит из одного переключателя, и padding 14px превращал полосу
       подсказки в блок 74px. Своей высоты у неё теперь нет - ровно тач-таргет
       переключателя (44px) плюс рамка, а в развёрнутом отступ снизу добирает текст. */
    .fact-hint-card {
        flex: none;
        width: 100%;
        overflow: visible;
        flex-direction: column;
        /* stretch перебивает align-items: flex-start десктопной карточки: в колонке
           он ужал бы переключатель и текст до ширины содержимого. */
        align-items: stretch;
        gap: 0;
        padding: 0 16px;
        transition: background-color 0.22s ease, border-color 0.22s ease;
    }

    /* Свёрнутая - тонкая подсказка, а не акцентная плашка: заливка акцентом во всю
       ширину читалась как главный элемент экрана над таблицей поста. Остаётся тот
       же акцент, но примесью (8% на поверхности) и текстом, а не фоном. Развёрнутая
       заливку сохраняет - там она отделяет чужой текст из конструктора от таблицы. */
    .fact-hint-card--collapsed {
        background-color: var(--accent-tint-solid);
        border-color: color-mix(in srgb, var(--accent) 30%, var(--surface));
        /* --hint-card-text в светлой теме = --accent-contrast (белый) - рассчитан на
           сплошную акцентную заливку развёрнутой карточки, а не на 8%-подмес выше.
           На нём подпись «Подсказка» пропадала (белое на белом). Переопределяем саму
           ПЕРЕМЕННУЮ в области видимости свёрнутого состояния, а не color в
           .fact-hint-card__toggle - тот текстом задаёт единый вид для обоих состояний
           (замок TablesComponent.factHint.spec.js) и должен остаться нетронутым. */
        --hint-card-text: var(--accent-text);
    }

    .hint-content {
        position: static;
        inset: auto;
        overflow: visible;
    }

    /* Сворачиваемая подсказка (#1097 S6). Высота анимируется через
       grid-template-rows 0fr -> 1fr (эталон §3.3: display и height не
       анимируются вовсе), стрелка - transform. Внутренний слой с overflow:hidden
       обязателен: padding содержимого иначе остаётся видимой полосой при нулевой
       строке. */
    .fact-hint-card__toggle {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
        width: 100%;
        /* Тач-таргет 44px (WCAG 2.5.5) - вся ширина карточки, промахнуться негде.
           Он же задаёт высоту свёрнутой подсказки: своих отступов у карточки нет. */
        min-height: 44px;
        padding: 0;
        background: none;
        border: 0;
        outline: none;
        cursor: pointer;
        font-size: 14px;
        font-weight: 600;
        color: var(--hint-card-text);
        transition: color 0.22s ease;
    }

    /* Стрелка нарисована смотрящей вправо, поэтому «развёрнуто» - это повёрнутая
       вниз стрелка, а свёрнутое состояние оставляет её как есть. */
    .fact-hint-card__chevron {
        width: 16px;
        height: 16px;
        flex-shrink: 0;
        transform: rotate(90deg);
        transition: transform 0.2s ease;
    }

    .fact-hint-card--collapsed .fact-hint-card__chevron {
        transform: rotate(0deg);
    }

    .fact-hint-card__collapse {
        display: grid;
        grid-template-rows: 1fr;
        transition: grid-template-rows 0.22s ease;
    }

    .fact-hint-card--collapsed .fact-hint-card__collapse {
        grid-template-rows: 0fr;
    }

    .fact-hint-card__collapse-inner {
        min-height: 0;
        overflow: hidden;
    }

    /* Нижний отступ развёрнутой подсказки: у карточки своего больше нет, иначе текст
       упирался бы в её нижнюю кромку. Задан содержимому, а не карточке, - иначе он
       остался бы видимой полосой и в свёрнутом виде. */
    .fact-hint-card__collapse-inner .hint-content {
        padding: 6px 0 14px;
    }

    /* Заголовок и «Инструкция» - в одну строку; длинное имя ужимается ellipsis,
       инструкция сжимается до иконки через measureHeader (не переносится). */
    .tables__header {
        flex-direction: row;
        flex-wrap: nowrap;
        align-items: center;
        gap: 8px;
    }

    .tables__title {
        flex: 1 1 auto;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .tables__instruction {
        flex-shrink: 0;
    }

    .tables__instruction--compact {
        width: 25px;
        padding: 0;
        gap: 0;
        justify-content: center;
    }

    .tables__filters {
        flex-direction: column;
        gap: 15px;
        align-items: flex-start;
        /* Линию рисует сам липкий блок - он растянут до краёв экрана, а поле
           фильтров обрывалось бы по отступу страницы. */
        border-bottom: none;
        padding-bottom: 0;
    }

    .filters__fields {
        flex-wrap: nowrap;
        width: 100%;
        gap: 10px;
    }

    .field {
        width: 100%;
    }

    /* Поиск свёрнут в иконку (#1097 S7): в ряду остаются «Фильтр» слева и иконка
       справа, а поле живёт в оверлее и занимает высоту ряда целиком. */
    .tables__search-overlay .field.search {
        width: 100%;
        height: 100%;
        min-width: 0;
        padding: 0 12px;
        margin: 0;
        gap: 8px;
    }

    .field.search .field__input {
        font-size: 14px;
    }

    /* Иконка-тоггл поиска: круг 36x36 (эталон §1.4 - круглые иконки-кнопки 50%).
       В Центре и кабинете этот круг 40px, но там он стоит в ряду заголовка; здесь
       он делит ряд с кнопкой «Фильтр» (36px), а весь мобильный ряд действий ниже
       после среза 6 тоже 36px. Разъехавшиеся по высоте контролы одного ряда -
       отдельная претензия пользователя (урок #1832), поэтому размер берём от соседа. */
    .search-icon-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 36px;
        height: 36px;
        margin-left: auto;
        padding: 0;
        border: 1px solid var(--border);
        border-radius: 50%;
        background: var(--surface);
        cursor: pointer;
        flex-shrink: 0;
        transition: background 0.15s ease, border-color 0.15s ease;
    }

    .search-icon-btn:hover,
    .search-icon-btn--active {
        background: var(--surface-2);
        border-color: var(--accent);
    }

    .search-icon-btn__img {
        width: 16px;
        height: 16px;
        color: var(--text);
        stroke-width: 2.1;
    }

    /* «⋯» - тот же круг 36px, что у поиска рядом: разъехавшиеся по высоте контролы
       одного ряда пользователь уже забраковал (урок #1832). */
    .more-icon-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 36px;
        height: 36px;
        padding: 0;
        border: 1px solid var(--border);
        border-radius: 50%;
        background: var(--surface);
        color: var(--text);
        font-size: 18px;
        line-height: 1;
        cursor: pointer;
        flex-shrink: 0;
        transition: background 0.15s ease, border-color 0.15s ease;
    }

    .more-icon-btn:hover {
        background: var(--surface-2);
        border-color: var(--accent);
    }

    /* Оверлей поиска поверх ряда. Правый вырез оставляет открытыми кнопки справа:
       36 + зазор 10 на каждую (значение приходит из шаблона, см. --search-overlay-right).
       Скругление под поле (15px): иначе белые квадратные углы оверлея торчат из-за
       pill-поля рядом с круглой иконкой (#1097 R3-3). */
    .tables__search-overlay {
        position: absolute;
        top: 0;
        bottom: 0;
        left: 0;
        right: var(--search-overlay-right, 46px);
        z-index: 1;
        display: flex;
        align-items: center;
        background: var(--surface);
        border-radius: var(--radius-md);
    }

    /* Крестик очистки внутри поля (появляется при вводе): сбрасывает и закрывает поиск. */
    .tables__search-clear {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 20px;
        height: 20px;
        padding: 0;
        border: none;
        background: transparent;
        color: var(--text-muted);
        font-size: 20px;
        line-height: 1;
        cursor: pointer;
        flex-shrink: 0;
    }

    .tables__search-clear:hover {
        color: var(--accent-text);
    }

    /* Раскрытие влево - clip-path (композитится, ряд не переставляется). */
    .table-search-enter-active,
    .table-search-leave-active {
        transition: clip-path 0.25s ease;
    }

    .table-search-enter-from,
    .table-search-leave-to {
        clip-path: inset(0 0 0 100%);
    }

    .table-search-enter-to,
    .table-search-leave-from {
        clip-path: inset(0 0 0 0);
    }

    .instruction-modal-large {
        margin: 10px;
        max-height: 90vh;
    }
}
</style>
