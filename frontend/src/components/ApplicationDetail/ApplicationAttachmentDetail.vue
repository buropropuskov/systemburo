<template>
  <div class="attachment-details">
    <div class="attachment-header-section">
      <div class="attachment-title-row">
        <h4>{{ attachment.attachment_display_name }}</h4>
        <div
          v-if="attachment.roof_access || attachment.free_parking"
          class="attachment-tags"
        >
          <Badge
            v-if="attachment.roof_access"
            variant="primary"
            size="lg"
          >
            Крыша
          </Badge>
          <Badge
            v-if="attachment.free_parking"
            variant="warning"
            size="lg"
          >
            Парковка
          </Badge>
        </div>
      </div>

      <!-- Даты действия -->
      <div
        v-if="attachment.entry_date_from || attachment.entry_date_to"
        class="date-range"
      >
        <span class="date-label">Срок действия:</span>
        <span class="date-value">
          {{ formatDateRange(attachment.entry_date_from, attachment.entry_date_to) }}
        </span>
      </div>

      <!-- Время действия -->
      <div
        v-if="attachment.entry_time_from || attachment.entry_time_to"
        class="time-range"
      >
        <span class="time-label">Время:</span>
        <span class="time-value">
          {{ formatTimeRange(attachment.entry_time_from, attachment.entry_time_to) }}
        </span>
      </div>
    </div>

    <div
      v-if="attachment.custom_values && attachment.custom_values.length"
      class="custom-values-section"
    >
      <div
        v-for="cv in attachment.custom_values"
        :key="cv.field_id"
        class="custom-value-row"
      >
        <span class="custom-value-label">{{ cv.label }}:</span>
        <span class="custom-value-text">{{ cv.value }}</span>
      </div>
    </div>

    <div
      ref="dataSection"
      class="attachment-data-section"
    >
      <div class="el-section">
        <div class="el-section__head">
          <h5>{{ headTitle }}</h5>
          <span
            v-if="!loading && rows.length"
            class="el-count"
            data-testid="attachment-elements-count"
          >{{ visibleRows.length }}</span>
          <SearchComponent
            v-if="!loading && rows.length"
            v-model="searchQuery"
            class="el-search"
            :title="searchPlaceholder"
            data-testid="attachment-elements-search"
            @search="searchVariants = $event"
          />
        </div>

        <div
          v-if="loading"
          class="loading-container"
        >
          <div class="loading-spinner" />
          <span class="loading-text">Загрузка...</span>
        </div>

        <div
          v-else-if="!rows.length"
          class="no-data"
        >
          {{ emptyText }}
        </div>

        <div
          v-else-if="!visibleRows.length"
          class="no-data"
          data-testid="attachment-elements-nothing-found"
        >
          Ничего не найдено по запросу «{{ searchQuery }}»
        </div>

        <div
          v-else
          class="el-table rt-table"
          data-testid="attachment-elements"
        >
          <div
            class="el-scroll"
            :class="{ 'el-scroll--scrollable': needsScroll }"
          >
            <div
              class="el-inner"
              :class="{ 'el-inner--compact': isCompact }"
              :style="{ minWidth: minInnerWidth + 'px' }"
            >
              <div class="el-head rt-head-row">
                <div class="c-num">
                  №
                </div>
                <div
                  v-for="col in columns"
                  :key="col.key"
                  class="el-cell"
                  :class="col.cls"
                  :style="cellStyle(col)"
                >
                  {{ col.label }}
                </div>
                <div
                  v-if="hasStateColumn"
                  class="c-state"
                />
              </div>

              <div
                v-for="(row, index) in visibleRows"
                :key="row.id"
                class="el-row rt-row"
                :class="[
                  supplementMarks[row.id] ? supplementMarks[row.id].rowClass : null,
                  {
                    'el-row--flagged': isFlagged(row),
                    'el-row--blacklisted': row.is_blacklisted,
                    'el-row--clickable': isClickable
                  }
                ]"
                data-testid="attachment-element-row"
                @click="openRow(row)"
              >
                <div class="c-num">
                  {{ index + 1 }}
                </div>

                <div
                  v-for="col in columns"
                  :key="col.key"
                  class="el-cell"
                  :class="[col.cls, { 'el-cell--key': col.type === 'key' }]"
                  :style="cellStyle(col)"
                  :data-label="col.label"
                >
                  <template v-if="col.type === 'chips'">
                    <div class="cell-chips">
                      <div class="chips">
                        <span
                          v-for="chip in visibleChips(row, col)"
                          :key="chip.key"
                          class="chip"
                          :class="{ 'chip--more': chip.isMore, 'chip--solo': chip.isSolo }"
                          :data-hint="chip.hint"
                          :title="chip.hint"
                          :data-testid="chip.isMore ? 'attachment-chip-more' : 'attachment-chip'"
                        ><span
                          class="chip__text"
                          :title="chip.hint"
                        >{{ chip.text }}</span></span>
                        <span
                          v-if="!chipItems(row, col).length && !canAssign"
                          class="chip chip--empty"
                        >—</span>
                      </div>
                      <button
                        v-if="canAssign && col.assignKind"
                        type="button"
                        class="chip chip--assign"
                        :class="{ 'chip--assign-empty': !chipItems(row, col).length }"
                        :data-hint="assignHint(row, col)"
                        data-testid="attachment-assign-open"
                        @click.stop="openAssign(row, col)"
                      >
                        {{ chipItems(row, col).length ? '+' : 'Добавить' }}
                      </button>
                    </div>
                  </template>

                  <template v-else-if="col.type === 'qty'">
                    <span class="qty">{{ row[col.field] }} шт</span>
                  </template>

                  <template v-else>
                    <span
                      class="val"
                      :data-hint="cellHint(row, col)"
                      :data-testid="col.type === 'key' ? 'attachment-element-key' : null"
                    >{{ col.value(row) }}</span>
                    <span
                      v-if="rowSub(row, col)"
                      class="val-sub"
                    >{{ rowSub(row, col) }}</span>
                  </template>

                  <!-- Метка дополнения (#1685) живёт в ключевой колонке: она есть у
                       всех трёх типов вложения, а колонка действий - только у машин
                       и сотрудников. Обёртка нужна карточке: она забирает строку
                       ячейки целиком, и метка встаёт под значением всегда в одном
                       месте, а не там, где её оставила длина наименования. -->
                  <div
                    v-if="col.type === 'key' && supplementMarks[row.id]"
                    class="supplement-line"
                  >
                    <Badge
                      class="supplement-badge"
                      :variant="supplementMarks[row.id].variant"
                      size="sm"
                      dot
                      :data-hint="supplementMarks[row.id].hint"
                      data-testid="attachment-supplement-badge"
                    >
                      {{ supplementMarks[row.id].text }}
                    </Badge>
                  </div>
                </div>

                <div
                  v-if="hasStateColumn"
                  class="c-state"
                >
                  <!-- Две кнопки в узкую ячейку не влезали: на помеченной строке
                       остаётся одна «Пропустить», а «Убрать» уходит в её меню. -->
                  <div
                    v-if="canOverride && isFlagged(row)"
                    class="row-actions"
                  >
                    <!-- Сдвоенная кнопка: слева действие, справа стрелка меню.
                         Разделены линией, чтобы было видно, что нажатия разные.
                         Без права убирать элементы меню состоит из одного дубля
                         "Принять" - тогда стрелки нет и кнопка остаётся цельной. -->
                    <div
                      class="split-btn"
                      :class="{ 'split-btn--single': !hasRowMenu }"
                    >
                      <button
                        type="button"
                        class="lk-button lk-button--danger split-btn__main"
                        data-testid="blacklist-override-btn"
                        @click.stop="chooseOverride(row)"
                      >
                        Принять
                      </button>
                      <button
                        v-if="hasRowMenu"
                        type="button"
                        class="lk-button lk-button--danger split-btn__toggle"
                        :class="{ 'split-btn__toggle--open': openRowMenu === row.id }"
                        data-testid="row-actions-toggle"
                        aria-label="Другие действия"
                        @click.stop="toggleRowMenu(row.id, $event)"
                      >
                        <svg
                          class="split-btn__caret"
                          width="10"
                          height="10"
                          viewBox="0 0 10 10"
                          fill="none"
                          aria-hidden="true"
                        >
                          <path
                            d="M2 3.5 5 6.5 8 3.5"
                            stroke="currentColor"
                            stroke-width="1.6"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          />
                        </svg>
                      </button>
                    </div>
                    <!-- Меню телепортируется в body: ячейка состояния узкая и с
                         overflow: hidden, внутри неё список обрезался целиком. -->
                    <Teleport to="body">
                      <transition name="row-menu">
                        <div
                          v-if="openRowMenu === row.id"
                          class="row-actions__menu"
                          :class="{ 'row-actions__menu--up': rowMenuOpenUp }"
                          :style="rowMenuStyle"
                          data-testid="row-actions-menu"
                          @click.stop
                        >
                          <button
                            type="button"
                            class="row-actions__item"
                            data-testid="row-action-override"
                            @click.stop="chooseOverride(row)"
                          >
                            Принять
                          </button>
                          <button
                            v-if="canRemove"
                            type="button"
                            class="row-actions__item row-actions__item--danger"
                            data-testid="row-action-remove"
                            @click.stop="chooseRemove(row)"
                          >
                            Убрать из заявки
                          </button>
                        </div>
                      </transition>
                    </Teleport>
                  </div>
                  <Badge
                    v-else-if="row.blacklist_similar"
                    class="blacklist-badge"
                    :variant="blacklistVariant(row.blacklist_similar)"
                    size="sm"
                    dot
                    :data-hint="blacklistTooltip(row.blacklist_similar)"
                  >
                    {{ blacklistLabel(row.blacklist_similar) }}
                  </Badge>
                  <button
                    v-if="canRemove && !(canOverride && isFlagged(row))"
                    type="button"
                    class="lk-button lk-button--ghost element-remove-btn"
                    data-testid="element-remove-btn"
                    data-hint="Убрать из заявки"
                    @click.stop="$emit('remove-element', { label: rowLabel(row), id: row.id })"
                  >
                    Убрать
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="el-foot">
            <span data-testid="attachment-elements-total">{{ totalText }}</span>

            <div class="el-foot__right">
              <Badge
                v-if="flaggedCount"
                variant="danger"
                size="sm"
                dot
                data-testid="attachment-flagged-summary"
              >
                {{ flaggedCount }} похоже на ЧС
              </Badge>

              <div
                v-if="canAssign && bulkColumns.length"
                class="el-foot__bulk"
              >
                <!-- На телефоне подписи «назначить всем места разгрузки / посты проезда»
                     не помещаются в подвал ни в каком виде - действия уезжают в лист,
                     где место под полную подпись есть. -->
                <button
                  v-if="isNarrowViewport"
                  type="button"
                  class="lk-button lk-button--ghost el-foot__bulk-btn"
                  data-testid="attachment-assign-all-open"
                  @click="bulkSheetOpen = true"
                >
                  Назначить всем…
                </button>

                <template v-else>
                  <span class="el-foot__bulk-label">Назначить всем:</span>
                  <button
                    v-for="col in bulkColumns"
                    :key="col.key"
                    type="button"
                    class="lk-button lk-button--ghost el-foot__bulk-btn"
                    :data-testid="`attachment-assign-all-${col.assignKind}`"
                    @click="openAssignAll(col)"
                  >
                    {{ col.label.toLowerCase() }}
                  </button>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Выбор, чему назначать всем: свёрнутый подвал мобилки раскрывается сюда.
         Слой 10004 - выше панели детали (10002) и карточек (10003), ниже самого
         окна назначения (10006), которое открывается поверх этого листа. -->
    <BaseModal
      :show="bulkSheetOpen"
      title="Назначить всем"
      width="480px"
      radius="30px"
      :z-index="10004"
      content-testid="attachment-assign-all-sheet"
      @close="bulkSheetOpen = false"
    >
      <div class="bulk-sheet">
        <button
          v-for="col in bulkColumns"
          :key="col.key"
          type="button"
          class="bulk-sheet__item"
          :data-testid="`attachment-assign-all-${col.assignKind}`"
          @click="chooseBulk(col)"
        >
          {{ col.bulkLabel || col.label }}
        </button>
      </div>
    </BaseModal>

    <!-- Без v-if: BaseModal анимирует по :show, а внешний v-if сносил бы
         компонент мгновенно и уход не проигрывался (см. заметку проекта). -->
    <ApplicationAssignModal
      :show="assign.open"
      :kind="assign.kind"
      :element-type="type === 'people' ? 'people' : 'cars'"
      :current-ids="assign.currentIds"
      :target-count="assign.elementIds.length"
      :submitting="assign.submitting"
      @close="closeAssign"
      @apply="applyAssign"
    />
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import ApplicationAssignModal from './ApplicationAssignModal.vue'
import SearchComponent from '@/components/SearchComponent.vue'
import { assignElementTables, assignCarUnloadPlaces } from '@/api/applicationAssignments'
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'
import { matchesSearchFuzzy } from '@/utils/searchVariants'
import { formatNumberForDisplay } from '@/composables/useNumberFormat'
import { getViewportZoom } from '@/utils/viewportScale'
import {
    SUPPLEMENT_ACCEPTED,
    SUPPLEMENT_APPROVED,
    SUPPLEMENT_PENDING,
    SUPPLEMENT_CLOSED_STATUSES,
} from '@/utils/supplementStatuses'

/** Ширины служебных частей строки: порядковый номер, колонка действий, отступы. */
const NUM_COLUMN_WIDTH = 22;
const STATE_COLUMN_WIDTH = 112;
const CELL_GAP = 10;
const ROW_PADDING = 24;

/** Сколько названий показываем до сворачивания в «+N». */
const CHIPS_VISIBLE_COMPACT = 1;
const CHIPS_VISIBLE_WIDE = 2;

/** Отступы и рамка чипа поверх текста, промежуток между чипами. */
const CHIP_SIDE_SPACE = 24;
const CHIP_GAP = 6;
/** Ширина символа, когда замер шрифтом недоступен (тесты в jsdom). */
const CHIP_CHAR_WIDTH = 7;
/** Шрифты ячеек - ими же меряем ширину текста, иначе расчёт врёт. */
const FONTS = {
    key: '600 14.5px Montserrat, sans-serif',
    text: '13.5px Montserrat, sans-serif',
    chip: '12.5px Montserrat, sans-serif'
};
/** Кнопка назначения с промежутком: столько ширины ячейки чипам недоступно. */
const ASSIGN_BUTTON_SPACE = 34;

/** Запас, чтобы значение не упиралось вплотную в край колонки. */
const TEXT_SIDE_SPACE = 6;

/* Меню действий строки: отступ от кнопки, поле до края окна и габариты, по которым
   решается, помещается ли меню снизу. Ширина совпадает с min-width в стилях. */
const ROW_MENU_GAP = 4;
const ROW_MENU_MARGIN = 8;
const ROW_MENU_WIDTH = 170;
const ROW_MENU_HEIGHT = 96;

// Строка состава несёт статус принёсшего её раунда в supplement_status;
// supplement_id === null - строка пришла с исходной подачей (#1685).

/**
 * Как выглядит строка в зависимости от судьбы принёсшего её раунда.
 *
 * Принятая добавка строку не красит: она уже работает наравне с исходным составом, и
 * подсветка «нового» висела бы на ней вечно. Отклонённая, наоборот, остаётся в составе
 * навсегда, поэтому её нужно отличать от рабочей - иначе автор будет ждать пропуска,
 * которого не будет.
 *
 * @param {Object} row строка состава вложения
 * @returns {{ state: string, variant: string, text: string, hint: string, rowClass: ?string }|null}
 */
function supplementRowMark(row) {
    if (!row || row.supplement_id === null || row.supplement_id === undefined) return null;

    const status = row.supplement_status || null;
    const title = row.supplement_number ? `Дополнение №${row.supplement_number}` : 'Дополнение';
    const shortTitle = row.supplement_number ? `Доп. №${row.supplement_number}` : 'Доп.';

    if (SUPPLEMENT_CLOSED_STATUSES.includes(status)) {
        return {
            state: 'closed',
            variant: 'neutral',
            text: 'Отклонено',
            hint: `${title} не состоялось: строка на проходную не попадёт.`,
            rowClass: 'el-row--supplement-closed'
        };
    }

    if (row.is_pending && status === SUPPLEMENT_PENDING) {
        return {
            state: 'pending',
            variant: 'warning',
            text: 'На согласовании',
            hint: `${title} ждёт голосов согласующих. На проходную строка пока не допущена.`,
            rowClass: 'el-row--supplement-pending'
        };
    }

    if (row.is_pending && status === SUPPLEMENT_APPROVED) {
        return {
            state: 'approved',
            variant: 'info',
            text: 'Ждёт принятия',
            hint: `${title} согласовано, ждёт решения принимающего. На проходную строка пока не допущена.`,
            rowClass: 'el-row--supplement-approved'
        };
    }

    // accepted - добавка принята; merged - влита в основной круг заявки. И там и там
    // строка живёт по общим правилам, остаётся только пометка происхождения.
    return {
        state: status === SUPPLEMENT_ACCEPTED ? SUPPLEMENT_ACCEPTED : 'origin',
        variant: 'primary',
        text: shortTitle,
        hint: `Строка добавлена дополнением к поданной заявке (${title}).`,
        rowClass: null
    };
}

export default {
    name: 'ApplicationAttachmentDetail',
    components: { ApplicationAssignModal, BaseModal, Badge, SearchComponent },
    props: {
        attachment: {
            type: Object,
            required: true
        },
        cars: {
            type: Array,
            default: () => []
        },
        employees: {
            type: Array,
            default: () => []
        },
        items: {
            type: Array,
            default: () => []
        },
        loading: {
            type: Boolean,
            default: false
        },
        // Показываем "Пропустить" ответственному и принимающему - право на подтверждение
        // пропуска у них общее.
        canOverride: {
            type: Boolean,
            default: false
        },
        // "Убрать" - только принимающему: он единственный, кто правит состав поданной заявки.
        canRemove: {
            type: Boolean,
            default: false
        },
        // "Доступные мне" показывают тот же список, но карточку элемента там не
        // открывают - строка не должна выглядеть кликабельной.
        interactive: {
            type: Boolean,
            default: true
        },
        // Принимающий может доназначить посты и места, пока заявка не закрыта (#1393).
        canAssign: {
            type: Boolean,
            default: false
        },
        applicationId: {
            type: Number,
            default: null
        }
    },
    emits: ['open-vehicle', 'open-employee', 'override-element', 'remove-element', 'assignments-changed'],
    data() {
        return {
            // Идентификатор строки, у которой открыто меню действий: одно на таблицу,
            // чтобы два меню не висели одновременно.
            openRowMenu: null,
            rowMenuStyle: null,
            // Меню раскрылось вверх - от этого зависит точка роста анимации.
            rowMenuOpenUp: false,
            // Кнопка, от которой считается место: меню лежит в body и при прокрутке
            // списка должно ехать за своей строкой, а не оставаться висеть на месте.
            rowMenuAnchor: null,
            containerWidth: 0,
            isNarrowViewport: false,
            resizeObserver: null,
            viewportQuery: null,
            // Форматы гос. номеров - только для колонки "Гос. номер" (машины). Грузим
            // один раз при монтировании, не на каждую строку: справочник общий для
            // всей ленты, matchNumberToFormat раскладывает каждую строку в JS.
            licensePlateFormats: [],
            // Ширина колонок с чипами: по ней считаем, сколько названий влезает
            // целиком. Пусто до первого замера - тогда работает запасной расчёт.
            chipColumnWidths: {},
            textMeasureContext: null,
            searchQuery: '',
            searchVariants: [],
            bulkSheetOpen: false,
            assign: {
                open: false,
                kind: 'tables',
                elementIds: [],
                currentIds: [],
                submitting: false
            }
        };
    },
    computed: {
        type() {
            return this.attachment.attachment_type;
        },

        /**
         * Есть ли в меню строки хоть один пункт, которого нет в основной кнопке.
         * Сейчас такой один - "Убрать из заявки", и без права на него меню сводилось
         * бы к дублю "Принять": стрелка вела бы в никуда.
         */
        hasRowMenu() {
            return this.canRemove;
        },

        rows() {
            if (this.type === 'cars') return this.cars;
            if (this.type === 'people') return this.employees;
            if (this.type === 'items') return this.items;
            return [];
        },

        sectionTitle() {
            if (this.type === 'cars') return 'Автомобили';
            if (this.type === 'people') return 'Сотрудники';
            return 'Товарно-материальные ценности';
        },

        /**
         * Заголовок блока делит строку с поиском, поэтому на телефоне длинное
         * название сокращается: «Товарно-материальные ценности» съедало бы всю
         * строку и оставляло полю поиска несколько пикселей.
         */
        headTitle() {
            if (this.isNarrowViewport && this.type === 'items') return 'ТМЦ';
            return this.sectionTitle;
        },

        emptyText() {
            if (this.type === 'cars') return 'Нет данных об автомобилях';
            if (this.type === 'people') return 'Нет данных о сотрудниках';
            return 'Нет данных о ТМЦ';
        },

        /**
         * Полный набор колонок по типу вложения. Колонка с `wideOnly` живёт
         * только когда ширины хватает: в узком контейнере её значение уходит
         * второй строкой под ключевое поле (`sub`), потому что резать
         * многоточием гос. номер или ФИО нельзя.
         */
        allColumns() {
            if (this.type === 'cars') {
                const brand = row => row.car_brand || '';
                return [
                    {
                        key: 'number',
                        type: 'key',
                        cls: 'c-key',
                        label: 'Гос. номер',
                        compactLabel: 'Гос. номер и марка',
                        // min/minCompact расширены под бейдж статуса дополнения ("На
                        // согласовании" и длиннее): при старых 114/140 фон бейджа не
                        // покрывал строки текста, и пилюля дополнения ложилась в три
                        // строки (владелец: "жирный шарик" из-за узкой колонки).
                        grow: 26, min: 160,
                        growCompact: 40, minCompact: 190,
                        // Номера машин, заведённых импортом бланка, хранятся слитно -
                        // раскладываем по формату для показа (#1392 разбор карточки).
                        value: row => formatNumberForDisplay(row.car_number, this.licensePlateFormats),
                        sub: brand
                    },
                    {
                        key: 'brand',
                        type: 'text',
                        cls: 'c-sub',
                        label: 'Марка',
                        grow: 18, min: 96,
                        wideOnly: true,
                        value: brand
                    },
                    {
                        key: 'places',
                        type: 'chips',
                        cls: 'c-places',
                        label: 'Места разгрузки',
                        assignKind: 'places',
                        grow: 30, min: 128,
                        growCompact: 36, minCompact: 124,
                        field: 'unload_places',
                        nameKey: 'name',
                        unit: ['место', 'места', 'мест']
                    },
                    {
                        key: 'tables',
                        type: 'chips',
                        cls: 'c-tables',
                        label: 'Проезд',
                        // В листе места хватает на полную подпись, в колонке - нет.
                        bulkLabel: 'Посты проезда',
                        assignKind: 'tables',
                        grow: 26, min: 92,
                        growCompact: 26, minCompact: 96,
                        field: 'target_tables',
                        nameKey: 'display_name',
                        unit: ['пост', 'поста', 'постов']
                    }
                ];
            }

            if (this.type === 'people') {
                const position = row => row.position || '';
                return [
                    {
                        key: 'name',
                        type: 'key',
                        cls: 'c-key',
                        label: 'ФИО',
                        compactLabel: 'ФИО и должность',
                        grow: 34, min: 248,
                        growCompact: 44, minCompact: 248,
                        value: row => this.employeeFullName(row),
                        sub: position
                    },
                    {
                        key: 'position',
                        type: 'text',
                        cls: 'c-sub',
                        label: 'Должность',
                        grow: 24, min: 132,
                        wideOnly: true,
                        value: position
                    },
                    {
                        key: 'tables',
                        type: 'chips',
                        cls: 'c-places',
                        label: 'Места прохода',
                        assignKind: 'tables',
                        grow: 28, min: 124,
                        growCompact: 30, minCompact: 124,
                        field: 'target_tables',
                        nameKey: 'display_name',
                        unit: ['пост', 'поста', 'постов']
                    }
                ];
            }

            return [
                {
                    key: 'name',
                    type: 'key',
                    cls: 'c-places',
                    label: 'Наименование',
                    grow: 1, min: 180,
                    value: row => row.name
                },
                {
                    key: 'count',
                    type: 'qty',
                    cls: 'c-state c-state--qty',
                    label: 'Количество',
                    fixed: 120,
                    field: 'count'
                }
            ];
        },

        /** Набор колонок под текущую ширину блока. */
        columns() {
            if (!this.isCompact) return this.allColumns;
            return this.allColumns
                .filter(col => !col.wideOnly)
                .map(col => (col.compactLabel ? { ...col, label: col.compactLabel } : col));
        },

        /**
         * Колонки, по которым можно назначить всем сразу. Берём из полного набора,
         * а не из видимых: в узком режиме часть колонок схлопнута, но назначать
         * по ним всё равно нужно.
         */
        bulkColumns() {
            return this.allColumns.filter(col => col.assignKind);
        },

        /** ТМЦ не несут флага ЧС и не открывают карточку - колонка действий им не нужна. */
        hasStateColumn() {
            return this.type !== 'items';
        },

        isClickable() {
            return this.interactive && this.hasStateColumn;
        },

        /**
         * Узкий контейнер: колонка второго плана (марка, должность) уходит из
         * своей колонки и встаёт рядом с ключевым значением. На телефоне режим
         * тоже включён: строка разворачивается в карточку, где отдельная строка
         * под марку - лишняя, а подписи полей всё равно скрыты.
         */
        isCompact() {
            if (!this.containerWidth) return false;
            return this.containerWidth < this.wideLayoutWidth;
        },

        /** Ширина, начиная с которой помещаются все колонки. */
        wideLayoutWidth() {
            return this.layoutWidth(this.allColumns);
        },

        minInnerWidth() {
            return this.layoutWidth(this.columns);
        },

        /**
         * Прокрутка включается, только если колонки не влезают: у скроллящегося
         * контейнера подсказки чипов обрезаются краем и дают лишнюю полосу.
         */
        needsScroll() {
            return !!this.containerWidth && this.containerWidth < this.minInnerWidth;
        },

        /**
         * Поиск идёт по всем видимым полям строки, включая названия мест и
         * постов: пользователь ищет и по номеру, и по месту разгрузки.
         * Опечатка не мешает: «942» находит «У 952 ЕУ 935».
         */
        visibleRows() {
            if (!this.searchVariants.length) return this.rows;
            return this.rows.filter(row => matchesSearchFuzzy(this.searchText(row), this.searchVariants));
        },

        searchPlaceholder() {
            return 'Поиск..';
        },

        isFiltered() {
            return this.searchVariants.length > 0 && this.visibleRows.length !== this.rows.length;
        },

        totalText() {
            if (this.type === 'items') {
                const units = this.visibleRows.reduce((sum, item) => sum + (Number(item.count) || 0), 0);
                const base = `Всего позиций: ${this.visibleRows.length}, единиц: ${units}`;
                return this.isFiltered ? `${base} (найдено из ${this.rows.length})` : base;
            }
            return this.isFiltered
                ? `Найдено: ${this.visibleRows.length} из ${this.rows.length}`
                : `Всего: ${this.rows.length}`;
        },

        flaggedCount() {
            return this.visibleRows.filter(row => this.isFlagged(row)).length;
        },

        /** Метки дополнения по id строки: считаем один раз, а не по три вызова на ячейку. */
        supplementMarks() {
            const marks = {};
            for (const row of this.rows) {
                const mark = supplementRowMark(row);
                if (mark) marks[row.id] = mark;
            }
            return marks;
        }
    },
    watch: {
        containerWidth() {
            this.$nextTick(this.measureChipColumns);
        },
        isCompact() {
            this.$nextTick(this.measureChipColumns);
        },
        rows() {
            this.$nextTick(this.measureChipColumns);
        },
        visibleRows() {
            this.$nextTick(this.measureChipColumns);
        }
    },
    mounted() {
        // Меню действий строки закрывается кликом мимо него - иначе висит открытым,
        // пока не нажмут саму кнопку.
        document.addEventListener('click', this.closeRowMenuOnOutside);

        if (typeof window.matchMedia === 'function') {
            this.viewportQuery = window.matchMedia('(max-width: 767.98px)');
            this.isNarrowViewport = this.viewportQuery.matches;
            this.viewportQuery.addEventListener('change', this.onViewportChange);
        }

        // Только для машин - колонка "Гос. номер" единственная, что использует
        // форматы. У сотрудников/ТМЦ запрос не нужен вовсе.
        if (this.type === 'cars') this.loadLicensePlateFormats();

        // Режим считаем по ширине блока, а не окна: в детали заявки колонка
        // узкая даже на широком экране, в "Доступных мне" - наоборот.
        if (typeof ResizeObserver !== 'undefined' && this.$refs.dataSection) {
            this.resizeObserver = new ResizeObserver(entries => {
                for (const entry of entries) {
                    this.containerWidth = entry.contentRect.width;
                }
            });
            this.resizeObserver.observe(this.$refs.dataSection);
            this.containerWidth = this.$refs.dataSection.clientWidth;
        }

        this.initTextMeasure();
        this.$nextTick(this.measureChipColumns);
    },
    beforeUnmount() {
        document.removeEventListener('click', this.closeRowMenuOnOutside);
        this.closeRowMenu();
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
            this.resizeObserver = null;
        }
        if (this.viewportQuery) {
            this.viewportQuery.removeEventListener('change', this.onViewportChange);
            this.viewportQuery = null;
        }
    },
    methods: {
        /** Canvas меряет ширину текста точнее прикидки по числу символов. */
        initTextMeasure() {
            if (typeof document === 'undefined') return;
            const canvas = document.createElement('canvas');
            const ctx = typeof canvas.getContext === 'function' ? canvas.getContext('2d') : null;
            if (!ctx) return;
            this.textMeasureContext = ctx;
        },

        onViewportChange(event) {
            this.isNarrowViewport = event.matches;
        },

        /** Тот же эндпоинт и разбор ответа, что у CarsTable/VehicleForm. */
        async loadLicensePlateFormats() {
            try {
                const response = await apiRequest('/license-plate-formats', {});
                if (response.ok) this.licensePlateFormats = await response.json();
            } catch (error) {
                console.error('Ошибка при загрузке форматов номеров:', error);
            }
        },

        /**
         * Значение колонки второго плана рядом с ключевым: марка у машины,
         * должность у сотрудника.
         *
         * В таблице прочерк обязателен - без него строка без марки оказывалась
         * ниже соседних и лента "дышала". В карточке подстрока стоит той же
         * строкой, что и номер, высоту не меняет, и пустой прочерк там только
         * мусор.
         *
         * @returns {?string} текст подстроки или null, если её не рисуем
         */
        rowSub(row, col) {
            if (!col.sub || !this.isCompact) return null;
            return col.sub(row) || (this.isNarrowViewport ? null : '—');
        },

        /** Минимальная ширина колонки в текущем режиме. */
        columnMin(col) {
            if (col.fixed) return col.fixed;
            return (this.isCompact && col.minCompact) || col.min || 0;
        },

        /** Доли и минимум задаём из описания колонок, чтобы CSS их не дублировал. */
        cellStyle(col) {
            if (col.fixed) return { width: col.fixed + 'px', flexShrink: 0 };
            const grow = (this.isCompact && col.growCompact) || col.grow || 1;
            return { flex: `${grow} 1 0`, minWidth: this.columnMin(col) + 'px' };
        },

        /** Сумма минимальных ширин колонок со служебными частями строки. */
        layoutWidth(columns) {
            const cells = columns.reduce((sum, col) => sum + this.columnMin(col), 0);
            const state = this.hasStateColumn ? STATE_COLUMN_WIDTH : 0;
            const gaps = CELL_GAP * columns.length;
            return NUM_COLUMN_WIDTH + cells + state + gaps + ROW_PADDING;
        },

        /** Склейка всех показываемых полей строки - по ней и ищем. */
        searchText(row) {
            const parts = [];
            for (const col of this.allColumns) {
                if (col.type === 'chips') {
                    parts.push(...this.chipItems(row, col).map(item => this.chipName(item, col)));
                } else if (col.type === 'qty') {
                    parts.push(row[col.field]);
                } else if (typeof col.value === 'function') {
                    parts.push(col.value(row));
                }
            }
            return parts.filter(Boolean).join(' ');
        },

        chipItems(row, col) {
            const list = row[col.field];
            return Array.isArray(list) ? list : [];
        },

        chipName(item, col) {
            return item[col.nameKey] || item.name || '';
        },

        /** Ширина текста нужным шрифтом; без canvas - прикидка по числу символов. */
        textWidth(text, font = FONTS.chip) {
            const ctx = this.textMeasureContext;
            if (!ctx) return Math.ceil(text.length * CHIP_CHAR_WIDTH);
            ctx.font = font;
            return Math.ceil(ctx.measureText(text).width);
        },

        /** Ширина чипа: текст плюс отступы и рамка. */
        chipWidth(text) {
            return this.textWidth(text, FONTS.chip) + CHIP_SIDE_SPACE;
        },

        /**
         * Сколько названий помещается в колонку целиком. Пока ширина не
         * измерена (первый кадр, jsdom), берём запасное количество по режиму.
         */
        fittingChipCount(names, col) {
            const measured = this.chipColumnWidths[col.key];
            if (!measured) {
                return this.isCompact ? CHIPS_VISIBLE_COMPACT : CHIPS_VISIBLE_WIDE;
            }
            // Кнопка назначения стоит в той же ячейке - её ширину чипам не отдаём.
            const available = this.canAssign && col.assignKind
                ? measured - ASSIGN_BUTTON_SPACE
                : measured;
            if (available <= 0) return 0;

            const moreWidth = this.chipWidth(`+${names.length}`);
            let used = 0;
            let fits = 0;

            for (let i = 0; i < names.length; i++) {
                const width = this.chipWidth(names[i]) + (fits ? CHIP_GAP : 0);
                const restHidden = i < names.length - 1;
                const budget = available - (restHidden ? moreWidth + CHIP_GAP : 0);
                if (used + width > budget) break;
                used += width;
                fits++;
            }
            return fits;
        },

        /**
         * Видимые чипы: сколько помещается плюс «+N», полный список - в подсказке.
         * Если в колонку не влезает даже одно название, показываем счётчик
         * («2 поста») - обрезанного на середине названия быть не должно.
         * Единственное длинное название оставляем чипом с подсказкой: счётчик
         * «1 место» не сказал бы ничего.
         */
        visibleChips(row, col) {
            const items = this.chipItems(row, col);
            if (!items.length) return [];

            const names = items.map(item => this.chipName(item, col));
            const hint = names.join(', ');
            const fits = this.fittingChipCount(names, col);

            if (!fits && names.length > 1) {
                return [{
                    key: 'summary',
                    text: `${names.length} ${this.plural(names.length, col.unit)}`,
                    hint,
                    isMore: true
                }];
            }

            const limit = Math.max(fits, 1);
            const shown = names.slice(0, limit).map((text, index) => ({
                key: `chip-${index}`,
                text,
                hint: names.length > limit ? null : hint,
                isMore: false,
                // Единственное название сжимается по ячейке и обрезается многоточием -
                // полное видно в подсказке. Соседи по колонке так не жмутся: там вместо
                // обрезки показывается счётчик.
                isSolo: names.length === 1
            }));

            if (names.length > limit) {
                shown.push({
                    key: 'more',
                    text: `+${names.length - limit}`,
                    hint,
                    isMore: true
                });
            }
            return shown;
        },

        /**
         * Полное значение в подсказке - только если текст не помещается в
         * колонку. Иначе пузырёк выскакивал бы на каждой строке без нужды.
         */
        cellHint(row, col) {
            if (col.type === 'chips' || col.type === 'qty') return null;
            const width = this.chipColumnWidths[col.key];
            if (!width) return null;
            const text = col.value(row) || '';
            const font = col.type === 'key' ? FONTS.key : FONTS.text;
            return this.textWidth(text, font) + TEXT_SIDE_SPACE > width ? text : null;
        },

        /**
         * Ширины колонок берём с первой строки: доли у всех строк одинаковые,
         * поэтому одного замера на колонку достаточно.
         */
        measureChipColumns() {
            if (!this.$el || typeof this.$el.querySelector !== 'function') return;
            const widths = {};
            for (const col of this.columns) {
                if (col.type === 'qty') continue;
                const cell = this.$el.querySelector(`.el-row .${col.cls}`);
                if (cell && cell.clientWidth) widths[col.key] = cell.clientWidth;
            }

            const current = this.chipColumnWidths;
            const keys = Object.keys(widths);
            const same = keys.length === Object.keys(current).length
                && keys.every(key => current[key] === widths[key]);
            if (!same) this.chipColumnWidths = widths;
        },

        plural(count, forms) {
            const [one, few, many] = forms;
            const mod100 = count % 100;
            if (mod100 >= 11 && mod100 <= 14) return many;
            const mod10 = count % 10;
            if (mod10 === 1) return one;
            if (mod10 >= 2 && mod10 <= 4) return few;
            return many;
        },

        /** Подсказка кнопки: пустой набор объясняем иначе, чем добавление к существующему. */
        assignHint(row, col) {
            const what = col.label.toLowerCase();
            return this.chipItems(row, col).length
                ? `Изменить ${what}`
                : `Назначить ${what}`;
        },

        /** Открывает выбор для одной строки: текущий набор показывается отмеченным. */
        openAssign(row, col) {
            this.assign = {
                open: true,
                kind: col.assignKind,
                elementIds: [row.id],
                currentIds: this.chipItems(row, col).map(item => item.id),
                submitting: false
            };
        },

        /**
         * Массовое назначение: отмеченным показываем то, что есть у всех строк
         * сразу - остальное принимающий добавит сам. Применяется к найденным
         * строкам, поэтому поиск заодно работает как фильтр «кому назначить».
         */
        openAssignAll(col) {
            const rows = this.visibleRows;
            const perRow = rows.map(row => this.chipItems(row, col).map(item => item.id));
            const common = perRow.length
                ? perRow.reduce((acc, ids) => acc.filter(id => ids.includes(id)))
                : [];

            this.assign = {
                open: true,
                kind: col.assignKind,
                elementIds: rows.map(row => row.id),
                currentIds: common,
                submitting: false
            };
        },

        /**
         * Выбор из листа: лист закрывается сразу, окно назначения открывается
         * поверх него (10006 против 10004), поэтому уходящий лист ничего не
         * перекрывает и ждать конца его анимации не нужно.
         */
        chooseBulk(col) {
            this.bulkSheetOpen = false;
            this.openAssignAll(col);
        },

        closeAssign() {
            this.assign.open = false;
        },

        /**
         * Сохраняет выбор режимом replace: окно показывало полный набор, значит
         * итог и есть желаемое состояние - так одно действие и добавляет, и снимает.
         */
        async applyAssign(selectedIds) {
            if (!this.applicationId) return;
            const notify = useDeletionsStore().notify;
            this.assign.submitting = true;
            try {
                if (this.assign.kind === 'places') {
                    await assignCarUnloadPlaces(this.applicationId, {
                        carIds: this.assign.elementIds,
                        placeIds: selectedIds,
                        mode: 'replace'
                    });
                } else {
                    await assignElementTables(this.applicationId, {
                        elementType: this.type === 'people' ? 'people' : 'cars',
                        elementIds: this.assign.elementIds,
                        tableIds: selectedIds,
                        mode: 'replace'
                    });
                }
                notify({
                    type: 'success',
                    message: this.assign.kind === 'places' ? 'Места разгрузки обновлены' : 'Посты обновлены'
                });
                this.assign.open = false;
                this.$emit('assignments-changed');
            } catch (error) {
                notify({ type: 'error', message: error.message || 'Не удалось сохранить' });
            } finally {
                this.assign.submitting = false;
            }
        },

        openRow(row) {
            if (!this.isClickable) return;
            if (this.type === 'cars') this.$emit('open-vehicle', row);
            if (this.type === 'people') this.$emit('open-employee', row);
        },

        rowLabel(row) {
            return this.type === 'cars' ? row.car_number : this.employeeFullName(row);
        },

        isFlagged(entity) {
            const flag = entity && entity.blacklist_similar;
            return !!flag && !flag.overridden;
        },

        employeeFullName(employee) {
            return [employee.last_name, employee.first_name, employee.middle_name].filter(Boolean).join(' ');
        },

        closeRowMenuOnOutside(event) {
            if (this.openRowMenu === null) return;
            if (event.target && event.target.closest && event.target.closest('.row-actions')) return;
            this.closeRowMenu();
        },

        toggleRowMenu(rowID, event) {
            if (this.openRowMenu === rowID) {
                this.closeRowMenu();
                return;
            }
            this.rowMenuAnchor = event.currentTarget;
            this.openRowMenu = rowID;
            this.updateRowMenuPosition();
            window.addEventListener('scroll', this.updateRowMenuPosition, true);
            window.addEventListener('resize', this.updateRowMenuPosition);
        },

        closeRowMenu() {
            this.openRowMenu = null;
            this.rowMenuAnchor = null;
            window.removeEventListener('scroll', this.updateRowMenuPosition, true);
            window.removeEventListener('resize', this.updateRowMenuPosition);
        },

        /**
         * Место меню считается от кнопки: оно лежит в body, а не в строке.
         *
         * Все величины приводятся к layout-px делением на масштаб страницы - rect
         * отдаёт device-px, а innerWidth/innerHeight незумленные, и без общего
         * знаменателя меню уезжает от кнопки тем дальше, чем правее строка (тот же
         * расчёт, что у BaseDropdown). Раньше делился только rect, окно - нет.
         *
         * Снизу меню помещается не всегда: помеченная строка часто последняя в
         * списке, и вниз оно уходило под край карточки. Не хватает места - открываем
         * вверх. По горизонтали держим меню целиком в окне: считаем от правого края
         * кнопки, но не даём левому краю уйти за поле.
         */
        updateRowMenuPosition() {
            const anchor = this.rowMenuAnchor;
            if (!anchor) {
                this.closeRowMenu();
                return;
            }
            const zoom = getViewportZoom();
            const rect = anchor.getBoundingClientRect();
            const top = rect.top / zoom;
            const bottom = rect.bottom / zoom;
            const right = rect.right / zoom;
            const vw = window.innerWidth / zoom;
            const vh = window.innerHeight / zoom;
            const spaceBelow = vh - bottom - ROW_MENU_GAP - ROW_MENU_MARGIN;
            const spaceAbove = top - ROW_MENU_GAP - ROW_MENU_MARGIN;
            const openUp = spaceBelow < ROW_MENU_HEIGHT && spaceAbove > spaceBelow;
            const offsetRight = Math.min(
                Math.max(ROW_MENU_MARGIN, vw - right),
                Math.max(ROW_MENU_MARGIN, vw - ROW_MENU_WIDTH - ROW_MENU_MARGIN)
            );
            this.rowMenuOpenUp = openUp;
            this.rowMenuStyle = {
                position: 'fixed',
                right: `${Math.round(offsetRight)}px`,
                ...(openUp
                    ? { bottom: `${Math.round(vh - top + ROW_MENU_GAP)}px`, top: 'auto' }
                    : { top: `${Math.round(bottom + ROW_MENU_GAP)}px`, bottom: 'auto' })
            };
        },

        chooseOverride(row) {
            this.closeRowMenu();
            this.$emit('override-element', { label: this.rowLabel(row), flag: row.blacklist_similar });
        },

        chooseRemove(row) {
            this.closeRowMenu();
            this.$emit('remove-element', { label: this.rowLabel(row), id: row.id });
        },

        blacklistVariant(flag) {
            return flag.overridden ? 'neutral' : 'danger';
        },

        /**
         * Подпись короткая в любом режиме: "пропуск подтверждён" не помещался
         * в колонку действий и обрезался многоточием. Полный текст - в подсказке.
         */
        blacklistLabel(flag) {
            return flag.overridden ? 'ЧС снят' : 'похоже на ЧС';
        },

        blacklistTooltip(flag) {
            const value = flag.matched_value || '';
            const base = value
                ? `Возможный обход чёрного списка. Похоже на: ${value}`
                : 'Возможный обход чёрного списка.';
            const full = flag.matched_reason ? `${base} (${flag.matched_reason})` : base;
            return flag.overridden ? `Пропуск подтверждён. ${full}` : full;
        },

        formatDate(date) {
            if (!date) return '';
            if (typeof date === 'string') {
                date = new Date(date);
            }
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },

        formatDateRange(dateFrom, dateTo) {
            if (!dateFrom && !dateTo) return '';
            const from = dateFrom ? this.formatDate(dateFrom) : '';
            const to = dateTo ? this.formatDate(dateTo) : '';
            if (from && to) {
                const fromDate = new Date(dateFrom);
                const toDate = new Date(dateTo);
                if (fromDate.toDateString() === toDate.toDateString()) {
                    return from;
                }
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `по ${to}`;
            }
            return '';
        },

        formatTime(time) {
            if (!time) return '';
            const timeParts = time.split(':');
            if (timeParts.length >= 2) {
                return `${timeParts[0]}:${timeParts[1]}`;
            }
            return time;
        },

        formatTimeRange(timeFrom, timeTo) {
            if (!timeFrom && !timeTo) return '';
            const from = timeFrom ? this.formatTime(timeFrom) : '';
            const to = timeTo ? this.formatTime(timeTo) : '';
            if (from && to) {
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `до ${to}`;
            }
            return '';
        }
    }
}
</script>

<style scoped>
.attachment-details {
    background: var(--surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: 0 2px 12px var(--shadow-drop);
}

.attachment-header-section {
    padding: 15px;
    border-bottom: 1px solid var(--color-border);
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px 24px;
}

.attachment-details h4 {
    font-size: 18px;
    color: var(--accent-text);
    font-weight: 700;
    margin: 0;
    grid-column: 1 / -1;
}

.date-range, .time-range {
    display: flex;
    flex-direction: column;
    gap: 0px;
    font-size: 14px;
}

.attachment-title-row {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
}

.attachment-tags {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    flex-shrink: 0;
}

.date-range:last-child, .time-range:last-child {
    margin-bottom: 0;
}

.date-label, .time-label {
    color: var(--text-muted);
    font-weight: 400;
    min-width: 110px;
    font-size: 14px;
}

.date-value, .time-value {
    color: var(--text);
    font-weight: 400;
    font-size: 15px;
}

.custom-values-section {
    padding: 15px;
    border-bottom: 1px solid var(--color-border);
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px 24px;
}

.custom-value-row {
    display: flex;
    flex-direction: column;
    font-size: 14px;
}

.custom-value-label {
    color: var(--text-muted);
    font-weight: 400;
}

.custom-value-text {
    color: var(--text);
    font-weight: 400;
    font-size: 15px;
}

.attachment-data-section {
    padding: 15px;
    min-height: 300px;
}

.el-section {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.el-section__head {
    display: flex;
    align-items: center;
    gap: 10px;
}

.el-search {
    margin-left: auto;
    flex-shrink: 0;
    /* Ниже собственных 35px компонента: рядом с заголовком списка так легче. */
    height: 30px;
}

.el-section__head h5 {
    margin: 0;
    font-size: 16px;
    font-weight: 700;
    color: var(--color-text);
}

.el-count {
    padding: 2px 10px;
    border-radius: var(--radius-pill);
    background: var(--color-primary-tint);
    color: var(--accent-text);
    font-size: 12px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
}

.el-table {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--surface);
}

/* Ниже суммы минимальных ширин колонок лента прокручивается по горизонтали,
   а не схлопывает колонку в ноль. В обычном состоянии переполнения нет:
   overflow не ставим, иначе он обрезал бы подсказки чипов. */
.el-scroll--scrollable {
    overflow-x: auto;
}

.el-head,
.el-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 12px;
}

.el-head {
    height: 38px;
    border-radius: var(--radius-md) var(--radius-md) 0 0;
    background: var(--accent-tint);
    border-bottom: 1px solid var(--color-border);
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
}

.el-row {
    min-height: 46px;
    padding-top: 8px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border);
    transition: background 0.15s ease;
    animation: slideIn 0.3s ease-out forwards;
    opacity: 0;
    transform: translateY(10px);
}

.el-row:last-child {
    border-bottom: none;
}

.el-row--clickable {
    cursor: pointer;
}

.el-row--clickable:hover {
    background: var(--color-bg);
}

.el-row--flagged {
    background: var(--danger-bg);
    box-shadow: inset 3px 0 0 var(--color-danger);
}

.el-row--flagged.el-row--clickable:hover {
    background: var(--danger-bg);
}

/* Строки дополнения (#1685). Подложка + полоса слева - тот же приём, что у ЧС:
   на мобилке строка разворачивается в карточку, и полоса остаётся единственным
   признаком роли, поэтому она обязана нести цвет роли, а не оттенок фона.

   :not(.el-row--flagged) обязателен: правила ЧС стоят выше в файле и при равной
   специфичности проиграли бы этим - помеченная возможным обходом ЧС строка потеряла бы
   красную подсветку, а это более критичный признак, чем «новая». Бейдж дополнения при
   этом остаётся - он в другой колонке. */
/* Подложка у добавленных строк нейтральная, а цвет несут полоса слева и бейдж. Заливка
   цветом по всей строке спорила с подсветкой чёрного списка и делала состав пёстрым:
   на вложении с несколькими раундами половина таблицы оказывалась крашеной. Серый при
   этом лёгкий - строка читается как обычная, просто помеченная. */
.el-row--supplement-pending:not(.el-row--flagged) {
    background: var(--surface-sunken);
    box-shadow: inset 3px 0 0 var(--warning);
}

.el-row--supplement-pending:not(.el-row--flagged).el-row--clickable:hover {
    background: var(--surface-sunken);
}

.el-row--supplement-approved:not(.el-row--flagged) {
    background: var(--surface-sunken);
    box-shadow: inset 3px 0 0 var(--info);
}

.el-row--supplement-approved:not(.el-row--flagged).el-row--clickable:hover {
    background: var(--surface-sunken);
}

/* Отклонённое дополнение остаётся в составе навсегда - приглушаем содержимое, чтобы
   строка не читалась как рабочая. Гасим сами значения, а не строку целиком: opacity на
   родителе утянула бы за собой и бейдж, который как раз объясняет, почему строка серая. */
.el-row--supplement-closed:not(.el-row--flagged) {
    box-shadow: inset 3px 0 0 var(--text-muted);
}

.el-row--supplement-closed .c-num,
.el-row--supplement-closed .val,
.el-row--supplement-closed .val-sub,
.el-row--supplement-closed .chip,
.el-row--supplement-closed .qty {
    opacity: 0.55;
}

.supplement-badge {
    display: inline-flex;
    max-width: 100%;
    margin-top: 3px;
}

/* Ключевая колонка машины на десктопе не уже 160px (расширена под этот же бейдж -
   см. min/minCompact у колонки 'number'), но на мобильной карточке (@768, узкий
   экран) поле снова может оказаться уже текста, а Badge.vue держит его в один ряд
   (white-space: nowrap): пилюля зажималась по max-width, и буквы вылезали наружу
   без фона под ними. Разрешаем перенос текста внутри самого бейджа - фон растёт
   вместе с ним, вместо того чтобы обрезать или ронять содержимое за свои границы.
   Три класса нужны, чтобы перебить scoped white-space: nowrap из Badge.vue по
   специфичности. */
.el-cell--key .supplement-line .supplement-badge {
    white-space: normal;
    text-align: left;
    line-height: 1.3;
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

/* Колонки - пропорциональные доли с минимумом: фиксированные проценты
   съедают остаток и схлопывают безразмерную колонку в ноль. */
.c-num {
    width: 22px;
    flex-shrink: 0;
    color: var(--text-muted);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
    user-select: none;
}

.c-state {
    width: 112px;
    flex-shrink: 0;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    overflow: hidden;
}

.c-state > * {
    max-width: 100%;
}

/* ТМЦ: количество прижато вправо, ширина приходит из описания колонок */
.c-state--qty {
    justify-content: flex-end;
}

.el-cell {
    min-width: 0;
}

.el-row .val {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    font-size: 13.5px;
}

.el-row .c-key .val {
    font-weight: 600;
    color: var(--text);
    font-size: 14.5px;
}

.el-row .c-places .val {
    font-weight: 600;
    color: var(--text);
    font-size: 14.5px;
}

.val-sub {
    display: block;
    margin-top: 1px;
    color: var(--text-muted);
    font-size: 12.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.cell-chips {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
}

.cell-chips .chips {
    flex: 0 1 auto;
}

.chips {
    display: flex;
    flex-wrap: nowrap;
    gap: 6px;
    min-width: 0;
    overflow: hidden;
}

.chip {
    display: inline-flex;
    align-items: center;
    flex-shrink: 0;
    max-width: 100%;
    padding: 3px 10px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    background: var(--surface);
    font-size: 12.5px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* Одиночный чип уступает ширине ячейки: без снятого flex-shrink он не сжимался и
   уезжал под обрезку ячейки без многоточия («Дебаркадер №1» читался как «Дебаркадер»). */
.chip--solo {
    flex-shrink: 1;
    min-width: 0;
}

/* Текст обязан лежать в своём элементе: сам чип - inline-flex, а на flex-контейнере
   text-overflow не действует, поэтому многоточия не появлялось, сколько ни ставь
   overflow: hidden на чип. */
.chip__text {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.chip--more {
    border-style: dashed;
    border-color: rgba(79, 91, 223, 0.4);
    background: var(--color-primary-tint);
    color: var(--accent-text);
    font-weight: 600;
}

.chip--assign {
    border-style: dashed;
    border-color: rgba(79, 91, 223, 0.45);
    background: var(--surface);
    color: var(--accent-text);
    font-weight: 700;
    line-height: 1;
    cursor: pointer;
    padding: 3px 9px;
}

.chip--assign:hover {
    background: var(--color-primary-tint);
}

/* Пустая колонка: вместо «+» пишем словом - иначе непонятно, что кнопка делает */
.chip--assign-empty {
    font-weight: 600;
    padding: 3px 12px;
}

.chip--empty {
    border: none;
    background: none;
    color: var(--accent-text);
    padding-left: 0;
}

/* Подсказка проекта: тёмный пузырёк по data-hint, а не браузерный title. */
.val[data-hint],
.blacklist-badge[data-hint],
.supplement-badge[data-hint] {
    position: relative;
}

/* У чипа своей подсказки нет: она рисовалась псевдоэлементом внутри него, а чип,
   строка чипов и сама ячейка обрезают содержимое (overflow: hidden держит раскладку
   таблицы) - подсказку срезало на всех трёх уровнях, и полное название не показывалось
   вовсе. Поэтому у чипов подсказка нативная, через title: её не обрезает ничто. */
.val[data-hint]::after,
.blacklist-badge[data-hint]::after,
.supplement-badge[data-hint]::after {
    content: attr(data-hint);
    position: absolute;
    bottom: calc(100% + 9px);
    left: 50%;
    transform: translateX(-50%);
    padding: 7px 11px;
    background: var(--hint-bg);
    color: var(--hint-text);
    font-size: 12px;
    font-weight: 500;
    white-space: nowrap;
    border-radius: var(--radius-sm);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s ease;
    z-index: 5;
}

/* Подсказка дополнения - фраза, а не пара слов: nowrap увёл бы её на полэкрана.
   Прижата к левому краю бейджа, а не отцентрирована: метка стоит в первой колонке, и
   центрированная подсказка уходила левее края карточки, где её срезал overflow контейнера
   детали - хвост фразы просто пропадал. От левого края она разворачивается вправо, где
   место есть. Стрелка сдвигается туда же, иначе указывала бы мимо. */
.supplement-badge[data-hint]::after {
    width: max-content;
    max-width: 240px;
    white-space: normal;
    text-align: left;
    line-height: 1.35;
    left: 0;
    transform: none;
}

.supplement-badge[data-hint]::before {
    left: 14px;
    transform: none;
}

.val[data-hint]::before,
.blacklist-badge[data-hint]::before,
.supplement-badge[data-hint]::before {
    content: '';
    position: absolute;
    bottom: calc(100% + 3px);
    left: 50%;
    transform: translateX(-50%);
    border: 6px solid transparent;
    border-top-color: var(--hint-bg);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s ease;
    z-index: 5;
}

/* Показ подсказки гейтим наличием курсора: на тач-экране :hover после тапа по строке
   залипает, и тёмный пузырёк шириной до 240px остаётся висеть поверх соседних полей и
   строк выше - метка дополнения читалась как "написанная поперёк карточки в случайном
   месте". Пальцу подсказка всё равно недоступна: навести, не нажав, нечем. */
@media (hover: hover) {
    .val[data-hint]:hover::after,
    .val[data-hint]:hover::before,
    .chip[data-hint]:hover::after,
    .chip[data-hint]:hover::before,
    .blacklist-badge[data-hint]:hover::after,
    .blacklist-badge[data-hint]:hover::before,
    .supplement-badge[data-hint]:hover::after,
    .supplement-badge[data-hint]:hover::before {
        opacity: 1;
    }
}

.qty {
    display: inline-flex;
    padding: 3px 11px;
    border-radius: var(--radius-pill);
    background: var(--color-primary-tint);
    color: var(--accent-text);
    font-size: 13px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
}

.blacklist-badge {
    flex-shrink: 0;
}

/* Строка, попавшая в чёрный список после подачи: из заявки не исчезает - заявка
   документ, - но перечёркивается, чтобы её не приняли за действующую. */
.el-row--blacklisted .el-cell,
.el-row--blacklisted .c-num {
  text-decoration: line-through;
  /* Линия красная, а текст остаётся читаемым: перечёркивание тут - признак запрета,
     и по цвету линии он различается с бледным «неактивно». */
  text-decoration-color: var(--danger);
  text-decoration-thickness: 2px;
  color: var(--text-muted);
}

.row-actions {
  position: relative;
}

/* Сдвоенная кнопка: действие и стрелка меню разделены линией, но выглядят
   одной кнопкой - у крайних скруглены только внешние углы. */
.split-btn {
  display: flex;
  align-items: stretch;
  flex-shrink: 0;
}

.split-btn__main,
.split-btn__toggle {
  padding: 5px 8px;
  font-size: 11px;
  white-space: nowrap;
}

.split-btn:not(.split-btn--single) .split-btn__main {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
  padding-right: 7px;
}

.split-btn__toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  padding-left: 6px;
  padding-right: 7px;
}

.split-btn__caret {
  transition: transform 180ms ease;
}

.split-btn__toggle--open .split-btn__caret {
  transform: rotate(180deg);
}

/* Сторону раскрытия выбирает updateRowMenuPosition: обычно вниз, а когда снизу мало
   места (помеченная строка часто последняя в списке) - вверх. Меню растёт от того
   края, которым прижато к кнопке, иначе появление читается как рывок. */
/* Раскрытие меню: только transform и opacity - остальное дёргает раскладку. */
.row-menu-enter-active,
.row-menu-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}

.row-menu-enter-from,
.row-menu-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

.row-actions__menu--up {
  transform-origin: bottom right;
}

.row-actions__menu--up.row-menu-enter-from,
.row-actions__menu--up.row-menu-leave-to {
  transform: translateY(4px) scale(0.98);
}

.row-actions__menu {
  transform-origin: top right;
  /* Слой выше карточки заявки (10002) и карточки элемента (10003), но ниже истории. */
  z-index: 10004;
  display: flex;
  flex-direction: column;
  min-width: 170px;
  padding: 4px;
  border-radius: var(--radius-md);
  background: var(--surface);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.16);
}

.row-actions__item {
  padding: 8px 10px;
  border: 0;
  border-radius: 10px;
  background: none;
  font-size: 12px;
  text-align: left;
  color: var(--text);
  cursor: pointer;
}

.row-actions__item:hover {
  background: var(--accent-tint);
}

.row-actions__item--danger {
  color: var(--danger-text);
}

.element-remove-btn {
  padding: 2px 10px;
  font-size: 11px;
  line-height: 18px;
  color: var(--danger-text);
}

.el-foot__right {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    justify-content: flex-end;
}

.el-foot__bulk {
    display: flex;
    align-items: center;
    gap: 6px;
}

.el-foot__bulk-label {
    color: var(--text-muted);
}

.el-foot__bulk-btn {
    padding: 3px 12px;
    font-size: 12.5px;
}

.el-foot {
    border-radius: 0 0 var(--radius-md) var(--radius-md);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    padding: 9px 12px;
    border-top: 1px solid var(--color-border);
    background: var(--accent-tint);
    font-size: 12.5px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
}

/* Лист выбора «Назначить всем»: пункт во всю ширину, подпись слева, высота 48 -
   палец попадает без прицеливания, а места хватает на полную формулировку. */
.bulk-sheet {
    display: flex;
    flex-direction: column;
    padding: 6px 0 10px;
}

.bulk-sheet__item {
    display: flex;
    align-items: center;
    width: 100%;
    min-height: 48px;
    padding: 0 20px;
    border: 0;
    background: transparent;
    color: var(--text);
    font: inherit;
    font-size: 15px;
    text-align: left;
    cursor: pointer;
    transition: background 0.15s ease;
}

.bulk-sheet__item:hover,
.bulk-sheet__item:active {
    background: var(--surface-sunken);
}

.loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px;
    gap: 15px;
}

.loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid var(--surface-2);
    border-top: 3px solid var(--color-primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

.loading-text {
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 500;
}

.no-data {
    text-align: center;
    color: var(--text-muted);
    padding: 40px 20px;
    font-size: 14px;
    font-style: italic;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

/* На мобилке блок вложения - по контенту: фикс min-height:300px оставлял много
   пустого белого снизу при коротких данных. Десктоп-стабильность не трогаем (W3.10). */
@media (max-width: 767.98px) {
    /* Единая сетка отступов: секции блока живут на тех же 12px, что и страница
       детали, поэтому боковые края шапки, ленты и подвала совпадают, а не идут
       тремя разными уступами (15 у секций против 14 у карточки). */
    .attachment-header-section,
    .custom-values-section,
    .attachment-data-section {
        padding: 12px;
    }

    .attachment-data-section {
        min-height: 0;
    }

    /* Поиск остаётся в строке заголовка, ужатый до 26px: отдельной строкой во всю
       ширину он весил как панель управления над списком из трёх машин. Селектор
       специфичнее собственного `.search{width:220px}` компонента поиска, порядок
       чанков тут не решает. */
    .el-section__head {
        flex-wrap: nowrap;
        gap: 8px;
    }

    /* flex-shrink: 0 - заголовок не отдаёт ширину растущему поиску. Раньше оба
       делили shrink поровну по content-basis, и на 320-360px "Автомобили"/
       "Сотрудники" резались до "А..."/"С..." (замер: доступно 242px на 320px,
       заголовку доставалось всего 61px из нужных 93). Ellipsis оставлен
       страховкой на непредвиденно длинный текст, а не рабочим режимом. */
    .el-section__head h5 {
        min-width: 0;
        flex-shrink: 0;
        font-size: 15px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    /* Счётчик - тоже не резервный донор ширины: без flex-shrink:0 трёхзначный
       итог (сотня машин/сотрудников) сжимался бы наравне с заголовком. */
    .el-section__head .el-count {
        flex-shrink: 0;
    }

    /* 36px - тач-таргет заголовков этого экрана (RefreshButton, шапки
       справочников); прежние 26px были меньше даже собственных 30px десктопа -
       владелец не мог попасть пальцем ("Поиск очень маленький").
       min-width: 70px - пол, ниже которого полю сжиматься некуда: заголовок и
       счётчик теперь забирают свою ширину первыми (flex-shrink:0 выше), и без
       пола поиск ужимался бы вплоть до одной иконки. */
    .el-section__head .el-search {
        flex: 1 1 0;
        width: auto;
        min-width: 70px;
        height: 36px;
        margin-left: 0;
        padding: 0 10px;
    }

    .el-section__head .el-search :deep(.search__input) {
        font-size: 14px;
    }

    .el-section__head .el-search :deep(.search__icon) {
        width: 15px;
        height: 15px;
    }

    /* Строку разворачивает в карточку глобальный responsive-tables.css:
       порядковый номер и горизонтальная прокрутка там не нужны. */
    .el-scroll--scrollable {
        overflow-x: hidden;
    }

    .el-inner {
        min-width: 0 !important;
    }

    .el-row .c-num {
        display: none;
    }

    /* Карточки строк уже несут рамку, фон и скругление, поэтому лента остаётся без
       своих: иначе рамка идёт вторым контуром вплотную к карточке (замер: лента 334px
       против карточки 332px), а её скруглённый низ упирается в прямой верх подвала. */
    .el-table {
        border: none;
        border-radius: 0;
        background: transparent;
    }

    .el-foot {
        margin-top: 8px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        flex-wrap: wrap;
    }

    /* Компактно на вид, крупно под палец: пилюля 28px, зона нажатия 44 невидимым
       расширением - подвал от этого не растёт. */
    .el-foot__bulk-btn {
        position: relative;
        min-height: 28px;
        white-space: nowrap;
    }

    .el-foot__bulk-btn::before {
        content: '';
        position: absolute;
        inset: -8px -4px;
    }

    /* Карточка строки: боковые 12px - ровно столько же, сколько у секции и подвала,
       поэтому их края совпадают (глобальное правило даёт 14 и несёт !important,
       перебиваем составным селектором). Зазор между полями снимаем: ритм задают
       отступы самих полей, а собственный gap строки давал бы его дважды.

       Вертикальные 8px равны отступу поля (ниже), поэтому от края карточки до первого
       значения ровно столько же, сколько между соседними значениями через разделитель:
       16px по всей карточке. */
    .el-table .el-row {
        gap: 0 !important;
        padding: 8px 12px !important;
    }

    /* Подписи полей в карточке не показываем: значение говорит само за себя, а колонка
       меток съедала ширину гос. номера и ФИО. Правило-источник в responsive-tables.css
       стоит на той же специфичности, поэтому !important - иначе исход решает порядок
       загрузки чанков. */
    .el-row .el-cell::before {
        display: none !important;
    }

    /* Поле карточки: высоту задаёт содержимое, а сверху и снизу - одинаковые 8px.
       Так расстояние от значения до разделителя одно и то же с обеих сторон и у всех
       трёх типов вложения; фиксированные 30px давали текстовому полю 7px до черты, а
       полю с чипами - вдвое меньше, потому что коробка чипа выше строки текста.

       flex сбрасываем в `0 0 auto`: доли колонок (`flex: 44 1 0`) приходят инлайном из
       cellStyle() и рассчитаны на строку таблицы, а в карточке строка перевёрнута в
       колонку, и базис 0 относится там к ВЫСОТЕ - ячейка получала долю высоты по своему
       grow вместо высоты содержимого. Заодно уходит `min-height`: у флекс-элемента он
       отменяет автоматический минимум (`min-height: auto` = не ниже контента), который
       и есть страховка от наложения. Вместе это и печатало должность и места прохода
       поверх разделителя, когда значение не умещалось в одну строку.

       Переносится из полей только ключевое, и `row-gap` там свой, мелкий: должность,
       ушедшая под ФИО, должна читаться подписью к значению, а не отдельным полем.

       Разделитель полей рисуем сверху, а не снизу: у машин и сотрудников последней в
       строке стоит колонка действий без подписи, поэтому глобальное
       `[data-label]:last-child` не снимало пунктир с последнего поля и он висел
       оторванной чертой над нижним краем карточки.

       Фиксированной высоты у поля больше нет, и это главное. `min-height` на
       флекс-элементе отменяет автоматический минимум (`min-height: auto`) - ту самую
       страховку, которая не даёт содержимому вылезти за коробку. С ней ячейка держала
       30px при содержимом в 68 (длинная должность плюс места прохода), и лишние 38
       печатались поверх разделителя и поверх соседнего поля. Высоту теперь задаёт
       содержимое.

       Отступ один и тот же по обе стороны любой границы: 8px от значения до пунктира и
       8px от пунктира до следующего значения. Раньше зазор зависел от того, текст в
       поле или пилюля (7px против 3.5px при фиксированных 30px), - отсюда «хромают
       отступы вверх и вниз».

       row-gap меньше column-gap намеренно: под ключевое значение переносится подпись
       (должность под ФИО, марка под номером), и она должна читаться как подпись к
       нему, а не как ещё одно поле. */
    .el-table .el-row .el-cell {
        flex: 0 0 auto !important;
        padding: 8px 0 !important;
        column-gap: 8px;
        row-gap: 2px;
        align-items: center;
        justify-content: flex-start !important;
        text-align: left !important;
        border-bottom: none !important;
    }

    .el-row .el-cell ~ .el-cell {
        border-top: 1px dashed color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    /* Колонка действий - такая же строка слева, а не 112px, прижатых к правому краю.
       Пустую (нет ни бейджа ЧС, ни кнопки) не отбиваем чертой: она невидима, а линия
       над ней осталась бы висеть у нижнего края карточки. */
    .el-row .c-state {
        width: 100%;
        justify-content: flex-start;
    }

    /* Пустая колонка отступов не получает: 8px сверху и снизу дали бы ей 16px высоты
       и карточка кончалась бы полосой пустоты. */
    .el-row .el-cell ~ .c-state:not(:empty) {
        padding: 8px 0;
        align-items: center;
        border-top: 1px dashed color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    .el-row .chips {
        flex-wrap: wrap;
        justify-content: flex-start;
        overflow: visible;
    }

    .el-row .val,
    .val-sub {
        white-space: normal;
        text-align: left;
    }

    /* Метке дополнения даём свою строку под значением, иначе она сжимает ФИО и
       гос. номер до многоточия. */
    .el-row .el-cell--key {
        flex-wrap: wrap;
    }

    /* Гос. номер и марка (ФИО и должность) - одной строкой: номер жирный слева,
       марка серым следом. Своя строка под марку добавляла карточке четвёртое поле
       и пунктир там, где хватает одной строки. */
    .el-row .el-cell--key .val,
    .el-row .el-cell--key .val-sub {
        flex: 0 1 auto;
        min-width: 0;
    }

    .el-row .el-cell--key .val-sub {
        margin-top: 0;
    }

    /* Метка дополнения занимает строку ячейки целиком и начинается слева - место у неё
       теперь одно при любом содержимом. Прежний `margin-left: auto` прижимал её к
       правому краю той строки, куда её занесло переносом: у короткого наименования -
       справа от значения, у длинного - под ним, у гос. номера с длинной маркой - под
       маркой. Нижние 4px не дают пилюле лечь на пунктир следующего поля. */
    .el-row .el-cell--key .supplement-line {
        flex: 0 0 100%;
        min-width: 0;
        padding-bottom: 4px;
    }

    .el-row .el-cell--key .supplement-badge {
        max-width: 100%;
        margin-top: 0;
    }
}
</style>
