<template>
    <div class="modal-overlay" @click="$emit('close')">
        <div class="modal-content" @click.stop>
            <div class="modal-header">
                <div class="modal-header__top">
                    <h3>Привязка новых данных</h3>
                </div>
                <button class="modal-close" @click="$emit('close')">×</button>
            </div>
            <div class="modal-body">
                <div class="binding-info">
                    <p class="binding-description">
                        Все добавленные данные будут <strong>автоматически привязаны</strong> к вашему аккаунту.
                        Вы можете дополнительно привязать их к организации и/или компании для использования <strong>другими сотрудниками</strong>:
                    </p>
                    
                    <!-- Секция для новых автомобилей -->
                    <div v-if="newVehiclesToBind.length > 0" class="data-section vehicles-section">
                        <div class="section-header">
                            <h4 class="section-title">Новые автомобили ({{ newVehiclesToBind.length }})</h4>
                        </div>
                        
                        <div class="vehicles-list">
                            <div 
                                v-for="vehicle in newVehiclesToBind" 
                                :key="vehicle.id"
                                class="vehicle-item"
                                :class="{ 'vehicle-item--fact': isVehicleByFact(vehicle) }"
                            >
                                <div class="vehicle-info">
                                    <span class="vehicle-number">{{ vehicle.plateNumber }}</span>
                                    <span class="vehicle-mark">{{ vehicle.mark }}</span>
                                    <span v-if="isVehicleByFact(vehicle)" class="fact-badge">По факту</span>
                                </div>
                            </div>
                        </div>

                        <div v-if="hasVehiclesForBinding" class="binding-options-section">
                            <p class="options-title">Привязать автомобили к:</p>
                            <div class="binding-options">
                                <label class="binding-option" v-if="hasOrganization">
                                    <input 
                                        type="checkbox" 
                                        v-model="vehiclesBindToOrganization"
                                        :disabled="!hasVehiclesForBinding"
                                    />
                                    <span class="option-text">Организации "{{ organization }}"</span>
                                </label>
                                <label class="binding-option" v-if="hasCompany">
                                    <input 
                                        type="checkbox" 
                                        v-model="vehiclesBindToCompany"
                                        :disabled="!hasVehiclesForBinding"
                                    />
                                    <span class="option-text">Компании "{{ company }}"</span>
                                </label>
                            </div>
                            <div v-if="!hasVehiclesForBinding" class="no-binding-message">
                                Автомобили "По факту" не требуют привязки к организации/компании
                            </div>
                        </div>
                    </div>
                    
                    <!-- Секция для новых сотрудников -->
                    <div v-if="newEmployeesToBind.length > 0" class="data-section employees-section">
                        <div class="section-header">
                            <h4 class="section-title">Новые сотрудники ({{ newEmployeesToBind.length }})</h4>
                        </div>
                        
                        <div class="employees-list">
                            <div 
                                v-for="employee in newEmployeesToBind" 
                                :key="employee.id"
                                class="employee-item"
                                :class="{ 'employee-item--fact': isEmployeeByFact(employee) }"
                            >
                                <div class="employee-info">
                                    <span class="employee-name">{{ formatFullName(employee) }}</span>
                                    <span class="employee-position">{{ employee.position }}</span>
                                    <span v-if="isEmployeeByFact(employee)" class="fact-badge">По факту</span>
                                </div>
                            </div>
                        </div>

                        <div v-if="hasEmployeesForBinding" class="binding-options-section">
                            <p class="options-title">Привязать сотрудников к:</p>
                            <div class="binding-options">
                                <label class="binding-option" v-if="hasOrganization">
                                    <input 
                                        type="checkbox" 
                                        v-model="employeesBindToOrganization"
                                        :disabled="!hasEmployeesForBinding"
                                    />
                                    <span class="option-text">Организации "{{ organization }}"</span>
                                </label>
                                <label class="binding-option" v-if="hasCompany">
                                    <input 
                                        type="checkbox" 
                                        v-model="employeesBindToCompany"
                                        :disabled="!hasEmployeesForBinding"
                                    />
                                    <span class="option-text">Компании "{{ company }}"</span>
                                </label>
                            </div>
                            <div v-if="!hasEmployeesForBinding" class="no-binding-message">
                                Все сотрудники имеют поля "По факту" и не требуют привязки
                            </div>
                        </div>
                    </div>

                    <div class="warning-section">
                        <p class="warning-text">
                            <strong class="red">Внимание!</strong> При привязке данных к организации или компании, они будут доступны для отображения и использования для всех сотрудников, привязанных к организации/компании.
                        </p>
                    </div>
                </div>
                
                <div class="modal-actions">
                    <button class="skip-btn" @click="handleSkip">Пропустить и отправить</button>
                    <button class="confirm-btn" @click="handleConfirm">
                        {{ buttonText }}
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
            
            if (hasAnyBinding) {
                return 'Привязать и отправить';
            } else {
                return 'Отправить без привязки';
            }
        },
        
        // Проверяем, есть ли автомобили для привязки (не "По факту")
        hasVehiclesForBinding() {
            return this.newVehiclesToBind.some(vehicle => !this.isVehicleByFact(vehicle));
        },
        
        // Проверяем, есть ли сотрудники для привязки (не "По факту")
        hasEmployeesForBinding() {
            return this.newEmployeesToBind.some(employee => !this.isEmployeeByFact(employee));
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
        
        // Проверка, является ли автомобиль "По факту"
        isVehicleByFact(vehicle) {
            return vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
        },
        
        // Проверка, является ли сотрудник "По факту"
        isEmployeeByFact(employee) {
            return employee.passportSeriesNumber === 'По факту' || 
                   employee.position === 'По факту';
        },
        
        handleConfirm() {
            // Отправляем данные о привязке раздельно для автомобилей и сотрудников
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
            this.$emit('skip-binding');
        }
    }
}
</script>

<style scoped>
/* Модальное окно привязки */
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
}

.modal-content {
    background: white;
    border-radius: 20px;
    padding: 0;
    width: 550px;
    max-width: 90vw;
    max-height: 700px;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 15px 20px;
    border-bottom: 1px solid #e6e6e6;
}

.modal-header__top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex: 1;
    height: 25px;
}

.modal-header h3 {
    margin: 0;
    color: #333;
    font-size: 18px;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: #a2a2a2;
    padding: 0;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 10px;
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: 600px;
    overflow-y: auto;
}

.binding-info {
    margin-bottom: 20px;
}

.binding-description {
    font-size: 14px;
    line-height: 1.5;
    color: #666;
    margin-bottom: 25px;
    text-align: left;
    padding-bottom: 15px;
    border-bottom: 1px solid #f0f0f0;
}

/* Общие стили для секций */
.data-section {
    margin-bottom: 25px;
    padding-bottom: 20px;
    border-bottom: 1px solid #f0f0f0;
}

.data-section:last-of-type {
    border-bottom: none;
    margin-bottom: 20px;
    padding-bottom: 0;
}

.section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
}

.section-title {
    font-size: 16px;
    font-weight: 600;
    color: #333;
    margin: 0;
}

/* Стили для списков */
.vehicles-list,
.employees-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 150px;
    overflow-y: auto;
    padding-right: 5px;
    margin-bottom: 20px;
}

.vehicle-item,
.employee-item {
    padding: 12px 15px;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    background: #f8f9fa;
    transition: all 0.2s ease;
    position: relative;
}

.vehicle-item:hover,
.employee-item:hover {
    border-color: #4F5BDF;
    background: #f0f2ff;
}

/* Стили для автомобилей */
.vehicles-section .section-title {
    color: #4F5BDF;
    border-left: 4px solid #4F5BDF;
    padding-left: 10px;
}

.vehicles-section .vehicle-item {
    border-left: 3px solid #4F5BDF;
}

.vehicle-info {
    display: flex;
    align-items: center;
    gap: 15px;
}

.vehicle-number {
    font-weight: 600;
    color: #333;
    font-size: 14px;
    min-width: 100px;
}

.vehicle-mark {
    color: #666;
    font-size: 13px;
    background: #e8e8e8;
    padding: 4px 10px;
    border-radius: 12px;
}

/* Стили для сотрудников */
.employees-section .section-title {
    color: #2e7d32;
    border-left: 4px solid #2e7d32;
    padding-left: 10px;
}

.employees-section .employee-item {
    border-left: 3px solid #2e7d32;
}

.employee-info {
    display: flex;
    align-items: center;
    gap: 15px;
}

.employee-name {
    font-weight: 600;
    color: #333;
    font-size: 14px;
    min-width: 150px;
}

.employee-position {
    color: #666;
    font-size: 13px;
    background: #e8e8e8;
    padding: 4px 10px;
    border-radius: 12px;
}

/* Стили для "По факту" */
.fact-badge {
    background: #ff9800;
    color: white;
    font-size: 11px;
    padding: 3px 8px;
    border-radius: 10px;
    font-weight: 500;
    margin-left: auto;
}

.vehicle-item--fact,
.employee-item--fact {
    background: #fff3e0;
    border-color: #ff9800;
}

.vehicle-item--fact:hover,
.employee-item--fact:hover {
    background: #ffe0b2;
    border-color: #f57c00;
}

/* Стили для опций привязки */
.binding-options-section {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #e6e6e6;
}

.options-title {
    font-size: 14px;
    font-weight: 600;
    color: #333;
    margin-bottom: 12px;
}

.binding-options {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-size: 14px;
    padding: 8px 0;
    transition: all 0.2s;
}

.binding-option:hover:not(:has(input:disabled)) {
    background-color: #f8f9fa;
    padding: 8px 10px;
    border-radius: 8px;
}

.binding-option input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #4F5BDF;
}

.binding-option input[type="checkbox"]:disabled {
    cursor: not-allowed;
    opacity: 0.5;
}

.option-text {
    color: #333;
}

.binding-option input[type="checkbox"]:disabled + .option-text {
    color: #999;
}

.no-binding-message {
    font-size: 12px;
    color: #666;
    font-style: italic;
    margin-top: 10px;
    padding: 8px;
    background: #f5f5f5;
    border-radius: 8px;
    border-left: 3px solid #ff9800;
}

.warning-section {
    margin-top: 20px;
    padding: 15px;
    background: #fff5f5;
    border-radius: 12px;
    border: 1px solid #ffcccc;
}

.warning-text {
    font-size: 13px;
    line-height: 1.5;
    color: #666;
    margin: 0;
    text-align: left;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding-top: 20px;
    border-top: 1px solid #e6e6e6;
}

.skip-btn {
    background: white;
    color: #666;
    border: 1px solid #e6e6e6;
    border-radius: 12px;
    padding: 12px 24px;
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s;
    min-width: 120px;
}

.skip-btn:hover {
    background: #f5f5f5;
    border-color: #ccc;
}

.confirm-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 12px;
    padding: 12px 30px;
    font-size: 14px;
    cursor: pointer;
    transition: background-color 0.2s;
    min-width: 180px;
}

.confirm-btn:hover {
    background: #3a45c0;
}

.blue {
    color: #4F5BDF;
}

.red {
    color: #ff4444;
}

@media (max-width: 768px) {
    .modal-content {
        width: 95vw;
        margin: 10px;
        max-height: 500px;
    }
    
    .modal-body {
        max-height: 400px;
    }
    
    .modal-actions {
        flex-direction: column;
    }
    
    .skip-btn,
    .confirm-btn {
        width: 100%;
        min-width: auto;
    }
    
    .vehicle-info,
    .employee-info {
        flex-direction: column;
        align-items: flex-start;
        gap: 5px;
    }
    
    .vehicle-number,
    .employee-name {
        min-width: auto;
    }
    
    .fact-badge {
        margin-left: 0;
        margin-top: 5px;
    }
}
</style>