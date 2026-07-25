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
          <h5>{{ sectionTitle }}</h5>
          <span
            v-if="!loading && rows.length"
            class="el-count"
            data-testid="attachment-elements-count"
          >{{ rows.length }}</span>
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
                v-for="(row, index) in rows"
                :key="row.id"
                class="el-row rt-row"
                :class="{
                  'el-row--flagged': isFlagged(row),
                  'el-row--clickable': isClickable
                }"
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
                  :class="col.cls"
                  :style="cellStyle(col)"
                  :data-label="col.label"
                >
                  <template v-if="col.type === 'chips'">
                    <div class="chips">
                      <span
                        v-for="chip in visibleChips(row, col)"
                        :key="chip.key"
                        class="chip"
                        :class="{ 'chip--more': chip.isMore }"
                        :data-hint="chip.hint"
                        :data-testid="chip.isMore ? 'attachment-chip-more' : 'attachment-chip'"
                      >{{ chip.text }}</span>
                      <span
                        v-if="!chipItems(row, col).length"
                        class="chip chip--empty"
                      >—</span>
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
                    <!-- Подстрока рисуется всегда: без неё строка без марки
                         оказывалась ниже соседних и лента "дышала". -->
                    <span
                      v-if="col.sub && isCompact"
                      class="val-sub"
                    >{{ col.sub(row) || '—' }}</span>
                  </template>
                </div>

                <div
                  v-if="hasStateColumn"
                  class="c-state"
                >
                  <button
                    v-if="canOverride && isFlagged(row)"
                    type="button"
                    class="lk-button lk-button--danger blacklist-override-btn"
                    data-testid="blacklist-override-btn"
                    @click.stop="$emit('override-element', { label: rowLabel(row), flag: row.blacklist_similar })"
                  >
                    Пропустить
                  </button>
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
                </div>
              </div>
            </div>
          </div>

          <div class="el-foot">
            <span data-testid="attachment-elements-total">{{ totalText }}</span>
            <Badge
              v-if="flaggedCount"
              variant="danger"
              size="sm"
              dot
              data-testid="attachment-flagged-summary"
            >
              {{ flaggedCount }} похоже на ЧС
            </Badge>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue'

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
/** Запас, чтобы значение не упиралось вплотную в край колонки. */
const TEXT_SIDE_SPACE = 6;

export default {
    name: 'ApplicationAttachmentDetail',
    components: { Badge },
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
        // Показываем "Пропустить" только ответственному - у остальных нет права на override.
        canOverride: {
            type: Boolean,
            default: false
        },
        // "Доступные мне" показывают тот же список, но карточку элемента там не
        // открывают - строка не должна выглядеть кликабельной.
        interactive: {
            type: Boolean,
            default: true
        }
    },
    emits: ['open-vehicle', 'open-employee', 'override-element'],
    data() {
        return {
            containerWidth: 0,
            isNarrowViewport: false,
            resizeObserver: null,
            viewportQuery: null,
            // Ширина колонок с чипами: по ней считаем, сколько названий влезает
            // целиком. Пусто до первого замера - тогда работает запасной расчёт.
            chipColumnWidths: {},
            textMeasureContext: null
        };
    },
    computed: {
        type() {
            return this.attachment.attachment_type;
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
                        grow: 22, min: 114,
                        growCompact: 38, minCompact: 140,
                        value: row => row.car_number,
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
                        grow: 30, min: 100,
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

        /** ТМЦ не несут флага ЧС и не открывают карточку - колонка действий им не нужна. */
        hasStateColumn() {
            return this.type !== 'items';
        },

        isClickable() {
            return this.interactive && this.hasStateColumn;
        },

        /**
         * Узкий контейнер: часть колонок схлопывается. На телефоне режим не
         * включаем - там строка разворачивается в карточку (responsive-tables.css),
         * и каждому полю нужна своя подпись.
         */
        isCompact() {
            if (this.isNarrowViewport) return false;
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

        totalText() {
            if (this.type === 'items') {
                const units = this.items.reduce((sum, item) => sum + (Number(item.count) || 0), 0);
                return `Всего позиций: ${this.rows.length}, единиц: ${units}`;
            }
            return `Всего: ${this.rows.length}`;
        },

        flaggedCount() {
            return this.rows.filter(row => this.isFlagged(row)).length;
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
        }
    },
    mounted() {
        if (typeof window.matchMedia === 'function') {
            this.viewportQuery = window.matchMedia('(max-width: 767.98px)');
            this.isNarrowViewport = this.viewportQuery.matches;
            this.viewportQuery.addEventListener('change', this.onViewportChange);
        }

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
            const available = this.chipColumnWidths[col.key];
            if (!available) {
                return this.isCompact ? CHIPS_VISIBLE_COMPACT : CHIPS_VISIBLE_WIDE;
            }

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
                isMore: false
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
    background: white;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
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
    color: var(--color-primary);
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
    color: #a2a2a2;
    font-weight: 400;
    min-width: 110px;
    font-size: 14px;
}

.date-value, .time-value {
    color: #000;
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
    color: #a2a2a2;
    font-weight: 400;
}

.custom-value-text {
    color: #000;
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
    color: var(--color-primary);
    font-size: 12px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
}

.el-table {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: #fff;
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
    background: #fbfbfd;
    border-bottom: 1px solid var(--color-border);
    font-size: 12px;
    font-weight: 600;
    color: #8a8fa3;
}

.el-row {
    min-height: 46px;
    padding-top: 8px;
    padding-bottom: 8px;
    border-bottom: 1px solid #f2f2f5;
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
    background: #fffafa;
    box-shadow: inset 3px 0 0 var(--color-danger);
}

.el-row--flagged.el-row--clickable:hover {
    background: #fff4f4;
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
    color: #b0b3c2;
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
    color: #6b6f80;
    font-size: 13.5px;
}

.el-row .c-key .val {
    font-weight: 600;
    color: #222;
    font-size: 14.5px;
}

.el-row .c-places .val {
    font-weight: 600;
    color: #222;
    font-size: 14.5px;
}

.val-sub {
    display: block;
    margin-top: 1px;
    color: #6b6f80;
    font-size: 12.5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    background: #fff;
    font-size: 12.5px;
    color: #444;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.chip--more {
    border-style: dashed;
    border-color: rgba(79, 91, 223, 0.4);
    background: var(--color-primary-tint);
    color: var(--color-primary);
    font-weight: 600;
    cursor: help;
}

.chip--empty {
    border: none;
    background: none;
    color: #c2c5d2;
    padding-left: 0;
}

/* Подсказка проекта: тёмный пузырёк по data-hint, а не браузерный title. */
.val[data-hint],
.chip[data-hint],
.blacklist-badge[data-hint] {
    position: relative;
}

.val[data-hint]::after,
.chip[data-hint]::after,
.blacklist-badge[data-hint]::after {
    content: attr(data-hint);
    position: absolute;
    bottom: calc(100% + 9px);
    left: 50%;
    transform: translateX(-50%);
    padding: 7px 11px;
    background: #333;
    color: #fff;
    font-size: 12px;
    font-weight: 500;
    white-space: nowrap;
    border-radius: var(--radius-sm);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s ease;
    z-index: 5;
}

.val[data-hint]::before,
.chip[data-hint]::before,
.blacklist-badge[data-hint]::before {
    content: '';
    position: absolute;
    bottom: calc(100% + 3px);
    left: 50%;
    transform: translateX(-50%);
    border: 6px solid transparent;
    border-top-color: #333;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s ease;
    z-index: 5;
}

.val[data-hint]:hover::after,
.val[data-hint]:hover::before,
.chip[data-hint]:hover::after,
.chip[data-hint]:hover::before,
.blacklist-badge[data-hint]:hover::after,
.blacklist-badge[data-hint]:hover::before {
    opacity: 1;
}

.qty {
    display: inline-flex;
    padding: 3px 11px;
    border-radius: var(--radius-pill);
    background: var(--color-primary-tint);
    color: var(--color-primary);
    font-size: 13px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
}

.blacklist-badge {
    flex-shrink: 0;
}

.blacklist-override-btn {
    flex-shrink: 0;
    padding: 5px 12px;
    font-size: 12px;
    white-space: nowrap;
}

.el-foot {
    border-radius: 0 0 var(--radius-md) var(--radius-md);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    padding: 9px 12px;
    border-top: 1px solid var(--color-border);
    background: #fbfbfd;
    font-size: 12.5px;
    color: #8a8fa3;
    font-variant-numeric: tabular-nums;
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
    border: 3px solid #f3f3f3;
    border-top: 3px solid var(--color-primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

.loading-text {
    color: #666;
    font-size: 14px;
    font-weight: 500;
}

.no-data {
    text-align: center;
    color: #a2a2a2;
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
    .attachment-data-section {
        min-height: 0;
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

    .el-row .chips {
        flex-wrap: wrap;
        justify-content: flex-end;
        overflow: visible;
    }

    .el-row .val,
    .val-sub {
        white-space: normal;
        text-align: right;
    }

    .blacklist-override-btn {
        width: 100%;
    }
}
</style>
