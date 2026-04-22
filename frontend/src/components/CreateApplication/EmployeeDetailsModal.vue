<template>
    <transition name="modal-fade">
        <div v-if="show" class="modal-overlay" @mousedown="onOverlayMousedown" @mouseup="onOverlayMouseup">
            <div class="modal-wrapper">
                <!-- Основное модальное окно с деталями сотрудника -->
                <div
                    class="modal-content compact-modal main-modal"
                    :class="{ 'shifted': isMainShifted }"
                    @mousedown.stop
                >
                    <div class="modal-header">
                        <h3 class="modal-title">Детальная информация о сотруднике</h3>
                        <div class="header-actions">
                            <button class="history-btn" @click="showHistory = true">
                                <span>Полная история</span>
                            </button>
                        </div>
                        <button @click="close" class="modal-close">
                            <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                                <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                            </svg>
                        </button>
                    </div>
                    
                    <div class="modal-body">
                        <div class="employee-details" v-if="employee">
                            <div class="details-section">
                                <h4 class="section-title">Основная информация</h4>
                                <div class="details-grid two-columns">
                                    <div class="detail-item">
                                        <label class="detail-label">Фамилия:</label>
                                        <span class="detail-value">{{ employee.lastName || 'Не указано' }}</span>
                                    </div>
                                    <div class="detail-item">
                                        <label class="detail-label">Имя:</label>
                                        <span class="detail-value">{{ employee.firstName || 'Не указано' }}</span>
                                    </div>
                                    <div class="detail-item">
                                        <label class="detail-label">Отчество:</label>
                                        <span class="detail-value">{{ employee.middleName || 'Не указано' }}</span>
                                    </div>
                                    <div class="detail-item">
                                        <label class="detail-label">Должность:</label>
                                        <span class="detail-value">{{ employee.position || 'Не указана' }}</span>
                                    </div>
                                    <div class="detail-item">
                                        <label class="detail-label">Гражданство:</label>
                                        <span class="detail-value">{{ employee.citizenshipName || 'Не указано' }}</span>
                                    </div>
                                </div>
                            </div>

                            <div class="details-section">
                                <h4 class="section-title">Документы</h4>
                                <div class="details-grid two-columns">
                                    <div class="detail-item">
                                        <label class="detail-label">Серия и номер паспорта:</label>
                                        <div class="sensitive-data">
                                            <span 
                                                class="data-text"
                                                :class="{ 'hidden-data': !showFullPassport }"
                                            >
                                                {{ employee.passportSeriesNumber || 'Не указан' }}
                                            </span>
                                            <button 
                                                v-if="employee.passportSeriesNumber"
                                                @click="togglePassportVisibility"
                                                class="show-more-btn"
                                                :class="{ 'active': showFullPassport }"
                                            >
                                                {{ showFullPassport ? 'Скрыть' : 'Показать' }}
                                            </button>
                                        </div>
                                    </div>
                                    <div v-if="employee.patentNumber" class="detail-item">
                                        <label class="detail-label">Номер патента:</label>
                                        <div class="sensitive-data">
                                            <span 
                                                class="data-text"
                                                :class="{ 'hidden-data': !showFullPatent }"
                                            >
                                                {{ employee.patentNumber }}
                                            </span>
                                            <button 
                                                @click="togglePatentVisibility"
                                                class="show-more-btn"
                                                :class="{ 'active': showFullPatent }"
                                            >
                                                {{ showFullPatent ? 'Скрыть' : 'Показать' }}
                                            </button>
                                        </div>
                                    </div>
                                    <div v-if="employee.otherPermission" class="detail-item full-width">
                                        <label class="detail-label">Иное разрешение:</label>
                                        <span class="detail-value">{{ employee.otherPermission }}</span>
                                    </div>
                                </div>
                            </div>

                            <div class="details-section">
                                <h4 class="section-title">Места прохода</h4>
                                <div class="places-list">
                                    <div 
                                        v-for="tableId in employee.targetTables" 
                                        :key="tableId"
                                        class="place-item"
                                        :class="{ 'active': showPlaceModal && selectedTable && selectedTable.table && selectedTable.table.id === tableId }"
                                        @click="showTableDetails(tableId)"
                                    >
                                        {{ getTableName(tableId) }}
                                    </div>
                                    <div v-if="!employee.targetTables || employee.targetTables.length === 0" class="no-places">
                                        Места прохода не указаны
                                    </div>
                                </div>
                            </div>

                            <div v-if="employee.isExisting" class="existing-badge">
                                <span class="badge-text">Существующий сотрудник</span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Дополнительное модальное окно с деталями места прохода -->
                <transition 
                    name="place-slide"
                    @after-leave="onPlaceLeave"
                >
                    <div v-if="showPlaceModal" class="place-modal-container">
                        <TableInfoModal
                            :table="selectedTable"
                            :all-tables="allTables"
                            @close="closeTableDetails"
                        />
                    </div>
                </transition>

                <EmployeeHistoryModal
                    :show="showHistory"
                    :last-name="employee?.lastName || ''"
                    :first-name="employee?.firstName || ''"
                    :middle-name="employee?.middleName || ''"
                    @close="showHistory = false"
                />
            </div>
        </div>
    </transition>
</template>

<script>
import TableInfoModal from './TableInfoModal.vue';
import EmployeeHistoryModal from './EmployeeHistoryModal.vue';
import { useOverlayClose } from '@/composables/useOverlayClose';

export default {
    name: 'EmployeeDetailsModal',
    components: { TableInfoModal, EmployeeHistoryModal },
    setup(_, { emit }) {
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => emit('close'));
        return { onOverlayMousedown, onOverlayMouseup };
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
        }
    },
    emits: ['close'],
    data() {
        return {
            selectedTable: null,
            showPlaceModal: false,
            isMainShifted: false,
            shiftTimer: null,
            showFullPassport: false,
            showFullPatent: false,
            showHistory: false
        };
    },
    methods: {
        close() {
            this.$emit('close');
            this.closeTableDetails();
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
        },

        togglePassportVisibility() {
            this.showFullPassport = !this.showFullPassport;
        },

        togglePatentVisibility() {
            this.showFullPatent = !this.showFullPatent;
        },

        getTableName(tableId) {
            // Пробуем найти таблицу в разных форматах
            let foundTable = null;
            
            for (const item of this.allTables) {
                if (item.table && item.table.id === tableId) {
                    foundTable = item.table;
                    break;
                }
                if (item.id === tableId) {
                    foundTable = item;
                    break;
                }
            }
            
            if (foundTable) {
                return foundTable.display_name || foundTable.name || `ID: ${tableId}`;
            }
            return `Неизвестное место (ID: ${tableId})`;
        },

        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => {
                if (t.table && t.table.id === tableId) return true;
                if (t.id === tableId) return true;
                return false;
            });
            
            if (!tableData) {
                console.error(`Таблица с ID ${tableId} не найдена`);
                alert(`Информация о месте прохода с ID ${tableId} недоступна`);
                return;
            }

            // Нормализуем данные для единообразной работы
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || (tableData.table && tableData.table.time_slots) || [],
                photos: tableData.photos || (tableData.table && tableData.table.photos) || [],
                current_status: tableData.current_status || (tableData.table && tableData.table.current_status) || 'closed'
            };

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

        closeTableDetails() {
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            this.isMainShifted = false;
            this.selectedTable = null;
        }
    }
};
</script>

<style scoped>
/* Стили взяты из исходного EmployeesList для модального окна */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.3);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(1px);
    animation: overlayAppear 0.4s ease-out;
}

@keyframes overlayAppear {
    from {
        background: rgba(0, 0, 0, 0);
        backdrop-filter: blur(0px);
    }
    to {
        background: rgba(0, 0, 0, 0.3);
        backdrop-filter: blur(1px);
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
    width: 520px;
    height: 450px;
    max-height: 450px;
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

.modal-content.compact-modal {
    width: 520px;
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

.employee-details {
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.details-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 20px;
    background: #fafafa;
}

.section-title {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: #333;
    padding-bottom: 8px;
    border-bottom: 1px solid #e6e6e6;
}

.details-grid.two-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
}

.detail-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.detail-item.full-width {
    grid-column: 1 / -1;
}

.detail-label {
    font-size: 12px;
    color: #666;
    font-weight: 500;
}

.detail-value {
    font-size: 13px;
    color: #333;
    font-weight: 500;
}

.sensitive-data {
    display: flex;
    align-items: center;
    gap: 15px;
}

.data-text {
    font-size: 13px;
    color: #333;
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
    background: #f8f9fa;
    border: 1px solid #e0e0e0;
    color: #4F5BDF;
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
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.show-more-btn.active {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    min-width: 75px;
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

.existing-badge {
    display: flex;
    justify-content: center;
    margin-top: 8px;
}

.badge-text {
    background: #e3f2fd;
    color: #1976d2;
    padding: 6px 12px;
    border-radius: 16px;
    font-size: 12px;
    font-weight: 500;
}

.header-actions {
    display: flex;
    gap: 8px;
    align-items: center;
}

.history-btn {
    background: #f8f9fa;
    border: 1px solid #e0e0e0;
    color: #4F5BDF;
    font-size: 12px;
    cursor: pointer;
    padding: 5px 12px;
    border-radius: 15px;
    transition: all 0.2s;
    font-weight: 500;
    white-space: nowrap;
}

.history-btn:hover {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

/* Анимации */
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
    .modal-wrapper {
        flex-direction: column;
        gap: 10px;
    }
    
    .modal-content {
        position: static;
        margin-bottom: 10px;
    }
    
    .modal-content.main-modal {
        left: auto;
        transform: none !important;
    }
    
    .modal-content.main-modal.shifted {
        transform: none !important;
    }
    
    .modal-content {
        height: auto;
        max-height: 80vh;
    }
    
    .modal-body {
        padding: 16px 20px;
    }
    
    .modal-header {
        padding: 16px 20px;
    }
    
    .details-section {
        border-radius: 16px;
        padding: 16px;
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

@media (max-width: 480px) {
    .modal-header {
        padding: 12px 16px;
    }
    
    .modal-title {
        font-size: 14px;
    }
    
    .section-title {
        font-size: 13px;
    }
    
    .detail-label {
        font-size: 11px;
    }
    
    .detail-value {
        font-size: 12px;
    }
    
    .modal-content.compact-modal {
        border-radius: 20px;
    }
    
    .details-section {
        border-radius: 12px;
        padding: 12px;
    }
}
</style>