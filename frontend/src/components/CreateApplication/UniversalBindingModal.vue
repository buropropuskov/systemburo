<template>
    <div class="modal-overlay" @click="$emit('close')">
        <div class="modal-content" @click.stop>
            <div class="modal-header">
                <h3>Привязка новых данных</h3>
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
                            <h4>Новые автомобили ({{ newVehiclesToBind.length }})</h4>
                        </div>
                        
                        <div class="items-list">
                            <div 
                                v-for="vehicle in newVehiclesToBind" 
                                :key="vehicle.id"
                                class="item vehicle-item"
                                :class="{ 'item-fact': isVehicleByFact(vehicle) }"
                            >
                                <div class="item-info">
                                    <span class="item-number">{{ vehicle.plateNumber }}</span>
                                    <span class="item-detail">{{ vehicle.mark }}</span>
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
                                    />
                                    <span>Организации "{{ organization }}"</span>
                                </label>
                                <label class="binding-option" v-if="hasCompany">
                                    <input 
                                        type="checkbox" 
                                        v-model="vehiclesBindToCompany"
                                    />
                                    <span>Компании "{{ company }}"</span>
                                </label>
                            </div>
                        </div>
                        <div v-else class="no-binding-message">
                            Автомобили "По факту" не требуют привязки к организации/компании
                        </div>
                    </div>
                    
                    <!-- Секция для новых сотрудников -->
                    <div v-if="newEmployeesToBind.length > 0" class="data-section employees-section">
                        <div class="section-header">
                            <h4>Новые сотрудники ({{ newEmployeesToBind.length }})</h4>
                        </div>
                        
                        <div class="items-list">
                            <div 
                                v-for="employee in newEmployeesToBind" 
                                :key="employee.id"
                                class="item employee-item"
                                :class="{ 'item-fact': isEmployeeByFact(employee) }"
                            >
                                <div class="item-info">
                                    <span class="item-name">{{ formatFullName(employee) }}</span>
                                    <span class="item-detail">{{ employee.position }}</span>
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
                                    />
                                    <span>Организации "{{ organization }}"</span>
                                </label>
                                <label class="binding-option" v-if="hasCompany">
                                    <input 
                                        type="checkbox" 
                                        v-model="employeesBindToCompany"
                                    />
                                    <span>Компании "{{ company }}"</span>
                                </label>
                            </div>
                        </div>
                        <div v-else class="no-binding-message">
                            Сотрудники "По факту" не требуют привязки к организации/компании
                        </div>
                    </div>

                    <div class="warning-section">
                        <p class="warning-text">
                            <strong class="warning-strong">Внимание!</strong> При привязке данных к организации или компании, они будут доступны для отображения и использования для <strong>всех</strong> сотрудников, которые в них числятся.
                        </p>
                    </div>
                </div>
                
                <div class="modal-actions">
                    <button class="btn skip-btn" @click="handleSkip">Отправить без привязки</button>
                    <button class="btn confirm-btn" @click="handleConfirm">
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
                return 'Отправить заявку';
            }
        },
        
        hasVehiclesForBinding() {
            return this.newVehiclesToBind.some(vehicle => !this.isVehicleByFact(vehicle));
        },
        
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
        
        isVehicleByFact(vehicle) {
            return vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
        },
        
        isEmployeeByFact(employee) {
            return employee.passportSeriesNumber === 'По факту' || 
                   employee.position === 'По факту';
        },
        
        handleConfirm() {
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
            // Кнопка "Отправить без привязки" - отправляет данные без привязки к организации/компании
            this.$emit('skip-binding');
        }
    }
}
</script>

<style scoped>
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
    width: 540px;
    max-width: 90vw;
    max-height: 80vh;
    overflow: hidden;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    border-bottom: 1px solid #e8e8e8;
}

.modal-header h3 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: #1a1a1a;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    line-height: 1;
    cursor: pointer;
    color: #666;
    padding: 0;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-close:hover {
    color: #333;
}

.modal-body {
    padding: 20px;
    max-height: calc(80vh - 60px);
    overflow-y: auto;
}

.binding-info {
    margin-bottom: 20px;
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

.data-section {
    margin-bottom: 20px;
    padding-bottom: 16px;
    border-bottom: 1px solid #f0f0f0;
}

.data-section:last-of-type {
    border-bottom: none;
    margin-bottom: 16px;
    padding-bottom: 0;
}

.section-header {
    margin-bottom: 12px;
}

.section-header h4 {
    font-size: 16px;
    font-weight: 600;
    color: #1a1a1a;
    margin: 0;
}

.vehicles-section .section-header h4 {
    color: #4F5BDF;
    border-left: 4px solid #4F5BDF;
    padding-left: 10px;
}

.employees-section .section-header h4 {
    color: #2e7d32;
    border-left: 4px solid #2e7d32;
    padding-left: 10px;
}

.items-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 120px;
    overflow-y: auto;
    margin-bottom: 16px;
    padding-right: 4px;
}

.item {
    padding: 10px 14px;
    border: 1px solid #e8e8e8;
    border-radius: 8px;
    background: #fafafa;
}

.vehicle-item {
    border-left: 3px solid #4F5BDF;
}

.employee-item {
    border-left: 3px solid #2e7d32;
}

.item-fact {
    background: #fff8e6;
    border-color: #ffc107;
}

.item-info {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 14px;
}

.item-number,
.item-name {
    font-weight: 600;
    color: #1a1a1a;
    min-width: 120px;
}

.item-detail {
    color: #595959;
    background: #f0f0f0;
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 13px;
}

.fact-badge {
    background: #ff9800;
    color: white;
    font-size: 11px;
    font-weight: 500;
    padding: 3px 8px;
    border-radius: 10px;
    margin-left: auto;
}

.binding-options-section {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid #f0f0f0;
}

.options-title {
    font-size: 14px;
    font-weight: 600;
    color: #1a1a1a;
    margin-bottom: 10px;
}

.binding-options {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 14px;
    padding: 6px;
    border-radius: 6px;
}

.binding-option:hover {
    background-color: #f8f9fa;
}

.binding-option input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #4F5BDF;
}

.binding-option span {
    color: #1a1a1a;
}

.no-binding-message {
    font-size: 12px;
    color: #666;
    font-style: italic;
    margin-top: 10px;
    padding: 8px;
    background: #f8f9fa;
    border-radius: 6px;
    border-left: 3px solid #ff9800;
}

.warning-section {
    margin-top: 16px;
    padding: 12px;
    background: #fff5f5;
    border-radius: 8px;
    border: 1px solid #ffcdd2;
}

.warning-text {
    font-size: 12px;
    line-height: 1.5;
    color: #666;
    margin: 0;
}

.warning-strong {
    color: #d32f2f;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding-top: 16px;
    border-top: 1px solid #f0f0f0;
}

.btn {
    padding: 10px 20px;
    font-size: 14px;
    font-weight: 500;
    border-radius: 8px;
    cursor: pointer;
    border: 1px solid;
    transition: all 0.2s;
    min-width: 140px;
}

.skip-btn {
    background: white;
    color: #595959;
    border-color: #d9d9d9;
}

.skip-btn:hover {
    background: #fafafa;
    border-color: #bfbfbf;
    color: #1a1a1a;
}

.confirm-btn {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.confirm-btn:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

@media (max-width: 768px) {
    .modal-content {
        width: 95vw;
        margin: 16px;
        max-height: 85vh;
    }
    
    .modal-body {
        max-height: calc(85vh - 60px);
        padding: 16px;
    }
    
    .modal-actions {
        flex-direction: column;
    }
    
    .btn {
        width: 100%;
        min-width: auto;
    }
    
    .item-info {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }
    
    .item-number,
    .item-name {
        min-width: auto;
    }
    
    .fact-badge {
        margin-left: 0;
        align-self: flex-start;
    }
}
</style>