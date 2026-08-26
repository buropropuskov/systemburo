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
        <!-- Закрытие по клику мимо окна висит на самом затемнении, а не на обёртке:
             у обёртки pointer-events: none, она целью события не становится вовсе, и
             прежний @click.self на ней не срабатывал никогда. Через useOverlayClose,
             чтобы выделение текста внутри окна, отпущенное на фоне, его не закрывало. -->
        <div class="modal-wrapper">
          <!-- Основное модальное окно с деталями сотрудника -->
          <div
            class="modal-content compact-modal main-modal"
            :class="{ 'shifted': isMainShifted, 'is-dragging': sheetDragging }"
            :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
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
              <div class="header-actions">
                <button
                  v-if="showHistoryButton"
                  class="history-btn"
                  @click="openFullHistory"
                >
                  <span>Полная история</span>
                </button>
                <button
                  v-if="!readonly && source !== 'application' && employee?.applicationId"
                  class="application-btn"
                  @click="openApplication"
                >
                  <span>Открыть заявку</span>
                </button>
                <button
                  v-if="!readonly && canManageBlacklist && hasPersonIdentity && !isBlacklisted"
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
                  Человек в чёрном списке
                </div>
                <div
                  v-if="blacklistReason"
                  class="bl-section-reason"
                >
                  <span class="bl-section-label">Причина:</span> {{ blacklistReason }}
                </div>
              </div>

              <!-- Подозрение на обход ЧС (#481, срез C): см. VehicleDetailsModal. -->
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

              <div
                v-if="employee"
                class="employee-details"
              >
                <!-- Секция Удаление (только корзина) -->
                <div
                  v-if="source === 'trash' && (employee.deletedByName || employee.deletedAtText)"
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
                        <span class="detail-value">{{ employee.deletedAtText || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Кто удалил:</span>
                        <span class="detail-value">{{ employee.deletedByName || '-' }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Основная информация -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Основная информация
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Фамилия:</span>
                        <span class="detail-value">{{ employee.last_name || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Имя:</span>
                        <span class="detail-value">{{ employee.first_name || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Отчество:</span>
                        <span class="detail-value">{{ employee.middle_name || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Должность:</span>
                        <span class="detail-value">{{ employee.position || 'Не указана' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Гражданство:</span>
                        <span class="detail-value">{{ employee.citizenshipName || 'Не указано' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Организация:</span>
                        <span class="detail-value">{{ employee.organization || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Компания:</span>
                        <span class="detail-value">{{ employee.company || '-' }}</span>
                      </div>
                      <!-- За кем закреплена запись реестра. Сервер отдаёт логин только
                           администратору, поэтому строку гейтим по наличию значения, а
                           не по роли: карточка живёт в заявке, проходной и реестре, и
                           перечислять контексты пришлось бы заново при каждом новом. -->
                      <!-- Согласие субъекта на обработку персональных данных: показываем
                           дату, когда отметка есть. Пустое поле у записей, заведённых до
                           введения отметки, - строку тогда не рисуем, чтобы не читалось
                           как «согласия нет». -->
                      <div
                        v-if="employee.pd_consent_at"
                        class="detail-item"
                      >
                        <span class="detail-label">Согласие на обработку ПД:</span>
                        <span
                          class="detail-value"
                          data-testid="employee-pd-consent-date"
                        >получено {{ formatConsentDate(employee.pd_consent_at) }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Действует до:</span>
                        <span class="detail-value">{{ formatDate(employee.entry_date_to) || '-' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Время прохода:</span>
                        <span class="detail-value">{{ employee.pass_time || '-' }}</span>
                      </div>
                    </div>
                    <!-- За кем закреплена запись реестра. Сведения служебные, для бюро,
                         поэтому идут подписью под блоком, а не строкой наравне с данными
                         человека. Сервер отдаёт их только администратору, поэтому строку
                         гейтим наличием значения: карточка живёт в заявке, проходной,
                         реестре и на странице чёрного списка. -->
                    <p
                      v-if="employee.user_name"
                      class="owner-note"
                      data-testid="employee-owner-login"
                    >
                      Запись закреплена за: {{ employee.user_name }}
                    </p>
                  </div>
                </div>

                <!-- Документы (чувствительные данные) - под правом detail.documents -->
                <div
                  v-if="allowedActions.documents"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      Документы
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="details-grid two-columns">
                      <div class="detail-item">
                        <span class="detail-label">Серия и номер паспорта:</span>
                        <div class="sensitive-data">
                          <span 
                            class="data-text"
                            :class="{ 'hidden-data': !showFullPassport }"
                          >
                            {{ employee.passport_series_number || 'Не указан' }}
                          </span>
                          <button 
                            v-if="employee.passport_series_number"
                            class="show-more-btn"
                            @click="togglePassport"
                          >
                            {{ showFullPassport ? 'Скрыть' : 'Показать' }}
                          </button>
                        </div>
                      </div>
                      <div
                        v-if="employee.patent_number"
                        class="detail-item"
                      >
                        <span class="detail-label">Номер патента:</span>
                        <div class="sensitive-data">
                          <span 
                            class="data-text"
                            :class="{ 'hidden-data': !showFullPatent }"
                          >
                            {{ employee.patent_number }}
                          </span>
                          <button 
                            class="show-more-btn"
                            @click="togglePatent"
                          >
                            {{ showFullPatent ? 'Скрыть' : 'Показать' }}
                          </button>
                        </div>
                      </div>
                      <div
                        v-if="employee.other_permission"
                        class="detail-item full-width"
                      >
                        <span class="detail-label">Иное разрешение:</span>
                        <span class="detail-value">{{ employee.other_permission }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Места прохода (таблицы) -->
                <div class="details-section">
                  <div class="section-header">
                    <h4 class="section-title">
                      Места прохода
                    </h4>
                  </div>
                  <div class="section-body">
                    <div class="places-list">
                      <!-- selectedTable - обёртка {table, time_slots, photos, current_status},
                           поэтому сравнение идёт по selectedTable.table.id (#1050): по
                           selectedTable.id оно давало undefined, и подсветка выбранной
                           таблицы не работала вовсе. У машин это место написано верно. -->
                      <div
                        v-for="t in passageActiveTables"
                        :key="t.id"
                        class="place-item"
                        :class="{ 'active': showPlaceModal && selectedTable && selectedTable.table && selectedTable.table.id === t.id }"
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
                        Места прохода не указаны
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Статус территории (скрыт только для employeeslist) -->
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

                <!-- История проходов (скрыта только для employeeslist) -->
                <div
                  v-if="showHistorySection"
                  class="details-section"
                >
                  <div class="section-header">
                    <h4 class="section-title">
                      История проходов
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
                      <div class="loader" />
                      <span>Загрузка истории...</span>
                    </div>
                                    
                    <div
                      v-else-if="entryExitHistory.length === 0"
                      class="no-history"
                    >
                      История проходов отсутствует
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
                            {{ item.comment || getActionComment(item) }}
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

          <!-- Дополнительное модальное окно с деталями места прохода -->
          <transition 
            name="place-slide"
            @after-leave="onPlaceLeave"
          >
            <div
              v-if="showPlaceModal"
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

  <!-- Модальное окно полной истории сотрудника -->
  <EmployeeHistoryModal
    v-if="showFullHistoryModal"
    :last-name="employee?.last_name"
    :first-name="employee?.first_name"
    :middle-name="employee?.middle_name"
    :current-user-id="currentUserId"
    :current-user-name="currentUserName"
    @close="showFullHistoryModal = false"
    @open-application="$emit('open-application', $event)"
  />

  <AddToBlacklistModal
    :show="showAddBlacklist"
    type="person"
    :entity-label="personLabel"
    :saving="savingBlacklist"
    :error="blacklistError"
    @close="closeAddBlacklist"
    @confirm="submitAddBlacklist"
  />
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled } from '@/utils/modalStack';
import { ref, getCurrentInstance } from 'vue';
import { apiRequest } from '@/api/client';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { useOverlayClose } from '@/composables/useOverlayClose';
import TableInfoModal from './TableInfoModal.vue';
import EmployeeHistoryModal from './EmployeeHistoryModal.vue';
import Badge from '@/components/ui/Badge.vue';
import AddToBlacklistModal from '@/components/admin/blacklist/AddToBlacklistModal.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { useDeletionsStore } from '@/stores/deletions';
import { getModalActionPermission } from '@/constants/detailModalActions';
import { checkPersonBlacklist, createPersonBlacklist } from '@/api/blacklist';
import ExcelJS from 'exceljs';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'EmployeeDetailsModal',
    components: {
        AppIcon,
        TableInfoModal,
        EmployeeHistoryModal,
        Badge,
        AddToBlacklistModal
    },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        employee: {
            type: Object,
            default: null
        },
        allTables: {
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
        source: {
            type: String,
            default: 'general'
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
    setup() {
        // Bottom-sheet свайп-вниз-закрытие на мобилке (#1097 r2). onDismiss зовёт метод
        // close() компонента (полная очистка таймеров/подмодалок) через proxy, не голый emit.
        const inst = getCurrentInstance();
        const sheetBody = ref(null);
        const swipe = useSwipeDismiss(() => inst?.proxy?.close?.(), {
            getScrollTop: () => sheetBody.value?.scrollTop ?? 0,
            handleSelector: '.sheet-handle',
        });
        const { isNarrow } = useNarrowScreen();
        // Закрытие по клику мимо окна. onClose зовёт close() компонента, а не голый
        // emit: у карточки есть таймеры и подокна, их гасит именно close().
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => inst?.proxy?.close?.());
        return {
            isNarrow,
            onOverlayMousedown,
            onOverlayMouseup,
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
            selectedTable: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            showFullPassport: false,
            showFullPatent: false,
            history: [],
            loadingHistory: false,
            isExporting: false,
            showFullHistoryModal: false,
            territoryStatus: 0,
            isBlacklisted: false,
            blacklistReason: '',
            showAddBlacklist: false,
            savingBlacklist: false,
            blacklistError: ''
        };
    },
    computed: {
        // Карточка, открытая ИЗ ApplicationDetail (source='application'), лежит ПОВЕРХ его
        // оверлея (z-index 10002). В остальных местах - базовый слой 10001, чтобы открытый
        // из карточки ApplicationDetail ("Открыть заявку") был выше карточки.
        overlayZIndex() {
            return this.source === 'application' ? 10003 : 10001;
        },
        // Предупреждение о возможном обходе ЧС - только в контексте заявки (#481, срез C).
        blacklistSimilar() {
            return this.source === 'application' ? (this.employee?.blacklist_similar || null) : null;
        },
        // Кнопки в шапке: "Полная история" (showHistoryButton) и
        // "Открыть заявку" (source !== 'application' и есть applicationId).
        visibleActionsCount() {
            const history = this.showHistoryButton ? 1 : 0;
            const application = (this.source !== 'application' && !!this.employee?.applicationId) ? 1 : 0;
            return history + application;
        },
        // На телефоне в строку шапки помещается только короткое имя: длинный вариант
        // отжимал крестик и переносился на вторую строку рядом с кнопками действий.
        modalTitle() {
            if (this.isNarrow) return 'Информация';
            const count = this.visibleActionsCount;
            if (count >= 2) return 'Информация';
            if (count === 1) return 'Детальная информация';
            return 'Детальная информация о сотруднике';
        },
        showHistoryButton() {
            return this.source !== 'employeeslist' && this.canSeeFullHistory;
        },
        entryExitHistory() {
            return this.history.filter(item => item.action_type === 'entry' || item.action_type === 'exit');
        },
        // Бейдж источника и зачёркнутые снятые - фича карточки ИЗ ПРОХОДНОЙ (#1227) - зеркало
        // VehicleDetailsModal. Только в проходной target_tables несут реальный source; в заявке/
        // списках/корзине - плоские ID БЕЗ source -> без бейджа (иначе каша: тот же элемент в
        // проходной «добавлено», в заявке дефолтно «из заявки»).
        hasPassageSource() {
            return this.passageActiveTables.some(t => t.source);
        },
        // Активные привязки «Места прохода» (#1227 P3) - зеркало VehicleDetailsModal. Нормализует
        // ОБЕ формы target_tables: заявка - плоский массив ID (число, source=null -> без бейджа),
        // проходной - объекты {id,name,source}. source НЕ фабрикуем (null = не показывать бейдж).
        passageActiveTables() {
            const raw = this.employee?.target_tables || [];
            return raw.map(t => (typeof t === 'number'
                ? { id: t, name: this.getTableName(t), source: null }
                : { id: t.id, name: t.name || this.getTableName(t.id), source: t.source || null }));
        },
        // Снятые/перенесённые таблицы (unbound_from_table/moved_between_tables из истории) -
        // зачёркнутыми, кроме тех, что сейчас снова активны. Дедуп по table_id. Только в проходной.
        passageRemovedTables() {
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
        getStatusClass() {
            const status = this.territoryStatus;
            if (status === 1) return 'status-on-territory';
            if (status === 2) return 'status-exited';
            return 'status-not-entered';
        },
        getStatusText() {
            const status = this.territoryStatus;
            if (status === 1) return 'На территории';
            if (status === 2) return 'Покинул территорию';
            return 'Не входил';
        },
        showStatusSection() {
            return this.source !== 'employeeslist' && this.source !== 'trash';
        },
        showHistorySection() {
            return this.source !== 'employeeslist' && this.canSeeEntryExit;
        },
        canManageBlacklist() {
            return usePermissionsStore().hasPermission('page.admin.blacklist');
        },
        // Право на кнопку «Полная история» / секцию «История проходов»: гейтим ТОЛЬКО
        // когда карта задаёт ключ права (string); контекстные false/true оставляем на
        // условия видимости выше (showHistoryButton/showHistorySection) — нулевая
        // регрессия дефолтной видимости, добавляется лишь возможность отозвать ролью.
        canSeeFullHistory() {
            const v = getModalActionPermission('employee', this.source, 'history');
            return typeof v === 'string' ? usePermissionsStore().hasPermission(v) : true;
        },
        canSeeEntryExit() {
            const v = getModalActionPermission('employee', this.source, 'entryExit');
            return typeof v === 'string' ? usePermissionsStore().hasPermission(v) : true;
        },
        // Доступные действия/разделы модалки по контексту (source) и правам (#187
        // Фаза 2). Значение из карты: boolean - по контексту; строка - ключ права,
        // резолвится через hasPermission. Сейчас гейтит раздел «Документы»
        // (detail.documents есть в базовой роли -> владелец видит документы своих
        // сотрудников; в корзине/списке/ЧС - скрыты).
        allowedActions() {
            const perms = usePermissionsStore();
            const resolve = (action) => {
                const v = getModalActionPermission('employee', this.source, action);
                return typeof v === 'boolean' ? v : perms.hasPermission(v);
            };
            return {
                openApplication: resolve('openApplication'),
                blacklist: resolve('blacklist'),
                documents: resolve('documents'),
            };
        },
        personLast() {
            return (this.employee?.last_name || '').trim();
        },
        personFirst() {
            return (this.employee?.first_name || '').trim();
        },
        personMiddle() {
            return (this.employee?.middle_name || '').trim();
        },
        personLabel() {
            return [this.personLast, this.personFirst, this.personMiddle].filter(Boolean).join(' ');
        },
        hasPersonIdentity() {
            return !!this.personLast && !!this.personFirst;
        }
    },
    watch: {
        show: {
        immediate: true,
        handler(val) {
            // Контракт окна: фон под листом не прокручивается.
            setBodyScrollLock(this, val);
            // И место в стопке окон - чтобы Escape закрывал только верхнее.
            setModalOpen(this, val, this.overlayZIndex);
            if (val) {
                this.loadHistory();
                this.loadEmployeeStatus(); // для EmployeeDetailsModal
                this.checkBlacklist();
                // для VehicleDetailsModal: this.loadCarStatus(); this.loadCarHistory();
            } else {
                this.close(); // вместо this.closeTableDetails()
            }
        }
    },
        employee: {
            deep: true,
            handler(newVal) {
                if (newVal && this.show) {
                    this.loadHistory();
                    this.loadEmployeeStatus();
                    this.checkBlacklist();
                }
            }
        }
    },
    mounted() {
        document.addEventListener('keydown', this.handleEscKey);
        if (this.show) setModalOpen(this, true, this.overlayZIndex);
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.handleEscKey);
        releaseModal(this);
        releaseBodyScrollLock(this);
    },
    methods: {
        /**
         * Закрытие по Escape (фон закрывается через @click.self на оверлее).
         *
         * Через общую стопку окон: карточка, открытая из заявки, лежит поверх её панели,
         * и без стопки одно нажатие закрывало обе - панель считала себя верхней.
         */
        handleEscKey(e) {
            if (e.key !== 'Escape' || !this.show) return;
            if (isEscapeHandled(e)) return;
            if (!isTopModal(this)) return;
            markEscapeHandled(e);
            this.close();
        },
        close() {
    this.$emit('close');
    this.closeTableDetails();
    if (this.shiftTimer) {
        clearTimeout(this.shiftTimer);
        this.shiftTimer = null;
    }
    this.isMainShifted = false; // принудительно сбрасываем
    this.selectedTable = null;
    this.showAddBlacklist = false;
},

        // Статус ЧС по ФИО (зеркалит серверный Check: фамилия+имя+отчество).
        async checkBlacklist() {
            this.isBlacklisted = false;
            this.blacklistReason = '';
            if (!this.hasPersonIdentity) return;
            try {
                const res = await checkPersonBlacklist({
                    last_name: this.personLast,
                    first_name: this.personFirst,
                    middle_name: this.personMiddle,
                });
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
                await createPersonBlacklist({
                    last_name: this.personLast,
                    first_name: this.personFirst,
                    middle_name: this.personMiddle,
                    reason,
                });
                this.isBlacklisted = true;
                this.blacklistReason = reason;
                this.showAddBlacklist = false;
                useDeletionsStore().notify({ prefix: 'Человек ', bold: this.personLabel, suffix: ' добавлен в чёрный список' });
            } catch (e) {
                this.blacklistError = e?.message || 'Не удалось добавить в чёрный список';
            } finally {
                this.savingBlacklist = false;
            }
        },

        togglePassport() {
            this.showFullPassport = !this.showFullPassport;
        },

        togglePatent() {
            this.showFullPatent = !this.showFullPatent;
        },

        getTableName(tableId) {
            let found = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (found) {
                let tbl = found.table || found;
                return tbl.display_name || tbl.name || `ID: ${tableId}`;
            }
            return `Неизвестное место (ID: ${tableId})`;
        },

        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (!tableData) {
                useDeletionsStore().notify({ bold: 'Информация о месте прохода недоступна', type: 'error' });
                return;
            }
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || [],
                photos: tableData.photos || [],
                current_status: tableData.current_status || 'closed'
            };
            if (this.shiftTimer) clearTimeout(this.shiftTimer);
            this.isMainShifted = true;
            this.shiftTimer = setTimeout(() => {
                this.showPlaceModal = true;
                this.shiftTimer = null;
            }, 300);
        },

        closeTableDetails() {
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            this.isMainShifted = false;
            this.selectedTable = null;
        },

        // Отметка согласия хранится полной меткой времени, а formatDate ниже рассчитан
        // на «ГГГГ-ММ-ДД» из полей срока заявки: разбор по дефисам даёт «Invalid Date».
        // Человеку нужен день, поэтому печатаем только дату.
        formatConsentDate(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleDateString('ru-RU');
        },

        formatDate(dateString) {
            if (!dateString) return '';
            try {
                const [y, m, d] = dateString.split('-');
                const date = new Date(y, m-1, d);
                return date.toLocaleDateString('ru-RU');
            } catch { return ''; }
        },

        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit', month: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit'
            }).replace(',', '');
        },

        getActionClass(item) {
            if (!item.user_id) return 'dot-system';
            const classes = { entry: 'dot-entry', exit: 'dot-exit' };
            return classes[item.action_type] || 'dot-default';
        },

        getActionText(item) {
            if (item.action_type === 'entry') return 'Проход на территорию';
            if (item.action_type === 'exit') return 'Выход с территории';
            return '';
        },

        getActionComment(item) {
            if (item.comment) return item.comment;
            const userName = item.user_name || 'Система';
            const fullName = `${this.employee?.last_name || ''} ${this.employee?.first_name || ''}`.trim();
            if (item.action_type === 'entry') return `Пользователь ${userName} отметил проход сотрудника ${fullName}`;
            if (item.action_type === 'exit') return `Пользователь ${userName} отметил выход сотрудника ${fullName}`;
            return '';
        },

        async loadHistory() {
            if (!this.employee) return;
            this.loadingHistory = true;
            try {
                let res;
                // employeesview и blacklist открывают карточку по реестру (unique_employee id),
                // поэтому история - по ФИО (unified), а не по id реальной записи employees.
                if (this.source === 'employeesview' || this.source === 'blacklist') {
                    const params = new URLSearchParams({
                        last_name: this.employee.last_name || '',
                        first_name: this.employee.first_name || ''
                    });
                    if (this.employee.middle_name) params.set('middle_name', this.employee.middle_name);
                    res = await apiRequest(`/employees/history/unified?${params}`, { method: 'GET' });
                } else {
                    res = await apiRequest(`/employees/${this.employee.id}/history`, { method: 'GET' });
                }
                if (res.ok) {
                    this.history = await res.json();
                }
            } catch (e) {
                console.error('Network error:', e);
            } finally {
                this.loadingHistory = false;
            }
        },

        async loadEmployeeStatus() {
            // На вкладке "Сотрудники" (employeesview) employee.id - id реестра, а статус
            // ключуется по employees.id (заявочная таблица), поэтому берём только
            // activeEmployeeId (без отката на employee.id - id-пространства разные).
            // В прочих источниках employee.id уже = employees.id.
            const statusEmployeeId = this.source === 'employeesview' ? this.employee?.activeEmployeeId : this.employee?.id;
            if (!statusEmployeeId) return;
            try {
                const response = await apiRequest('/employees/history/current-status', { method: 'GET' });
                if (response.ok) {
                    const statuses = await response.json();
                    const status = statuses.find(s => s.employee_id === statusEmployeeId);
                    if (status) {
                        this.territoryStatus = status.territory_status;
                    } else {
                        this.territoryStatus = 0;
                    }
                }
            } catch (error) {
                console.error('Ошибка при загрузке статуса сотрудника:', error);
            }
        },

        async exportHistory() {
            if (this.entryExitHistory.length === 0) return;
            this.isExporting = true;
            try {
                const workbook = new ExcelJS.Workbook();
                const worksheet = workbook.addWorksheet('Istoriya_prokhodov');
                
                const headers = ['Дата и время', 'Пользователь', 'Действие', 'Комментарий', 'Место'];
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

                this.entryExitHistory.forEach((item, index) => {
                    const row = worksheet.addRow([
                        this.formatDateTime(item.created_at),
                        item.user_name || 'Система',
                        this.getActionText(item),
                        item.comment || this.getActionComment(item),
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

                const lastDataRow = this.entryExitHistory.length;
                for (let row = 1; row <= lastDataRow + 1; row++) {
                    const rightCell = worksheet.getCell(row, 5);
                    rightCell.border = { ...rightCell.border, right: { style: 'medium', color: { argb: 'FF000000' } } };
                    const leftCell = worksheet.getCell(row, 1);
                    leftCell.border = { ...leftCell.border, left: { style: 'medium', color: { argb: 'FF000000' } } };
                }
                for (let col = 1; col <= 5; col++) {
                    const topCell = worksheet.getCell(1, col);
                    topCell.border = { ...topCell.border, top: { style: 'medium', color: { argb: 'FF000000' } } };
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

                worksheet.columns = [ { width: 25 }, { width: 40 }, { width: 30 }, { width: 60 }, { width: 30 } ];

                const buffer = await workbook.xlsx.writeBuffer();
                const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                const fullName = `${this.employee.last_name}_${this.employee.first_name}`.replace(/[^a-zA-Z0-9]/g, '_');
                a.download = `Istoriya_${fullName}.xlsx`;
                a.href = url;
                a.click();
                window.URL.revokeObjectURL(url);
            } catch (e) { console.error(e); useDeletionsStore().notify({ bold: 'Ошибка экспорта в Excel', type: 'error' }); } finally { this.isExporting = false; }
        },

        openFullHistory() {
            this.showFullHistoryModal = true;
        },

        openApplication() {
            if (this.employee?.applicationId) {
                this.$emit('open-application', this.employee.applicationId);
            }
        }
    }
};
</script>



<style scoped>
/* Все стили остаются без изменений, как в предыдущей версии */
.place-name {
    font-size: 10px;
    color: var(--accent-text);
    margin-top: 2px;
}

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
    
    }
    to {
        background: var(--overlay);
       
    }
}

.modal-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;  /* добавляем */
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
    pointer-events: auto;  /* добавляем */
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

/* Блок "Подозрение на обход ЧС" (#481, срез C) - зеркалит VehicleDetailsModal. */
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

.employee-details {
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

.sensitive-data {
    display: flex;
    align-items: center;
    gap: 15px;
}

.data-text {
    font-size: 13px;
    color: var(--text);
    font-weight: 500;
    letter-spacing: 0.5px;
    transition: all 0.3s ease;
    word-break: break-all;
}

.data-text.hidden-data {
    filter: blur(4px);
    user-select: none;
}

.show-more-btn {
    background: var(--surface-2);
    border: 1px solid var(--border);
    color: var(--accent-text);
    font-size: 11px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 15px;
    transition: all 0.2s;
    font-weight: 500;
    white-space: nowrap;
    min-width: 75px;
    text-align: center;
}

.show-more-btn:hover {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
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
   в ApplicationHistory.vue - "проход был, но сейчас не действует". */
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

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
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

.dot-system { background: #8b5cf6; }
.dot-entry { background: #059669; }
.dot-exit { background: #dc2626; }
.dot-default { background: #9ca3af; }

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
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
}

@keyframes details-modal-out {
    from { opacity: 1; transform: scale(1); }
    to { opacity: 0; transform: scale(0.95); }
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
        /* transition для свайп-спринга/слайда; НЕ transform:none!important - блокировал бы
           inline-transform свайпа (#1097 r2). */
        transition: transform 0.3s ease;
    }

    .modal-content.is-dragging {
        transition: none;
    }

    .sheet-handle {
        display: block;
        width: 40px;
        height: 4px;
        border-radius: 2px;
        background: var(--border);
        margin: 8px auto 0;
        flex-shrink: 0;
    }

    /* Enter/leave = слайд снизу (перебивает базовый scale-поп). */
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
    
    .place-modal-container {
        width: 90%;
        left: 5%;
        height: auto;
        max-height: 80dvh;
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
    
    .sensitive-data {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }
    
    .show-more-btn {
        align-self: flex-start;
    }
}
</style>