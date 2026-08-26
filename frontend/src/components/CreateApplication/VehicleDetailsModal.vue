<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        :style="{ zIndex: overlayZIndex }"
      >
        <!-- Обёртка занимает всю площадь затемнения, поэтому закрытие по клику мимо
             окна висит на ней, а не на самом затемнении: до него событие не доходит.
             Держать обработчики на обоих нельзя - состояние «клик начался на фоне»
             у них общее, и всплывшее с обёртки событие его сбрасывало. -->
        <div
          class="modal-wrapper"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <!-- Основное модальное окно с деталями ТС -->
          <div
            class="modal-content compact-modal main-modal"
            :class="{ 'shifted': isMainShifted, 'is-dragging': sheetDragging }"
            :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
            @mousedown.stop
            @touchstart="onSheetTouchStart"
            @touchmove="onSheetTouchMove"
            @touchend="onSheetTouchEnd"
          >
            <div
              class="sheet-handle"
              aria-hidden="true"
            />
            <div class="modal-header">
              <h3 class="modal-title">
                {{ modalTitle }}
              </h3>
              <div
                v-if="showCarFeatures || (!readonly && canManageBlacklist && hasVehicleIdentity)"
                class="header-actions"
              >
                <button
                  v-if="showCarFeatures && canSeeFullHistory"
                  class="history-btn"
                  @click="openCarHistory"
                >
                  <span>Полная история</span>
                </button>
                <button
                  v-if="!readonly && canOpenApplication"
                  class="application-btn"
                  @click="openApplication"
                >
                  <span>Открыть заявку</span>
                </button>
                <button
                  v-if="!readonly && canManageBlacklist && hasVehicleIdentity && !isBlacklisted"
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
                    
            <div
              ref="sheetBody"
              class="modal-body"
            >
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
                      Заявка {{ activeInfo.application_number }}<br>
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
                    <!-- За кем закреплена запись реестра: служебная пометка бюро, поэтому
                         подписью под блоком, а не строкой наравне с данными машины.
                         Сервер отдаёт её только администратору, см. EmployeeDetailsModal. -->
                    <p
                      v-if="vehicle.user_name"
                      class="owner-note"
                      data-testid="vehicle-owner-login"
                    >
                      Запись закреплена за: {{ vehicle.user_name }}
                    </p>
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

                <!-- Секция Проезд (таблицы) -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Проезд
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="places-list">
                      <div
                        v-for="t in passageActiveTables"
                        :key="t.id"
                        class="place-item"
                        :class="{ 'active': showTableModal && selectedTable && selectedTable.table && selectedTable.table.id === t.id }"
                        @click="showTableDetails(t.id)"
                      >
                        {{ t.name }}
                        <Badge
                          v-if="t.source"
                          :label="t.source === 'manual' ? 'добавлено' : 'из заявки'"
                          :variant="t.source === 'manual' ? 'neutral' : 'primary'"
                          size="sm"
                        />
                      </div>
                      <div
                        v-for="t in passageRemovedTables"
                        :key="'removed-' + t.id"
                        class="place-item place-item--removed"
                      >
                        {{ t.name }}
                      </div>
                      <div
                        v-if="passageActiveTables.length === 0 && passageRemovedTables.length === 0"
                        class="no-places"
                      >
                        Проезд не указан
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
                  v-if="showCarFeatures && canSeeEntryExit"
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

                          <!-- Данные, введённые охранником при пропуске "по факту" (#1132) -->
                          <div
                            v-if="passInfo(item)"
                            class="pass-data"
                          >
                            <div class="pass-data__row">
                              <span class="pass-data__key">Номер:</span> {{ passInfo(item).number }}
                            </div>
                            <div
                              v-if="passInfo(item).mark_name"
                              class="pass-data__row"
                            >
                              <span class="pass-data__key">Марка:</span> {{ passInfo(item).mark_name }}
                            </div>
                            <div
                              v-if="passInfo(item).format_name"
                              class="pass-data__row"
                            >
                              <span class="pass-data__key">Формат:</span> {{ passInfo(item).format_name }}
                            </div>
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

          <!-- Дополнительное модальное окно с деталями таблицы «Проезд» (#1036) -->
          <transition
            name="place-slide"
            @after-leave="onTableLeave"
          >
            <div
              v-if="showTableModal"
              class="place-modal-container"
            >
              <TableInfoModal
                :table="selectedTable"
                :all-tables="allTables"
                @close="closeTableDetails"
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
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { apiRequest } from '@/api/client'
import UnloadPlaceModal from './UnloadPlaceModal.vue';
import TableInfoModal from './TableInfoModal.vue';
import CarHistoryModal from '../CarHistoryModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import Badge from '@/components/ui/Badge.vue';
import AddToBlacklistModal from '@/components/admin/blacklist/AddToBlacklistModal.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useEscapeClose } from '@/composables/useEscapeClose';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { usePermissionsStore } from '@/stores/permissions';
import { getModalActionPermission } from '@/constants/detailModalActions';
import { useDeletionsStore } from '@/stores/deletions';
import { checkVehicleBlacklist, createVehicleBlacklist } from '@/api/blacklist';
import { listMarks } from '@/api/marks';
import ExcelJS from 'exceljs';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'VehicleDetailsModal',
    components: {
        AppIcon,
        UnloadPlaceModal,
        TableInfoModal,
        CarHistoryModal,
        LoaderSpinner,
        Badge,
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
        allTables: {
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
        },
        // Режим просмотра (список заявки): прячет кнопки действий (ЧС, открыть заявку).
        readonly: {
            type: Boolean,
            default: false
        }
    },
    emits: ['close', 'open-application', 'override', 'cancel-override'],
    setup(props, { emit }) {
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
        // Слой карточки: из заявки она лежит поверх её панели (10003), иначе 10001 -
// то же значение, что у оверлея, чтобы Escape закрывал именно верхнее окно.
useEscapeClose(() => emit('close'), () => props.show, props.source === 'application' ? 10003 : 10001);
        // Bottom-sheet свайп-вниз-закрытие на мобилке (#1097 r2). getScrollTop от тела:
        // свайп из контента закрывает, только когда прокручено вверх; с ползунка - всегда.
        const sheetBody = ref(null);
        const swipe = useSwipeDismiss(() => emit('close'), {
            getScrollTop: () => sheetBody.value?.scrollTop ?? 0,
            handleSelector: '.sheet-handle',
        });
        const { isNarrow } = useNarrowScreen();
        return {
            onOverlayMousedown, onOverlayMouseup,
            isNarrow,
            sheetBody,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        };
    },
    data() {
        return {
            selectedUnloadPlace: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            // Drill-down таблиц «Проезд» (#1036): отдельный набор состояний от места
            // разгрузки - общий слот place-modal-container один, поэтому подмодалки
            // взаимоисключаются (открытие одной закрывает другую).
            selectedTable: null,
            showTableModal: false,
            tableShiftTimer: null,
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
        // но переход в заявку нужен. Гейт: право detail.open_application по контексту
        // (карта detailModalActions, как у EmployeeDetailsModal) И наличие заявки.
        canOpenApplication() {
            const perm = getModalActionPermission('vehicle', this.source, 'openApplication');
            const allowed = typeof perm === 'boolean'
                ? perm
                : usePermissionsStore().hasPermission(perm);
            return allowed && !!(this.vehicle?.applicationId || this.vehicle?.application_id);
        },
        // Право на кнопку «Полная история» / секцию «История въездов и выездов»:
        // гейтим ТОЛЬКО когда карта задаёт ключ права (string); контекстные false/true
        // оставляем на showCarFeatures — нулевая регрессия дефолтной видимости,
        // добавляется лишь возможность отозвать ролью.
        canSeeFullHistory() {
            const v = getModalActionPermission('vehicle', this.source, 'history');
            return typeof v === 'string' ? usePermissionsStore().hasPermission(v) : true;
        },
        canSeeEntryExit() {
            const v = getModalActionPermission('vehicle', this.source, 'entryExit');
            return typeof v === 'string' ? usePermissionsStore().hasPermission(v) : true;
        },
        // Кнопки в шапке: "Полная история" видна при showCarFeatures + право,
        // "Открыть заявку" - при canOpenApplication.
        visibleActionsCount() {
            const history = (this.showCarFeatures && this.canSeeFullHistory) ? 1 : 0;
            const application = this.canOpenApplication ? 1 : 0;
            return history + application;
        },
        // На телефоне в строку шапки помещается только короткое имя: длинный вариант
        // отжимал крестик и переносился на вторую строку рядом с кнопками действий.
        modalTitle() {
            if (this.isNarrow) return 'Информация';
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
        // Бейдж источника и зачёркнутые снятые - фича карточки ИЗ ПРОХОДНОЙ (#1227): только там
        // target_tables несут реальный source (объекты {id,name,source} от P1/P2) и история привязок
        // осмысленна. В заявке (source='application') / списках / корзине target_tables - плоские ID
        // БЕЗ source -> бейджа нет (иначе один элемент в проходной «добавлено», а в заявке дефолтно
        // «из заявки» - каша). Признак проходной = у активных есть реальный source.
        hasPassageSource() {
            return this.passageActiveTables.some(t => t.source);
        },
        // Активные привязки «Проезд» (#1227 P3). Нормализует ОБЕ формы target_tables: контекст
        // заявки - плоский массив ID (число, source=null -> без бейджа), проходной - объекты
        // {id,name,source}. source НЕ фабрикуем - null значит «источник неизвестен, не показывать».
        passageActiveTables() {
            const raw = this.vehicle?.target_tables || [];
            return raw.map(t => (typeof t === 'number'
                ? { id: t, name: this.getTableName(t), source: null }
                : { id: t.id, name: t.name || this.getTableName(t.id), source: t.source || null }));
        },
        // Снятые/перенесённые таблицы (unbound_from_table/moved_between_tables из истории) -
        // показываем зачёркнутыми, кроме тех, что сейчас снова активны (активная привязка
        // перекрывает снятую). Дедуп по table_id - несколько снятий одной таблицы не дублируем.
        passageRemovedTables() {
            // Зачёркнутые снятые - только когда карточка показывает реальные привязки проходной
            // (hasPassageSource); в заявке/списках/корзине история привязок машины не показывается.
            if (!this.hasPassageSource) return [];
            const activeIds = new Set(this.passageActiveTables.map(t => t.id));
            const seen = new Set();
            const removed = [];
            // history не гейтится правом (в отличие от entryExitHistory) - рендерится
            // всегда, поэтому защищаемся от неожиданной формы ответа (не массив).
            const items = Array.isArray(this.history) ? this.history : [];
            items.forEach(item => {
                if (item.action_type !== 'unbound_from_table' && item.action_type !== 'moved_between_tables') return;
                const tableId = item.table_id;
                if (tableId == null || activeIds.has(tableId) || seen.has(tableId)) return;
                seen.add(tableId);
                removed.push({ id: tableId, name: item.table_name || this.getTableName(tableId) });
            });
            return removed;
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
                // Контракт окна: фон под листом не прокручивается.
                setBodyScrollLock(this, newVal);
                if (newVal) {
                    this.loadCarStatus();
                    this.checkBlacklist();
                    if (this.showCarFeatures && this.vehicle?.id) {
                        this.loadCarHistory();
                    }
                } else {
                    this.closeUnloadPlaceDetails();
                    this.closeTableDetails();
                    if (this.shiftTimer) {
                        clearTimeout(this.shiftTimer);
                        this.shiftTimer = null;
                    }
                    this.isMainShifted = false;
                    this.selectedUnloadPlace = null;
                    this.selectedTable = null;
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
        releaseBodyScrollLock(this);
        if (this.shiftTimer) {
            clearTimeout(this.shiftTimer);
        }
        if (this.tableShiftTimer) {
            clearTimeout(this.tableShiftTimer);
        }
    },
    methods: {
        close() {
            this.$emit('close');
            this.closeUnloadPlaceDetails();
            this.closeTableDetails();
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.isMainShifted = false;
            this.selectedUnloadPlace = null;
            this.selectedTable = null;
            this.showAddBlacklist = false;
        },

        // Статус ЧС: точечная серверная проверка по номеру+марке (весь список ЧС в
        // браузер больше не грузим). mark_id резолвим через открытый справочник марок.
        async checkBlacklist() {
            this.isBlacklisted = false;
            this.blacklistReason = '';
            if (!this.hasVehicleIdentity) return;
            try {
                const marks = await listMarks({ includeArchived: true });
                const arr = Array.isArray(marks) ? marks : [];
                const mark = arr.find((m) => (m.name || '').trim().toLowerCase() === this.vehicleMark.trim().toLowerCase());
                if (!mark) return; // марки нет в справочнике - точного совпадения по ЧС не будет
                const res = await checkVehicleBlacklist({ car_number: this.vehicleNumber, mark_id: mark.id });
                if (res && res.is_blacklisted) {
                    this.isBlacklisted = true;
                    this.blacklistReason = res.reason || '';
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

            // Взаимоисключение с drill-down таблицы: слот place-modal-container один,
            // поэтому гасим подмодалку таблицы, если открыта.
            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }
            this.showTableModal = false;
            this.selectedTable = null;

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

        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (!tableData) {
                useDeletionsStore().notify({ bold: 'Информация о месте проезда недоступна', type: 'error' });
                return;
            }

            // Взаимоисключение с drill-down места разгрузки (общий слот).
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.showPlaceModal = false;
            this.selectedUnloadPlace = null;

            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || [],
                photos: tableData.photos || [],
                current_status: tableData.current_status || 'closed'
            };

            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }

            this.isMainShifted = true;

            this.tableShiftTimer = setTimeout(() => {
                this.showTableModal = true;
                this.tableShiftTimer = null;
            }, 300);
        },

        closeTableDetails() {
            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }
            this.showTableModal = false;
        },

        onTableLeave() {
            this.isMainShifted = false;
            this.selectedTable = null;
        },

        getPlaceName(placeId) {
            if (!this.allUnloadingPlaces || this.allUnloadingPlaces.length === 0) {
                return `ID: ${placeId}`;
            }
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            return place ? place.name : `ID: ${placeId}`;
        },

        getTableName(tableId) {
            let found = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (found) {
                let tbl = found.table || found;
                return tbl.display_name || tbl.name || `ID: ${tableId}`;
            }
            return `Неизвестное место (ID: ${tableId})`;
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

        // Данные пропуска "по факту" (#1132): охранник вводит их при въезде, бэкенд
        // кладёт в details.metadata записи entry. Возвращаем объект {number, mark_name,
        // format_name} или null, если это обычная запись без данных пропуска.
        passInfo(item) {
            const m = item && item.metadata;
            if (m && typeof m === 'object' && m.number) return m;
            return null;
        },

        async loadCarHistory() {
            if (!this.vehicle?.id || !this.showCarFeatures) return;
            
            this.loadingHistory = true;
            try {
                // Фактовая таблица (#1132): у машин "по факту" car_number - плейсхолдер,
                // и unified (по номеру+марке) склеил бы истории всех таких машин. Берём
                // историю ОДНОЙ машины, чтобы данные пропуска (metadata) относились к ней.
                if (this.source === 'facttable') {
                    const response = await apiRequest(`/cars/${this.vehicle.id}/history`, {});
                    if (response.ok) {
                        this.history = await response.json();
                    }
                    return;
                }
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
    color: var(--accent-text);
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
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10001;
  animation: overlayAppear 0.4s ease-out;
}

@keyframes overlayAppear {
  from {
    background: var(--overlay);
    backdrop-filter: blur(0px);
  }
  to {
    background: var(--overlay);
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
  background: var(--surface);
  border-radius: 50px;
  padding: 0;
  padding-bottom: 15px;
  width: 550px;
  height: 450px;
  box-shadow: 0 20px 60px var(--shadow-drop);
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
  left: calc(50% - 275px);
  transition: transform 0.5s cubic-bezier(0.25, 0.1, 0.15, 1);
  transform: translateX(0);
}

.modal-content.main-modal.shifted {
  transform: translateX(-295px);
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
  border-bottom: 1px solid var(--border);
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
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.history-btn:hover, .application-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}

.blacklist-add-btn {
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: 20px;
  font-size: 12px;
  color: var(--danger-text);
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
  white-space: nowrap;
}

.blacklist-add-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.bl-section {
  margin-bottom: 16px;
  padding: 12px 16px;
  background: var(--danger-bg);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: 20px;
}

.bl-section-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--danger-text);
}

.bl-section-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--danger);
  flex-shrink: 0;
}

.bl-section-reason {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--danger-text);
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
  background: var(--danger-bg);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: 20px;
}

.bl-suspicion-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--danger-text);
}

.bl-suspicion-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--danger);
  flex-shrink: 0;
}

.bl-suspicion-row {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--danger-text);
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
  color: var(--danger-text);
}

.bl-suspicion-confirmed {
  font-size: 13px;
  font-weight: 600;
  color: var(--success-text);
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
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  color: var(--danger-text);
}

.bl-suspicion-btn--allow:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.bl-suspicion-btn--cancel {
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  color: var(--text-muted);
}

.bl-suspicion-btn--cancel:hover {
  background: var(--accent-tint);
  border-color: var(--text-muted);
}

.bl-suspicion-section.is-resolved {
  background: var(--success-bg);
  border-color: color-mix(in srgb, var(--success) 30%, var(--surface));
}

.bl-suspicion-section.is-resolved .bl-suspicion-head {
  color: var(--success-text);
}

.bl-suspicion-section.is-resolved .bl-suspicion-dot {
  background: var(--success);
}

.bl-suspicion-section.is-resolved .bl-suspicion-row {
  color: var(--success-text);
}

.modal-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
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
  background-color: var(--surface-2);
}

.modal-body {   
  padding: 20px 30px;
  overflow-y: auto;
  flex: 1;
}

.active-warning-section {
  margin-bottom: 15px;
  padding: 15px;
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
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
  color: var(--warning-text);
  margin: 0 0 5px 0;
  font-size: 14px;
}

.warning-details {
  color: var(--warning-text);
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
  border: 1px solid var(--border);
  border-radius: 20px;
  background: var(--surface-2);
  overflow: hidden;
}

.section-header {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
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

.owner-note {
  margin: 10px 0 0;
  font-size: 11px;
  color: var(--text-muted);
  opacity: 0.75;
}

.detail-label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 400;
  letter-spacing: 0.3px;
}

.detail-value {
  font-size: 14px;
  color: var(--text);
  font-weight: 500;
  word-break: break-word;
}

.places-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.place-item {
  border: 1px solid var(--border);
  border-radius: 50px;
  padding: 6px 12px;
  font-size: 12px;
  color: var(--text);
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.place-item:hover {
  background: var(--border);
  border-color: var(--accent);
}

.place-item.active {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

/* Снятая/перенесённая таблица (#1227 P3): не кликабельна, зачёркнута как .old-status
   в ApplicationHistory.vue - "проезд был, но сейчас не действует". */
.place-item--removed {
  cursor: default;
  color: var(--danger-text);
  text-decoration: line-through;
  border-style: dashed;
}

.place-item--removed:hover {
  background: transparent;
  border-color: var(--border);
}

.no-places {
  text-align: center;
  color: var(--text-muted);
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
  background: color-mix(in srgb, var(--accent) 10%, var(--surface));
  color: var(--accent-text);
  border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-exited {
  background: color-mix(in srgb, var(--danger) 10%, var(--surface));
  color: var(--danger-text);
  border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-not-entered {
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.export-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  height: 28px;
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
  width: 12px;
  height: 12px;
}

.export-loader {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border);
  border-top: 2px solid var(--accent);
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
  background: var(--border);
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
  color: var(--text);
  font-size: 12px;
}

.action-time {
  color: var(--text-muted);
  font-size: 10px;
}

.action-text {
  color: var(--text-muted);
  font-size: 11px;
  margin-bottom: 2px;
}

.action-comment {
  font-size: 10px;
  color: var(--text-muted);
  font-style: italic;
  margin-top: 2px;
  padding-left: 6px;
  border-left: 2px solid var(--border);
}

/* Данные пропуска "по факту" (#1132) под записью въезда */
.pass-data {
  margin-top: 4px;
  padding: 6px 8px;
  background: var(--accent-tint);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pass-data__row {
  font-size: 11px;
  color: var(--text);
}

.pass-data__key {
  font-weight: 600;
  color: var(--text);
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 20px;
  gap: 10px;
  /* Резерв под контент истории: без него spinner крошечный, а при загрузке списка
     секция резко растёт - модалка прыгает (#1097 R3-6). Держим высоту стабильной. */
  min-height: 120px;
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

.no-history {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-muted);
  padding: 20px;
  font-size: 13px;
  font-style: italic;
  /* Та же высота, что у загрузки/короткого списка - пустое состояние не прыгает. */
  min-height: 120px;
}

/* Появление и скрытие - как у остальных окон (BaseModal): затемнение гаснет
   прозрачностью, само окно приезжает масштабом. Прежние правила задавали переход
   корню перехода (.modal-overlay), а не окну внутри него, поэтому затемнение
   плавно гасло, а окно прыгало. */
.modal-fade-enter-active {
  transition: opacity 0.3s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-content {
  animation: details-modal-in 0.3s ease;
}

.modal-fade-leave-active .modal-content {
  animation: details-modal-out 0.2s ease;
}

@keyframes details-modal-in {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes details-modal-out {
  from {
    opacity: 1;
    transform: scale(1);
  }
  to {
    opacity: 0;
    transform: scale(0.95);
  }
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

/* Ползунок скрыт по умолчанию (десктоп), показывается только в bottom-sheet @768. */
.sheet-handle {
  display: none;
}

@media (max-width: 768px) {
  /* Bottom-sheet: wrapper центрировал контент (align-items:center + height:100%) и
     побеждал flex-end оверлея из App.vue - выравниваем к низу, модалка выезжает
     снизу (detail 4). Ширина/скругление приходят из App.vue (.modal-content). */
  .modal-wrapper {
    align-items: flex-end;
  }
  .modal-content {
    width: 90%;
    left: 0 !important;
    height: auto;
    max-height: 80dvh;
    /* transition для свайп-спринга и слайда; НЕ transform:none!important - иначе
       блокировался бы inline-transform свайпа (#1097 r2). */
    transition: transform 0.3s ease;
  }

  .modal-content.is-dragging {
    transition: none;
  }

  /* Ползунок bottom-sheet (тянуть вниз для закрытия). */
  .sheet-handle {
    display: block;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 8px auto 0;
    flex-shrink: 0;
  }

  /* Enter/leave = слайд снизу (перебивает базовый scale(.9)translateY(-20px) -
     раньше модалка "спавнилась" поп-ом из центра, а не выезжала снизу). */
  .modal-fade-enter-from .modal-content,
  .modal-fade-leave-to .modal-content {
    transform: translateY(100%);
    opacity: 1;
  }
  
  .modal-content .modal-body {
    height: auto;
    max-height: calc(80dvh - 70px);
  }
  
  .modal-content.main-modal.shifted {
    transform: none !important;
  }
  
  /* Bottom-sheet: инфо места прохода/разгрузки выезжает снизу во всю ширину поверх
     детали Т/С (place-slide уже слайдит с translateY(100vh), #1097 R4-10). */
  .place-modal-container {
    left: 0;
    right: 0;
    width: 100%;
    bottom: 0;
    top: auto;
    height: auto;
    max-height: 90dvh;
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