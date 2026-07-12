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

    <div class="attachment-data-section">
      <div class="attachment-data">
        <!-- Автомобили -->
        <div
          v-if="attachment.attachment_type === 'cars'"
          class="cars-section"
        >
          <h5>Список автомобилей</h5>
          <template v-if="loading">
            <div class="loading-container">
              <div class="loading-spinner" />
              <span class="loading-text">Загрузка...</span>
            </div>
          </template>
          <template v-else>
            <div
              v-if="cars.length > 0"
              class="cars-list"
            >
              <div
                v-for="(car, index) in cars"
                :key="car.id"
                class="car-item"
                :class="{ 'car-item--flagged': isFlagged(car) }"
                @click="$emit('open-vehicle', car)"
              >
                <div class="car-item-content">
                  <div class="item-number">
                    {{ index + 1 }}.
                  </div>
                  <div class="car-main-info">
                    <span class="car-number">{{ car.car_number }}</span>
                    <span class="car-brand">{{ car.car_brand }}</span>
                  </div>
                  <div
                    v-if="car.unload_places && car.unload_places.length > 0"
                    class="unload-places-container"
                    :title="getFullPlacesList(car.unload_places)"
                  >
                    <span class="places-list">
                      {{ getTruncatedPlacesList(car.unload_places) }}
                    </span>
                  </div>
                  <div
                    v-if="car.target_tables && car.target_tables.length > 0"
                    class="target-tables-container"
                    :title="getFullTablesList(car.target_tables)"
                  >
                    <span class="tables-list">
                      {{ getTruncatedTablesList(car.target_tables) }}
                    </span>
                  </div>
                  <Badge
                    v-if="car.blacklist_similar"
                    class="blacklist-badge"
                    :variant="blacklistVariant(car.blacklist_similar)"
                    size="sm"
                    dot
                    :title="blacklistTooltip(car.blacklist_similar)"
                  >
                    {{ blacklistLabel(car.blacklist_similar) }}
                  </Badge>
                  <button
                    v-if="canOverride && isFlagged(car)"
                    type="button"
                    class="lk-button lk-button--danger blacklist-override-btn"
                    @click.stop="$emit('override-element', { label: car.car_number, flag: car.blacklist_similar })"
                  >
                    Пропустить
                  </button>
                </div>
              </div>
            </div>
            <div
              v-else
              class="no-data"
            >
              Нет данных об автомобилях
            </div>
          </template>
        </div>

        <!-- Сотрудники -->
        <div
          v-if="attachment.attachment_type === 'people'"
          class="employees-section"
        >
          <h5>Сотрудники</h5>
          <template v-if="loading">
            <div class="loading-container">
              <div class="loading-spinner" />
              <span class="loading-text">Загрузка...</span>
            </div>
          </template>
          <template v-else>
            <div
              v-if="employees.length > 0"
              class="employees-list"
            >
              <div
                v-for="(employee, index) in employees"
                :key="employee.id"
                class="employee-item"
                :class="{ 'employee-item--flagged': isFlagged(employee) }"
                @click="$emit('open-employee', employee)"
              >
                <div class="employee-item-content">
                  <div class="item-number">
                    {{ index + 1 }}.
                  </div>
                  <div class="employee-main-info">
                    <span class="employee-name">{{ employee.last_name }} {{ employee.first_name }} {{ employee.middle_name || '' }}</span>
                    <span class="employee-position">{{ employee.position }}</span>
                  </div>
                  <div
                    v-if="employee.target_tables && employee.target_tables.length > 0"
                    class="target-tables-container"
                    :title="getFullTablesList(employee.target_tables)"
                  >
                    <span class="tables-list">
                      {{ getTruncatedTablesList(employee.target_tables) }}
                    </span>
                  </div>
                  <Badge
                    v-if="employee.blacklist_similar"
                    class="blacklist-badge"
                    :variant="blacklistVariant(employee.blacklist_similar)"
                    size="sm"
                    dot
                    :title="blacklistTooltip(employee.blacklist_similar)"
                  >
                    {{ blacklistLabel(employee.blacklist_similar) }}
                  </Badge>
                  <button
                    v-if="canOverride && isFlagged(employee)"
                    type="button"
                    class="lk-button lk-button--danger blacklist-override-btn"
                    @click.stop="$emit('override-element', { label: employeeFullName(employee), flag: employee.blacklist_similar })"
                  >
                    Пропустить
                  </button>
                </div>
              </div>
            </div>
            <div
              v-else
              class="no-data"
            >
              Нет данных о сотрудниках
            </div>
          </template>
        </div>

        <!-- ТМЦ -->
        <div
          v-if="attachment.attachment_type === 'items'"
          class="items-section"
        >
          <h5>Товарно-материальные ценности</h5>
          <template v-if="loading">
            <div class="loading-container">
              <div class="loading-spinner" />
              <span class="loading-text">Загрузка...</span>
            </div>
          </template>
          <template v-else>
            <div
              v-if="items.length > 0"
              class="items-list"
            >
              <div
                v-for="(item, index) in items"
                :key="item.id"
                class="item-item"
              >
                <div class="item-item-content">
                  <div class="item-number">
                    {{ index + 1 }}.
                  </div>
                  <span class="item-name">{{ item.name }}</span>
                  <span class="item-count">Количество: {{ item.count }}</span>
                </div>
              </div>
            </div>
            <div
              v-else
              class="no-data"
            >
              Нет данных о ТМЦ
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue'

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
        }
    },
    emits: ['open-vehicle', 'open-employee', 'override-element'],
    methods: {
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

        blacklistLabel(flag) {
            return flag.overridden ? 'пропуск подтверждён' : 'похоже на ЧС';
        },

        blacklistTooltip(flag) {
            const value = flag.matched_value || '';
            const base = value
                ? `Возможный обход чёрного списка. Похоже на: ${value}`
                : 'Возможный обход чёрного списка.';
            return flag.matched_reason ? `${base} (${flag.matched_reason})` : base;
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
        },

        getFullPlacesList(places) {
            if (!places || !places.length) return '';
            return places.map(p => p.name).join(', ');
        },

        getTruncatedPlacesList(places) {
            if (!places || !places.length) return '';
            const maxPlaces = 2;
            const placeNames = places.map(p => p.name);
            if (placeNames.length <= maxPlaces) {
                return placeNames.join(', ');
            }
            const shownPlaces = placeNames.slice(0, maxPlaces);
            return `${shownPlaces.join(', ')} и др.`;
        },

        getFullTablesList(tables) {
            if (!tables || !tables.length) return '';
            return tables.map(t => t.display_name).join(', ');
        },

        getTruncatedTablesList(tables) {
            if (!tables || !tables.length) return '';
            const maxTables = 2;
            const tableNames = tables.map(t => t.display_name);
            if (tableNames.length <= maxTables) {
                return tableNames.join(', ');
            }
            const shownTables = tableNames.slice(0, maxTables);
            return `${shownTables.join(', ')} и др.`;
        }
    }
}
</script>

<style scoped>
.attachment-details {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.attachment-header-section {
    padding: 15px;
    border-bottom: 1px solid #e6e6e6;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px 24px;
}

.attachment-details h4 {
    font-size: 18px;
    color: #4F5BDF;
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
    border-bottom: 1px solid #e6e6e6;
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
    max-height: 500px;
}

.attachment-data {
    margin-top: 0;
}

.cars-section h5,
.employees-section h5,
.items-section h5 {
    font-size: 16px;
    color: #333;
    margin: 0 0 15px 0;
    font-weight: 700;
    padding-top: 10px;
    border-top: 1px solid #e6e6e6;
}

.cars-section:first-child h5,
.employees-section:first-child h5,
.items-section:first-child h5 {
    border-top: none;
    padding-top: 0;
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
    border-top: 3px solid #4F5BDF;
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

.cars-list, .employees-list, .items-list {
    display: flex;
    flex-direction: column;
    max-height: 330px;
    overflow: scroll;
    gap: 8px;
}

.car-item, .employee-item, .item-item {
    padding: 12px;
    background: #f9f9f9;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
    animation: slideIn 0.3s ease-out forwards;
    opacity: 0;
    transform: translateY(10px);
    cursor: pointer;
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

.car-item:hover, .employee-item:hover, .item-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.car-item--flagged, .employee-item--flagged {
    background: #fff5f5;
    border-color: #fecaca;
}

.car-item--flagged:hover, .employee-item--flagged:hover {
    border-color: #f87171;
    background: #fee2e2;
}

.blacklist-badge {
    flex-shrink: 0;
    margin-left: auto;
}

/* кнопка идёт сразу за бейджем (тот уже прижат вправо через margin-left:auto) */
.blacklist-override-btn {
    flex-shrink: 0;
    padding: 5px 12px;
    font-size: 12px;
}

.car-item-content, .employee-item-content, .item-item-content {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    width: 100%;
    gap: 12px;
}

.item-number {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 500;
    min-width: 20px;
    flex-shrink: 0;
    pointer-events: none;
    user-select: none;
}

.car-main-info, .employee-main-info {
    display: flex;
    gap: 15px;
    min-width: 250px;
    flex-shrink: 0;
}

.car-number {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.car-brand {
    color: #666;
    font-size: 14px;
}

.employee-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.employee-position {
    color: #666;
    font-size: 14px;
}

.unload-places-container, .target-tables-container {
    display: flex;
    gap: 8px;
    font-size: 13px;
    align-items: flex-start;
    flex: 1;
    min-width: 0;
    position: relative;
    cursor: help;
    z-index: 1000;
}

.places-list, .tables-list {
    color: #333;
    line-height: 1.4;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: end;
    user-select: none;
}

.item-item-content {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    width: 100%;
}

.item-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
    flex: 1;
}

.item-count {
    color: #4F5BDF;
    font-size: 14px;
    font-weight: 600;
    background: rgba(79, 91, 223, 0.1);
    padding: 4px 10px;
    border-radius: 15px;
    white-space: nowrap;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

/* На мобилке блок вложения - по контенту: фикс min-height:300px оставлял много
   пустого белого снизу при коротких данных. Десктоп-стабильность не трогаем (W3.10). */
@media (max-width: 768px) {
    .attachment-data-section {
        min-height: 0;
    }
}
</style>
