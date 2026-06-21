<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        :style="{ zIndex: overlayZIndex }"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div class="modal-wrapper">
          <!-- Основное модальное окно с деталями ТС -->
          <div
            class="modal-content compact-modal main-modal"
            :class="{ 'shifted': isMainShifted }"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3 class="modal-title">
                {{ modalTitle }}
              </h3>
              <div
                v-if="showCarFeatures || (canManageBlacklist && hasVehicleIdentity)"
                class="header-actions"
              >
                <button
                  v-if="showCarFeatures"
                  class="history-btn"
                  @click="openCarHistory"
                >
                  <span>Полная история</span>
                </button>
                <button
                  v-if="canOpenApplication"
                  class="application-btn"
                  @click="openApplication"
                >
                  <span>Открыть заявку</span>
                </button>
                <button
                  v-if="canManageBlacklist && hasVehicleIdentity && !isBlacklisted"
                  class="blacklist-add-btn"
                  @click="openAddBlacklist"
                >
                  <span>В ЧС</span>
                </button>
              </div>
              <button
                class="modal-close"
                @click="close"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>
                    
            <div class="modal-body">
              <!-- Секция статуса ЧС -->
              <div
                v-if="isBlacklisted"
                class="bl-section"
              >
                <div class="bl-section-head">
                  <span class="bl-section-dot" />
                  Машина в чёрном списке
                </div>
                <div
                  v-if="blacklistReason"
                  class="bl-section-reason"
                >
                  <span class="bl-section-label">Причина:</span> {{ blacklistReason }}
                </div>
              </div>

              <!-- Подозрение на обход ЧС (#481, срез C): мягкое предупреждение о похожем на
                   запись ЧС элементе + управление пропуском (подтвердить/отменить), от которого
                   зависит гейт согласования заявки. Видно только в контексте заявки. -->
              <div
                v-if="blacklistSimilar"
                class="bl-suspicion-section"
                :class="{ 'is-resolved': blacklistSimilar.overridden }"
              >
                <div class="bl-suspicion-head">
                  <span class="bl-suspicion-dot" />
                  Подозрение на обход чёрного списка
                </div>
                <div class="bl-suspicion-row">
                  <span class="bl-suspicion-label">Похоже на:</span> {{ blacklistSimilar.matched_value }}
                </div>
                <div
                  v-if="blacklistSimilar.matched_reason"
                  class="bl-suspicion-row"
                >
                  <span class="bl-suspicion-label">Причина:</span> {{ blacklistSimilar.matched_reason }}
                </div>

                <div
                  v-if="blacklistSimilar.overridden"
                  class="bl-suspicion-foot"
                >
                  <span class="bl-suspicion-confirmed">Пропуск подтверждён</span>
                  <button
                    v-if="canCancelOverride"
                    type="button"
                    class="bl-suspicion-btn bl-suspicion-btn--cancel"
                    @click="$emit('cancel-override')"
                  >
                    Отменить
                  </button>
                </div>
                <div
                  v-else
                  class="bl-suspicion-foot"
                >
                  <span class="bl-suspicion-blocked">Согласование заблокировано до подтверждения пропуска</span>
                  <button
                    v-if="canOverride"
                    type="button"
                    class="bl-suspicion-btn bl-suspicion-btn--allow"
                    @click="$emit('override')"
                  >
                    Всё равно пропустить
                  </button>
                </div>
              </div>

              <!-- Блок предупреждения об активной заявке -->
              <div
                v-if="activeInfo"
                class="active-warning-section"
              >
                <div class="warning-content">
                  <div class="warning-text">
                    <p class="warning-title">
                      На это авто уже есть активная заявка!
                    </p>
                    <p class="warning-details">
                      Действует до: {{ formatDate(activeInfo.entry_date_to) }} {{ formatTime(activeInfo.entry_time_to) }}<br>
                      Заявка №{{ activeInfo.application_number }}<br>
                      Организация: {{ activeInfo.organization_name || 'Не указана' }}<br>
                      Компания: {{ activeInfo.company_name || 'Не указана' }}
                    </p>
                  </div>
                </div>
              </div>

              <div
                v-if="vehicle"
                class="vehicle-details"
              >
                <!-- Секция Удаление (только корзина) -->
                <div
                  v-if="source === 'trash' && (vehicle.deletedByName || vehicle.deletedAtText)"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      Удаление
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Дата и время удаления:</span>
                        <span class="detail-value">{{ vehicle.deletedAtText || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Кто удалил:</span>
                        <span class="detail-value">{{ vehicle.deletedByName || '-' }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Секция Основная информация -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Основная информация
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Номер Т/С:</span>
                        <span class="detail-value">{{ vehicle.plateNumber || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Марка:</span>
                        <span class="detail-value">{{ vehicle.mark || 'Не указано' }}</span>
                      </div>
                      <div
                        v-if="getFormatName(vehicle.formatId)"
                        class="detail-item full-width"
                      >
                        <span class="detail-label">Формат номера:</span>
                        <span class="detail-value">{{ getFormatName(vehicle.formatId) }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Организация:</span>
                        <span class="detail-value">{{ vehicle.organization || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Компания:</span>
                        <span class="detail-value">{{ vehicle.company || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Действует до:</span>
                        <span class="detail-value">{{ formatDate(vehicle.entry_date_to) || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Время пребывания:</span>
                        <span class="detail-value">{{ formatTimeRange(vehicle.entry_time_from, vehicle.entry_time_to) || '-' }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Секция Места разгрузки -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Места разгрузки
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="places-list">
                      <div 
                        v-for="placeId in vehicle.unloadPlaces" 
                        :key="placeId"
                        class="place-item"
                        :class="{ 'active': showPlaceModal && selectedUnloadPlace && selectedUnloadPlace.id === placeId }"
                        @click="showUnloadPlaceDetails(placeId)"
                      >
                        {{ getPlaceName(placeId) }}
                      </div>
                      <div
                        v-if="!vehicle.unloadPlaces || vehicle.unloadPlaces.length === 0"
                        class="no-places"
                      >
                        Места разгрузки не указаны
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Секция Статус (только для автомобилей; в корзине не показываем) -->
                <div
                  v-if="showStatusSection"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      Статус
                    </h4>
                  </div>
                  <div class="section-body">
                    <div
                      class="status-badge"
                      :class="getStatusClass"
                    >
                      {{ getStatusText }}
                    </div>
                  </div>
                </div>

                <!-- Секция История въездов и выездов (только entry/exit) -->
                <div
                  v-if="showCarFeatures"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      История въездов и выездов
                    </h4>
                    <button
                      class="export-btn"
                      :disabled="entryExitHistory.length === 0 || isExporting"
                      @click="exportHistory"
                    >
                      <img
                        v-if="!isExporting"
                        src="@/assets/icons/export.png"
                        class="export-icon"
                      >
                      <span v-if="!isExporting">Экспорт</span>
                      <div
                        v-else
                        class="export-loader"
                      />
                    </button>
                  </div>
                  <div class="section-body">
                    <div
                      v-if="loadingHistory"
                      class="loading-container"
                    >
                      <LoaderSpinner label="Загрузка истории…" />
                    </div>
                                    
                    <div
                      v-else-if="entryExitHistory.length === 0"
                      class="no-history"
                    >
                      История въездов и выездов отсутствует
                    </div>
                                    
                    <div
                      v-else
                      class="history-timeline"
                    >
                      <div 
                        v-for="(item, index) in entryExitHistory" 
                        :key="item.id" 
                        class="history-item"
                      >
                        <div
                          class="timeline-dot"
                          :class="getActionClass(item)"
                        />
                        <div
                          v-if="index < entryExitHistory.length - 1"
                          class="timeline-line"
                        />
                                            
                        <div class="history-content">
                          <div class="history-header">
                            <span class="user-name">{{ item.user_name || 'Система' }}</span>
                            <span class="action-time">{{ formatDateTime(item.created_at) }}</span>
                          </div>
                                                
                          <div class="action-text">
                            {{ getActionText(item) }}
                          </div>
                                                
                          <div class="action-comment">
                            {{ getActionComment(item) }}
                          </div>
                          <div
                            v-if="item.table_name"
                            class="place-name"
                          >
                            {{ item.table_name }}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Дополнительное модальное окно с деталями места разгрузки -->
          <transition 
            name="place-slide"
            @after-leave="onPlaceLeave"
          >
            <div
              v-if="showPlaceModal"
              class="place-modal-container"
            >
              <UnloadPlaceModal
                :place="selectedUnloadPlace"
                :all-unloading-places="allUnloadingPlaces"
                @close="closeUnloadPlaceDetails"
              />
            </div>
          </transition>
        </div>
      </div>
    </transition>
  </Teleport>

  <!-- Модальное окно полной истории автомобиля -->
  <CarHistoryModal
    v-if="showCarHistoryModal"
    :car-id="vehicle?.id"
    :car-number="vehicle?.plateNumber || vehicle?.car_number || 'по факту'"
    :car-brand="vehicle?.mark || vehicle?.car_brand || ''"
    :organization-id="vehicle?.organizationId"
    :organization-name="vehicle?.organization"
    :company-id="vehicle?.companyId"
    :company-name="vehicle?.company"
    :current-user-id="currentUserId"
    :current-user-name="currentUserName"
    @close="showCarHistoryModal = false"
    @open-application="$emit('open-application', $event)"
  />

  <AddToBlacklistModal
    :show="showAddBlacklist"
    type="vehicle"
    :entity-label="vehicleLabel"
    :saving="savingBlacklist"
    :error="blacklistError"
    @close="closeAddBlacklist"
    @confirm="submitAddBlacklist"
  />
</template>

<script>
import { apiRequest } from '@/api/client'
import UnloadPlaceModal from './UnloadPlaceModal.vue';
import CarHistoryModal from '../CarHistoryModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import AddToBlacklistModal from '@/components/admin/blacklist/AddToBlacklistModal.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { usePermissionsStore } from '@/stores/permissions';
import { useDeletionsStore } from '@/stores/deletions';
import { listVehicleBlacklist, createVehicleBlacklist } from '@/api/blacklist';
import { listMarks } from '@/api/marks';
import ExcelJS from 'exceljs';

export default {
    name: 'VehicleDetailsModal',
    components: {
        UnloadPlaceModal,
        CarHistoryModal,
        LoaderSpinner,
        AddToBlacklistModal
    },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        vehicle: {
            type: Object,
            default: null
        },
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        },
        licensePlateFormats: {
            type: Array,
            default: () => []
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        showCarFeatures: {
            type: Boolean,
            default: false
        },
        source: {
            type: String,
            default: 'general'
        },
        activeInfo: {
            type: Object,
            default: null
        },
        // Право подтвердить пропуск (POST override) - только ответственный по заявке.
        canOverride: {
            type: Boolean,
            default: false
        },
        // Право снять подтверждение (DELETE override) - ответственный или принимающий.
        canCancelOverride: {
            type: Boolean,
            default: false
        }
    },
    emits: ['close', 'open-application', 'override', 'cancel-override'],
    setup(_, { emit }) {
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
        return { onOverlayMousedown, onOverlayMouseup };
    },
    data() {
        return {
            selectedUnloadPlace: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            history: [],          // полная история (все действия)
            loadingHistory: false,
            isExporting: false,
            showCarHistoryModal: false,
            entryChecked: false,
            exitChecked: false,
            isBlacklisted: false,
            blacklistReason: '',
            showAddBlacklist: false,
            savingBlacklist: false,
            blacklistError: ''
        }
    },
    computed: {
        // Карточка, открытая ИЗ ApplicationDetail (source='application'), лежит ПОВЕРХ его
        // оверлея (z-index 10002). В остальных местах - базовый слой 10001, чтобы открытый
        // из карточки ApplicationDetail ("Открыть заявку") был выше карточки.
        overlayZIndex() {
            return this.source === 'application' ? 10003 : 10001;
        },
        // Намеренно НЕ зависит от showCarFeatures: на вкладке Автомобили features выкл,
        // но переход в заявку нужен. Гейт - наличие заявки (как у EmployeeDetailsModal).
        canOpenApplication() {
            return this.source !== 'application' && this.source !== 'blacklist'
                && !!(this.vehicle?.applicationId || this.vehicle?.application_id);
        },
        // Кнопки в шапке: "Полная история" видна при showCarFeatures,
        // "Открыть заявку" - при canOpenApplication.
        visibleActionsCount() {
            const history = this.showCarFeatures ? 1 : 0;
            const application = this.canOpenApplication ? 1 : 0;
            return history + application;
        },
        modalTitle() {
            const count = this.visibleActionsCount;
            if (count >= 2) return 'Информация';
            if (count === 1) return 'Детальная информация';
            return 'Детальная информация о Т/С';
        },
        getStatusClass() {
            if (this.entryChecked && !this.exitChecked) return 'status-on-territory';
            if (this.exitChecked) return 'status-exited';
            return 'status-not-entered';
        },
        getStatusText() {
            if (this.entryChecked && !this.exitChecked) return 'На территории';
            if (this.exitChecked) return 'Выехал';
            return 'Не въезжал';
        },
        // Статус территории нужен на вкладке "Автомобили" (source='carsview', features выкл)
        // наравне с Центром/деталью заявки. Скрываем только в корзине.
        showStatusSection() {
            return (this.showCarFeatures || this.source === 'carsview') && this.source !== 'trash';
        },
        // Только события въезда/выезда
        entryExitHistory() {
            return this.history.filter(item => item.action_type === 'entry' || item.action_type === 'exit');
        },
        canManageBlacklist() {
            return usePermissionsStore().hasPermission('page.admin.blacklist');
        },
        vehicleNumber() {
            return (this.vehicle?.plateNumber || this.vehicle?.car_number || '').trim();
        },
        vehicleMark() {
            return (this.vehicle?.mark || this.vehicle?.car_brand || '').trim();
        },
        vehicleLabel() {
            return [this.vehicleNumber, this.vehicleMark].filter(Boolean).join(' ');
        },
        // Добавить в ЧС можно только реальную машину с номером и маркой (не "по факту").
        hasVehicleIdentity() {
            const n = this.vehicleNumber.toLowerCase();
            return !!this.vehicleNumber && !!this.vehicleMark && n !== 'по факту';
        },
        // Предупреждение о возможном обходе ЧС показываем только в контексте заявки -
        // флаг приходит из деталей вложения (#481, срез C).
        blacklistSimilar() {
            return this.source === 'application' ? (this.vehicle?.blacklist_similar || null) : null;
        }
    },
    watch: {
        show: {
            immediate: true,
            handler(newVal) {
                if (newVal) {
                    this.loadCarStatus();
                    this.checkBlacklist();
                    if (this.showCarFeatures && this.vehicle?.id) {
                        this.loadCarHistory();
                    }
                } else {
                    this.closeUnloadPlaceDetails();
                    if (this.shiftTimer) {
                        clearTimeout(this.shiftTimer);
                        this.shiftTimer = null;
                    }
                    this.isMainShifted = false;
                    this.selectedUnloadPlace = null;
                }
            }
        },
        vehicle: {
            deep: true,
            handler(newVal) {
                if (newVal && this.show) {
                    this.checkBlacklist();
                    if (this.showCarFeatures) {
                        this.loadCarStatus();
                        this.loadCarHistory();
                    }
                }
            }
        }
    },
    beforeUnmount() {
        if (this.shiftTimer) {
            clearTimeout(this.shiftTimer);
        }
    },
    methods: {
        close() {
            this.$emit('close');
            this.closeUnloadPlaceDetails();
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.isMainShifted = false;
            this.selectedUnloadPlace = null;
            this.showAddBlacklist = false;
        },

        // Статус ЧС: матч активного списка по номеру+марке (зеркалит серверный CheckByName).
        async checkBlacklist() {
            this.isBlacklisted = false;
            this.blacklistReason = '';
            if (!this.hasVehicleIdentity) return;
            try {
                const list = await listVehicleBlacklist();
                const arr = Array.isArray(list) ? list : [];
                const key = (n, m) => `${(n || '').trim().toLowerCase()}|${(m || '').trim().toLowerCase()}`;
                const want = key(this.vehicleNumber, this.vehicleMark);
                const hit = arr.find((e) => key(e.car_number, e.mark_name) === want);
                if (hit) {
                    this.isBlacklisted = true;
                    this.blacklistReason = hit.reason || '';
                }
            } catch {
                // Молча: статус ЧС - вспомогательная плашка, не критична для карточки.
            }
        },

        openAddBlacklist() {
            this.blacklistError = '';
            this.showAddBlacklist = true;
        },

        closeAddBlacklist() {
            if (this.savingBlacklist) return;
            this.showAddBlacklist = false;
        },

        async submitAddBlacklist(reason) {
            if (this.savingBlacklist) return;
            this.savingBlacklist = true;
            this.blacklistError = '';
            try {
                // includeArchived: марка машины могла быть заархивирована, но как ключ
                // для создания записи ЧС она валидна (сервер резолвит mark_id без фильтра).
                const marks = await listMarks({ includeArchived: true });
                const arr = Array.isArray(marks) ? marks : [];
                const mark = arr.find((m) => (m.name || '').trim().toLowerCase() === this.vehicleMark.toLowerCase());
                if (!mark) {
                    this.blacklistError = `Марка "${this.vehicleMark}" не найдена в справочнике. Добавьте через раздел Чёрный список.`;
                    return;
                }
                await createVehicleBlacklist({ car_number: this.vehicleNumber, mark_id: mark.id, reason });
                this.isBlacklisted = true;
                this.blacklistReason = reason;
                this.showAddBlacklist = false;
                useDeletionsStore().notify({ prefix: 'Машина ', bold: this.vehicleLabel, suffix: ' добавлена в чёрный список' });
            } catch (e) {
                this.blacklistError = e?.message || 'Не удалось добавить в чёрный список';
            } finally {
                this.savingBlacklist = false;
            }
        },

        showUnloadPlaceDetails(placeId) {
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            if (!place) return;

            this.selectedUnloadPlace = place;

            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }

            this.isMainShifted = true;

            this.shiftTimer = setTimeout(() => {
                this.showPlaceModal = true;
                this.shiftTimer = null;
            }, 300);
        },

        closeUnloadPlaceDetails() {
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            this.isMainShifted = false;
            this.selectedUnloadPlace = null;
        },

        getPlaceName(placeId) {
            if (!this.allUnloadingPlaces || this.allUnloadingPlaces.length === 0) {
                return `ID: ${placeId}`;
            }
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            return place ? place.name : `ID: ${placeId}`;
        },

        getFormatName(formatId) {
            if (!formatId || !this.licensePlateFormats || this.licensePlateFormats.length === 0) {
                return null;
            }
            for (const format of this.licensePlateFormats) {
                if (format.format && format.format.id === formatId) {
                    return format.format.name;
                }
            }
            return null;
        },

        formatDate(dateString) {
            if (!dateString) return '';
            try {
                const [year, month, day] = dateString.split('-');
                const date = new Date(year, month - 1, day);
                return date.toLocaleDateString('ru-RU');
            } catch {
                return '';
            }
        },

        formatTime(timeString) {
            if (!timeString) return '';
            return timeString.substring(0, 5);
        },

        formatTimeRange(timeFrom, timeTo) {
            if (!timeFrom && !timeTo) return '-';
            
            const formatTime = (timeStr) => {
                if (!timeStr) return '';
                const parts = timeStr.split(':');
                if (parts.length >= 2) {
                    return `${parts[0]}:${parts[1]}`;
                }
                return timeStr;
            };

            const formattedTimeFrom = formatTime(timeFrom);
            const formattedTimeTo = formatTime(timeTo);
            
            if (!formattedTimeTo) return formattedTimeFrom;
            if (!formattedTimeFrom) return formattedTimeTo;
            return `${formattedTimeFrom} - ${formattedTimeTo}`;
        },

        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            }).replace(',', '');
        },

        getActionClass(item) {
            // Если пользователь не указан (системное действие), делаем точку фиолетовой
            if (!item.user_id) {
                return 'dot-system';
            }
            const classes = {
                'entry': 'dot-entry',
                'exit': 'dot-exit'
            };
            return classes[item.action_type] || 'dot-default';
        },

        getActionText(item) {
            if (item.action_type === 'entry') {
                return 'Отметил о прибытии';
            } else if (item.action_type === 'exit') {
                return 'Машина уехала';
            }
            return item.action_type; // на всякий случай
        },

        getActionComment(item) {
            const userName = item.user_name || 'Система';
            const carNumber = item.car_number || this.vehicle?.plateNumber || this.vehicle?.car_number || '';
            const carBrand = item.car_brand || this.vehicle?.mark || this.vehicle?.car_brand || '';
            
            if (item.action_type === 'entry') {
                return `Пользователь ${userName} отметил о прибытии автомобиля ${carNumber} ${carBrand} на территорию`;
            } else if (item.action_type === 'exit') {
                return `Пользователь ${userName} отметил об убытии автомобиля ${carNumber} ${carBrand} с территории`;
            }
            return item.comment || '';
        },

        async loadCarHistory() {
            if (!this.vehicle?.id || !this.showCarFeatures) return;
            
            this.loadingHistory = true;
            try {
                // Используем unified endpoint, как в CarHistoryModal.
                // Собираем query-строку руками — apiRequest ожидает строку-путь,
                // а не URL object (после /api префикса URL object ломается при конкатенации).
                const params = new URLSearchParams();
                params.append('car_number', this.vehicle.plateNumber || this.vehicle.car_number || '');
                params.append('car_brand', this.vehicle.mark || this.vehicle.car_brand || '');
                if (this.vehicle.organizationId) {
                    params.append('organization_id', this.vehicle.organizationId);
                }
                if (this.vehicle.companyId) {
                    params.append('company_id', this.vehicle.companyId);
                }

                const response = await apiRequest(`/cars/history/unified?${params.toString()}`, {});
                
                if (response.ok) {
                    const allHistory = await response.json();
                    this.history = allHistory; // сохраняем всё, но показываем только entry/exit через computed
                }
            } catch (error) {
                console.error('Ошибка сети при загрузке истории:', error);
            } finally {
                this.loadingHistory = false;
            }
        },

        async loadCarStatus() {
            // На вкладке "Автомобили" (carsview) vehicle.id - id реестра уникальных машин,
            // а статус ключуется по cars.id (заявочная таблица), поэтому берём только
            // activeCarId (без отката на vehicle.id - id-пространства разные, возможно ложное
            // совпадение). В прочих источниках vehicle.id уже = cars.id.
            const statusCarId = this.source === 'carsview' ? this.vehicle?.activeCarId : this.vehicle?.id;
            if (!statusCarId) return;
            try {
                const response = await apiRequest('/cars/history/current-status', {});
                if (response.ok) {
                    const statuses = await response.json();
                    const status = statuses.find(s => s.car_id === statusCarId);
                    if (status) {
                        this.entryChecked = status.territory_status === 1;
                        this.exitChecked = status.territory_status === 2;
                    } else {
                        this.entryChecked = false;
                        this.exitChecked = false;
                    }
                }
            } catch (error) {
                console.error('Ошибка при загрузке статуса:', error);
            }
        },

        async exportHistory() {
            const dataToExport = this.entryExitHistory;
            if (dataToExport.length === 0) return;
            
            this.isExporting = true;
            
            try {
                const workbook = new ExcelJS.Workbook();
                const worksheet = workbook.addWorksheet('Istoriya_viezdov');
                
                const headers = [
                    'Дата и время',
                    'Пользователь',
                    'Действие',
                    'Комментарий',
                    'Место'
                ];
                
                const headerRow = worksheet.addRow(headers);
                headerRow.height = 25;
                headerRow.eachCell((cell) => {
                    cell.fill = {
                        type: 'pattern',
                        pattern: 'solid',
                        fgColor: { argb: 'FF4F5BDF' }
                    };
                    cell.font = {
                        name: 'Verdana',
                        size: 11,
                        bold: true,
                        color: { argb: 'FFFFFFFF' }
                    };
                    cell.alignment = { vertical: 'middle', horizontal: 'center' };
                    cell.border = {
                        top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                        right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                    };
                });
                
                dataToExport.forEach((item, index) => {
                    const row = worksheet.addRow([
                        this.formatDateTime(item.created_at),
                        item.user_name || 'Система',
                        this.getActionText(item),
                        this.getActionComment(item),
                        item.table_name || ''
                    ]);
                    
                    row.height = 20;
                    const fillColor = index % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
                    
                    row.eachCell((cell) => {
                        cell.fill = {
                            type: 'pattern',
                            pattern: 'solid',
                            fgColor: { argb: fillColor }
                        };
                        cell.font = {
                            name: 'Verdana',
                            size: 9,
                            color: { argb: 'FF333333' }
                        };
                        cell.alignment = { vertical: 'middle' };
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });
                
                const lastDataRow = dataToExport.length;
                
                for (let row = 1; row <= lastDataRow + 1; row++) {
                    const rightCell = worksheet.getCell(row, 5);
                    rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
                    const leftCell = worksheet.getCell(row, 1);
                    leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
                }

                for (let col = 1; col <= 5; col++) {
                    const topCell = worksheet.getCell(1, col);
                    topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
                }

                for (let col = 1; col <= 5; col++) {
                    const bottomCell = worksheet.getCell(lastDataRow + 1, col);
                    bottomCell.border = { ...bottomCell.border, bottom: { style: 'medium', color: { argb: 'FF000000' } } };
                }
                
                worksheet.addRow([]);
                
                const infoRow1 = worksheet.addRow(['Отчёт сформировал:', this.currentUserName || 'Пользователь']);
                const infoRow2 = worksheet.addRow(['Дата формирования:', new Date().toLocaleString('ru-RU')]);
                
                [infoRow1, infoRow2].forEach(row => {
                    row.eachCell((cell) => {
                        cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
                        cell.alignment = { vertical: 'middle' };
                        cell.border = {
                            top: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            bottom: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            left: { style: 'thin', color: { argb: 'FFE6E6E6' } },
                            right: { style: 'thin', color: { argb: 'FFE6E6E6' } }
                        };
                    });
                });
                
                worksheet.columns = [
                    { width: 25 },
                    { width: 40 },
                    { width: 30 },
                    { width: 60 },
                    { width: 30 }
                ];
                
                const buffer = await workbook.xlsx.writeBuffer();
                const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                
                const carNumberSafe = (this.vehicle?.plateNumber || this.vehicle?.car_number || 'auto').replace(/[^a-zA-Z0-9]/g, '_');
                a.download = `Istoriya_viezdov_${carNumberSafe}_${new Date().toLocaleString().replace(/[.:,]/g, '-')}.xlsx`;
                a.href = url;
                a.click();
                window.URL.revokeObjectURL(url);
                
            } catch (error) {
                console.error('Error exporting to Excel:', error);
                useDeletionsStore().notify({ bold: 'Ошибка при экспорте в Excel', type: 'error' });
            } finally {
                this.isExporting = false;
            }
        },

        openCarHistory() {
            this.showCarHistoryModal = true;
        },

        openApplication() {
            const applicationId = this.vehicle?.applicationId || this.vehicle?.application_id;
            if (applicationId) {
                this.$emit('open-application', applicationId);
            }
        }
    }
}
</script>

<style scoped>
/* Все предыдущие стили остаются, добавляем новый класс для системной точки */
.dot-system {
    background: #8b5cf6; /* фиолетовый */
}

.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }
.dot-default { background: #9ca3af; }

.place-name {
    font-size: 10px;
    color: #4F5BDF;
    margin-top: 2px;
    font-style: italic;
}

/* Остальные стили без изменений */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10001;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.4s ease-out;
}

@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
  }
  to {
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(0.1px);
  }
}

.modal-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-content {
  background: #fff;
  border-radius: 50px;
  padding: 0;
  padding-bottom: 15px;
  width: 550px;
  height: 450px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  position: absolute;
}

.modal-content .modal-body {
  overflow-y: auto;
  height: calc(450px - 70px);
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.modal-content .modal-body::-webkit-scrollbar {
  display: none;
}

.modal-content.main-modal {
  left: calc(50% - 260px);
  transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
  transform: translateX(0);
}

.modal-content.main-modal.shifted {
  transform: translateX(-280px);
}

.place-modal-container {
  position: absolute;
  left: 50%;
  width: 520px;
  height: 450px;
  pointer-events: auto;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 30px 16px;
  border-bottom: 1px solid #f0f0f0;
  flex-shrink: 0;
  height: 70px;
  box-sizing: border-box;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-right: 10px;
}

.history-btn, .application-btn {
  padding: 6px 12px;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  font-size: 12px;
  color: #333;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.history-btn:hover, .application-btn:hover {
  background: #f5f5f5;
  border-color: #4F5BDF;
}

.blacklist-add-btn {
  padding: 6px 12px;
  background: white;
  border: 1px solid #fecaca;
  border-radius: 20px;
  font-size: 12px;
  color: #dc2626;
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
  white-space: nowrap;
}

.blacklist-add-btn:hover {
  background: #fee2e2;
  border-color: #dc2626;
}

.bl-section {
  margin-bottom: 16px;
  padding: 12px 16px;
  background: #fdeaea;
  border: 1px solid #f5b5b5;
  border-radius: 20px;
}

.bl-section-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #b91c1c;
}

.bl-section-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dc2626;
  flex-shrink: 0;
}

.bl-section-reason {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: #7f1d1d;
  word-break: break-word;
}

.bl-section-label {
  font-weight: 600;
}

/* Блок "Подозрение на обход ЧС" (#481, срез C). По умолчанию красный (требует решения),
   после подтверждения пропуска - зелёный "решено" (.is-resolved). */
.bl-suspicion-section {
  margin-bottom: 16px;
  padding: 12px 16px;
  background: #fdeaea;
  border: 1px solid #f5b5b5;
  border-radius: 20px;
}

.bl-suspicion-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #b91c1c;
}

.bl-suspicion-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dc2626;
  flex-shrink: 0;
}

.bl-suspicion-row {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: #7f1d1d;
  word-break: break-word;
}

.bl-suspicion-label {
  font-weight: 600;
}

.bl-suspicion-foot {
  margin-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.bl-suspicion-blocked {
  font-size: 12px;
  color: #7f1d1d;
}

.bl-suspicion-confirmed {
  font-size: 13px;
  font-weight: 600;
  color: #047857;
}

.bl-suspicion-btn {
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.bl-suspicion-btn--allow {
  background: #fff;
  border: 1px solid #fecaca;
  color: #dc2626;
}

.bl-suspicion-btn--allow:hover {
  background: #fee2e2;
  border-color: #dc2626;
}

.bl-suspicion-btn--cancel {
  background: #fff;
  border: 1px solid #cbd5e1;
  color: #475569;
}

.bl-suspicion-btn--cancel:hover {
  background: #f1f5f9;
  border-color: #94a3b8;
}

.bl-suspicion-section.is-resolved {
  background: #ecfdf5;
  border-color: #a7f3d0;
}

.bl-suspicion-section.is-resolved .bl-suspicion-head {
  color: #047857;
}

.bl-suspicion-section.is-resolved .bl-suspicion-dot {
  background: #10b981;
}

.bl-suspicion-section.is-resolved .bl-suspicion-row {
  color: #065f46;
}

.modal-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1a1a1a;
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.modal-close:hover {
  background-color: #f5f5f5;
}

.modal-body {   
  padding: 20px 30px;
  overflow-y: auto;
  flex: 1;
}

.active-warning-section {
  margin-bottom: 15px;
  padding: 15px;
  background: #fff3cd;
  border: 1px solid #ffeeba;
  border-radius: 20px;
}

.warning-content {
  display: flex;
  gap: 15px;
  align-items: flex-start;
}

.warning-text {
  flex: 1;
}

.warning-title {
  font-weight: 600;
  color: #856404;
  margin: 0 0 5px 0;
  font-size: 14px;
}

.warning-details {
  color: #856404;
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}

.vehicle-details {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.details-section {
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  background: #fafafa;
  overflow: hidden;
}

.section-header {
  padding: 12px 20px;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #333;
}

.section-body {
  padding: 16px 20px;
}

.details-grid.two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 16px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-item.full-width {
  grid-column: 1 / -1;
}

.detail-label {
  font-size: 11px;
  color: #a2a2a2;
  font-weight: 400;
  letter-spacing: 0.3px;
}

.detail-value {
  font-size: 14px;
  color: #333;
  font-weight: 500;
  word-break: break-word;
}

.places-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.place-item {
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  padding: 6px 12px;
  font-size: 12px;
  color: #333;
  transition: all 0.2s ease;
  display: inline-block;
  cursor: pointer;
}

.place-item:hover {
  background: #f0f0f0;
  border-color: #4F5BDF;
}

.place-item.active {
  background: #4F5BDF;
  color: white;
  border-color: #4F5BDF;
}

.no-places {
  text-align: center;
  color: #a2a2a2;
  font-size: 13px;
  font-style: italic;
  padding: 10px;
}

.status-badge {
  display: inline-block;
  padding: 6px 16px;
  border-radius: 50px;
  font-size: 13px;
  font-weight: 500;
}

.status-on-territory {
  background: rgba(79, 91, 223, 0.1);
  color: #4F5BDF;
  border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-exited {
  background: rgba(220, 38, 38, 0.1);
  color: #dc2626;
  border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-not-entered {
  background: #f5f5f5;
  color: #9ca3af;
  border: 1px solid #e6e6e6;
}

.export-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  font-size: 12px;
  color: #333;
  cursor: pointer;
  transition: all 0.2s ease;
  height: 28px;
}

.export-btn:hover:not(:disabled) {
  background: #f5f5f5;
  border-color: #4F5BDF;
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.export-icon {
  width: 12px;
  height: 12px;
}

.export-loader {
  width: 14px;
  height: 14px;
  border: 2px solid #e6e6e6;
  border-top: 2px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.history-timeline {
  position: relative;
  max-height: 200px;
  overflow-y: auto;
}

.history-item {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  position: relative;
}

.history-item:last-child {
  margin-bottom: 0;
}

.timeline-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 4px;
  z-index: 1;
}

.timeline-line {
  position: absolute;
  left: 3px;
  top: 16px;
  width: 2px;
  height: calc(100% + 2px);
  background: #e6e6e6;
}

.history-content {
  flex: 1;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 2px;
}

.user-name {
  font-weight: 500;
  color: #333;
  font-size: 12px;
}

.action-time {
  color: #a2a2a2;
  font-size: 10px;
}

.action-text {
  color: #666;
  font-size: 11px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 10px;
  color: #666;
  font-style: italic;
  margin-top: 2px;
  padding-left: 6px;
  border-left: 2px solid #e6e6e6;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 20px;
  gap: 10px;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.no-history {
  text-align: center;
  color: #a2a2a2;
  padding: 20px;
  font-size: 13px;
  font-style: italic;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
  transition: all 0.4s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
  transition: all 0.4s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
  background: rgba(0, 0, 0, 0);
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.9) translateY(-20px);
}

.place-slide-enter-active,
.place-slide-leave-active {
  transition: transform 0.6s cubic-bezier(0.2, 0.9, 0.1, 1), opacity 0.5s ease;
}
.place-slide-enter-from {
  transform: translateY(100vh);
  opacity: 0;
}
.place-slide-enter-to {
  transform: translateY(0);
  opacity: 1;
}
.place-slide-leave-from {
  transform: translateY(0);
  opacity: 1;
}
.place-slide-leave-to {
  transform: translateY(600px);
  opacity: 0;
}

@media (max-width: 768px) {
  .modal-content {
    width: 90%;
    left: 5% !important;
    transform: none !important;
    height: auto;
    max-height: 80vh;
  }
  
  .modal-content .modal-body {
    height: auto;
    max-height: calc(80vh - 70px);
  }
  
  .modal-content.main-modal.shifted {
    transform: none !important;
  }
  
  .place-modal-container {
    width: 90%;
    left: 5%;
    height: auto;
    max-height: 80vh;
  }
  
  .modal-header {
    padding: 16px 20px;
    flex-wrap: wrap;
    height: auto;
  }
  
  .header-actions {
    order: 3;
    width: 100%;
    justify-content: flex-start;
    margin-top: 10px;
  }
  
  .modal-body {
    padding: 16px 20px;
  }
  
  .details-section {
    border-radius: 16px;
  }
  
  .section-header {
    padding: 10px 16px;
  }
  
  .section-body {
    padding: 14px 16px;
  }
  
  .section-title {
    font-size: 13px;
  }
  
  .details-grid.two-columns {
    grid-template-columns: 1fr;
  }
  
  .places-list {
    gap: 6px;
  }
  
  .place-item {
    padding: 4px 10px;
    font-size: 11px;
  }
}
</style>