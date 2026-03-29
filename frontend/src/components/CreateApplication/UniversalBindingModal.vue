<template>
    <div v-if="show" class="modal-overlay" @click.self="closeModal">
        <div class="modal-content">
            <div class="modal-header">
                <h3 class="modal-title">Привязка новых данных</h3>
                <button class="modal-close" @click="closeModal">
                    <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                        <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                    </svg>
                </button>
            </div>

            <div class="modal-body">
                <!-- Описание -->
                <div class="binding-description">
                    Все добавленные данные будут <strong>автоматически привязаны</strong> к вашему аккаунту.
                    Вы можете дополнительно привязать их к организации и/или компании для использования <strong>другими сотрудниками</strong>:
                </div>

                <!-- Секция: Новые автомобили -->
                <div v-if="newVehiclesToBind.length > 0" class="data-section">
                    <div class="section-header">
                        <h4 class="section-title">Новые автомобили</h4>
                        <span class="count-badge">{{ newVehiclesToBind.length }}</span>
                    </div>
                    <div class="section-body">
                        <div class="items-list">
                            <div
                                v-for="vehicle in newVehiclesToBind"
                                :key="vehicle.id"
                                class="item"
                                :class="{ 'item-fact': isVehicleByFact(vehicle) }"
                            >
                                <div class="item-info">
                                    <span class="item-number">{{ vehicle.plateNumber }}</span>
                                    <span class="item-detail">{{ vehicle.mark }}</span>
                                    <span v-if="isVehicleByFact(vehicle)" class="fact-badge">По факту</span>
                                </div>
                            </div>
                        </div>

                        <div v-if="hasVehiclesForBinding" class="binding-options">
                            <p class="options-title">Привязать автомобили к:</p>
                            <div class="options-group">
                                <label v-if="hasOrganization" class="binding-option">
                                    <input type="checkbox" v-model="vehiclesBindToOrganization" />
                                    <span>Организации «{{ organization }}»</span>
                                </label>
                                <label v-if="hasCompany" class="binding-option">
                                    <input type="checkbox" v-model="vehiclesBindToCompany" />
                                    <span>Компании «{{ company }}»</span>
                                </label>
                            </div>
                        </div>
                        <div v-else class="no-binding-message">
                            Автомобили «По факту» не требуют привязки к организации/компании
                        </div>
                    </div>
                </div>

                <!-- Секция: Новые сотрудники -->
                <div v-if="newEmployeesToBind.length > 0" class="data-section">
                    <div class="section-header">
                        <h4 class="section-title">Новые сотрудники</h4>
                        <span class="count-badge">{{ newEmployeesToBind.length }}</span>
                    </div>
                    <div class="section-body">
                        <div class="items-list">
                            <div
                                v-for="employee in newEmployeesToBind"
                                :key="employee.id"
                                class="item"
                                :class="{ 'item-fact': isEmployeeByFact(employee) }"
                            >
                                <div class="item-info">
                                    <span class="item-name">{{ formatFullName(employee) }}</span>
                                    <span class="item-detail">{{ employee.position }}</span>
                                    <span v-if="isEmployeeByFact(employee)" class="fact-badge">По факту</span>
                                </div>
                            </div>
                        </div>

                        <div v-if="hasEmployeesForBinding" class="binding-options">
                            <p class="options-title">Привязать сотрудников к:</p>
                            <div class="options-group">
                                <label v-if="hasOrganization" class="binding-option">
                                    <input type="checkbox" v-model="employeesBindToOrganization" />
                                    <span>Организации «{{ organization }}»</span>
                                </label>
                                <label v-if="hasCompany" class="binding-option">
                                    <input type="checkbox" v-model="employeesBindToCompany" />
                                    <span>Компании «{{ company }}»</span>
                                </label>
                            </div>
                        </div>
                        <div v-else class="no-binding-message">
                            Сотрудники «По факту» не требуют привязки к организации/компании
                        </div>
                    </div>
                </div>

                <!-- Предупреждение -->
                <div class="warning-section">
                    <p class="warning-text">
                        <strong class="warning-strong">Внимание!</strong> При привязке данных к организации или компании,
                        они будут доступны для отображения и использования <strong>всем</strong> сотрудникам, которые в них числятся.
                    </p>
                </div>

                <!-- Кнопки действий -->
                <div class="modal-actions">
                    <button class="btn skip-btn" @click="handleSkip">Отправить без привязки</button>
                    <button class="btn confirm-btn" @click="handleConfirm">
                        <transition name="fade" mode="out-in">
                            <span :key="buttonText" class="button-text">{{ buttonText }}</span>
                        </transition>
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'UniversalBindingModal',
    props: {
        newVehiclesToBind: {
            type: Array,
            default: () => []
        },
        newEmployeesToBind: {
            type: Array,
            default: () => []
        },
        organization: {
            type: String,
            default: ''
        },
        company: {
            type: String,
            default: ''
        },
        hasOrganization: {
            type: Boolean,
            default: false
        },
        hasCompany: {
            type: Boolean,
            default: false
        },
        show: {
            type: Boolean,
            default: false
        }
    },
    emits: ['confirm-binding', 'skip-binding', 'close'],
    data() {
        return {
            vehiclesBindToOrganization: false,
            vehiclesBindToCompany: false,
            employeesBindToOrganization: false,
            employeesBindToCompany: false
        }
    },
    computed: {
        buttonText() {
            const hasAnyBinding = this.vehiclesBindToOrganization || this.vehiclesBindToCompany ||
                                 this.employeesBindToOrganization || this.employeesBindToCompany;
            return hasAnyBinding ? 'Привязать и отправить' : 'Отправить заявку';
        },
        hasVehiclesForBinding() {
            return this.newVehiclesToBind.some(vehicle => !this.isVehicleByFact(vehicle));
        },
        hasEmployeesForBinding() {
            return this.newEmployeesToBind.some(employee => !this.isEmployeeByFact(employee));
        }
    },
    watch: {
        show: {
            immediate: true,
            handler(newVal) {
                console.log('UniversalBindingModal show prop changed:', newVal);
            }
        }
    },
    methods: {
        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },
        isVehicleByFact(vehicle) {
            return vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
        },
        isEmployeeByFact(employee) {
            return employee.passportSeriesNumber === 'По факту' ||
                   employee.position === 'По факту';
        },
        closeModal() {
            console.log('Closing modal');
            this.$emit('close');
        },
        handleConfirm() {
            console.log('Confirm binding clicked');
            this.$emit('confirm-binding', {
                vehicles: {
                    bindToOrganization: this.vehiclesBindToOrganization,
                    bindToCompany: this.vehiclesBindToCompany,
                    hasVehiclesForBinding: this.hasVehiclesForBinding
                },
                employees: {
                    bindToOrganization: this.employeesBindToOrganization,
                    bindToCompany: this.employeesBindToCompany,
                    hasEmployeesForBinding: this.hasEmployeesForBinding
                }
            });
        },
        handleSkip() {
            console.log('Skip binding clicked');
            this.$emit('skip-binding');
        }
    }
}
</script>

<style scoped>
/* Анимация текста кнопки - как в UserProfileHeader */
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
    transform: scale(0.95);
}

.fade-enter-to,
.fade-leave-from {
    opacity: 1;
    transform: scale(1);
}

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
        background: rgba(0, 0, 0, 0.5);
        backdrop-filter: blur(1px);
    }
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: 80vh;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: contentAppear 0.4s cubic-bezier(0.25, 0.1, 0.15, 1);
}

@keyframes contentAppear {
    from {
        opacity: 0;
        transform: scale(0.9) translateY(-20px);
    }
    to {
        opacity: 1;
        transform: scale(1) translateY(0);
    }
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
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-body::-webkit-scrollbar {
    display: none;
}

.binding-description {
    font-size: 14px;
    line-height: 1.5;
    color: #595959;
    margin-bottom: 20px;
    padding-bottom: 12px;
    border-bottom: 1px solid #f0f0f0;
}

.binding-description strong {
    color: #1a1a1a;
    font-weight: 600;
}

/* Секции данных */
.data-section {
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    background: #fafafa;
    margin-bottom: 20px;
    overflow: hidden;
}

.data-section:last-of-type {
    margin-bottom: 16px;
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

.count-badge {
    background: #e6e6e6;
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
    color: #666;
}

.section-body {
    padding: 16px 20px;
}

.items-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
}

.item {
    padding: 10px 14px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    transition: all 0.2s ease;
}

.item-fact {
    background: #fff8e6;
    border-color: #ffc107;
}

.item-info {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
    flex-wrap: wrap;
}

.item-number,
.item-name {
    font-weight: 500;
    color: #333;
}

.item-detail {
    color: #666;
    background: #f0f0f0;
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
}

.fact-badge {
    background: #ff9800;
    color: white;
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    margin-left: auto;
}

.binding-options {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid #e6e6e6;
}

.options-title {
    font-size: 13px;
    font-weight: 500;
    color: #333;
    margin-bottom: 8px;
}

.options-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 13px;
    padding: 4px;
    border-radius: 20px;
    transition: background 0.2s;
}

.binding-option:hover {
    background: #f0f0f0;
}

.binding-option input {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #4F5BDF;
}

.binding-option span {
    color: #333;
}

.no-binding-message {
    font-size: 12px;
    color: #666;
    font-style: italic;
    padding: 8px 12px;
    background: #f8f9fa;
    border-radius: 20px;
    border-left: 3px solid #ff9800;
    margin-top: 8px;
}

/* Предупреждение */
.warning-section {
    background: #fff3cd;
    border: 1px solid #ffeeba;
    border-radius: 20px;
    padding: 12px 20px;
    margin: 8px 0 20px;
}

.warning-text {
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
    color: #856404;
}

.warning-strong {
    font-weight: 600;
}

/* Кнопки действий */
.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 16px;
}

.btn {
    padding: 8px 20px;
    font-size: 13px;
    font-weight: 500;
    border-radius: 30px;
    cursor: pointer;
    border: 1px solid;
    transition: all 0.2s ease;
}

.skip-btn {
    background: white;
    color: #666;
    border-color: #e6e6e6;
}

.skip-btn:hover {
    background: #f5f5f5;
    border-color: #ccc;
    color: #333;
}

.confirm-btn {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    width: 205px;
    min-width: 200px;
    position: relative;
    overflow: hidden;
}

.confirm-btn:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

.button-text {
    display: inline-block;
}

/* Адаптивность */
@media (max-width: 768px) {
    .modal-content {
        width: 95vw;
        margin: 16px;
        max-height: 85vh;
        border-radius: 30px;
    }

    .modal-header {
        padding: 16px 20px;
    }

    .modal-body {
        padding: 16px 20px;
    }

    .section-header {
        padding: 10px 16px;
    }

    .section-body {
        padding: 14px 16px;
    }

    .item-info {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }

    .fact-badge {
        margin-left: 0;
        align-self: flex-start;
    }

    .modal-actions {
        flex-direction: column;
        gap: 8px;
    }

    .btn {
        width: 100%;
        min-width: auto;
    }
    
    .confirm-btn {
        width: 100%;
        min-width: auto;
    }
}
</style>