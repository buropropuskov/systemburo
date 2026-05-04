<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      @click="$emit('close')"
    >
      <div
        class="modal-content"
        @click.stop
      >
        <div class="modal-header">
          <div class="modal-header__top">
            <h3>Привязка новых автомобилей</h3>
          </div>
          <button
            class="modal-close"
            @click="$emit('close')"
          >
            ×
          </button>
        </div>
        <div class="modal-body">
          <div class="binding-info">
            <p class="binding-description">
              Все добавленные автомобили ниже <strong>автоматически привязываются</strong> к вашему аккаунту.
              Вы можете выбрать и привязать автомобили к организации и/или компании для использования <strong>другими сотрудниками</strong>:
            </p>
                    
            <div class="cars-list-section">
              <p class="section-title">
                Список новых автомобилей:
              </p>
              <div class="cars-list">
                <div 
                  v-for="car in newCarsToBind" 
                  :key="car.plateNumber"
                  class="car-item"
                  :class="{ 'car-item--shared': car.bindToEntity }"
                  @click="$emit('toggle-car-binding', car)"
                >
                  <div class="car-selector">
                    <div class="selector-checkbox">
                      <div
                        class="checkbox"
                        :class="{ 'checkbox--checked': car.bindToEntity }"
                      />
                    </div>
                    <div class="car-info">
                      <span class="car-number">{{ car.plateNumber }}</span>
                      <span class="car-mark">{{ car.mark }}</span>
                    </div>
                  </div>
                  <div class="car-binding-status">
                    <span
                      v-if="car.bindToEntity"
                      class="status-shared"
                    >
                      Будет доступна
                    </span>
                    <span
                      v-else
                      class="status-private"
                    >
                      Привязка только к вам
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="binding-options-section">
              <p class="section-title">
                Привязать выбранные автомобили к:
              </p>
              <div class="binding-options">
                <label
                  v-if="hasOrganization"
                  class="binding-option"
                >
                  <input 
                    v-model="bindToOrganization" 
                    type="checkbox"
                    :disabled="bindToCompany"
                  >
                  <span class="option-text">К организации "{{ organization }}"</span>
                </label>
                <label
                  v-if="hasCompany"
                  class="binding-option"
                >
                  <input 
                    v-model="bindToCompany" 
                    type="checkbox"
                    :disabled="bindToOrganization"
                  >
                  <span class="option-text">К компании "{{ company }}"</span>
                </label>
              </div>
            </div>

            <div class="warning-section">
              <p class="warning-text">
                <strong class="red">Внимание!</strong> При привязке автомобиля к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании.
              </p>
            </div>
          </div>
                
          <div class="modal-actions">
            <button
              class="cancel-btn"
              @click="$emit('skip-binding')"
            >
              Пропустить
            </button>
            <button
              class="confirm-btn"
              @click="$emit('confirm-binding')"
            >
              Привязать и отправить
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
export default {
    name: 'BindingModal',
    props: {
        newCarsToBind: { type: Array, default: () => [] },
        organization: { type: String, default: null },
        company: { type: String, default: null },
        hasOrganization: Boolean,
        hasCompany: Boolean
    },
    emits: ['toggle-car-binding', 'confirm-binding', 'skip-binding', 'close'],
    data() {
        return {
            bindToOrganization: false,
            bindToCompany: false
        }
    },
    watch: {
        bindToOrganization(newVal) {
            if (newVal) {
                this.bindToCompany = false;
            }
        },
        
        bindToCompany(newVal) {
            if (newVal) {
                this.bindToOrganization = false;
            }
        }
    }
}
</script>

<style scoped>
/* Модальное окно привязки - супер минималистичный дизайн */
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
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
    background: white;
    border-radius: 20px;
    padding: 0;
    width: 500px;
    max-width: 90vw;
    max-height: 80vh;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 15px;
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
    max-height: 60vh;
    overflow-y: auto;
}

.binding-info {
    margin-bottom: 20px;
}

.binding-description {
    font-size: 14px;
    line-height: 1.5;
    color: #666;
    margin-bottom: 20px;
    text-align: left;
}

.section-title {
    font-size: 14px;
    font-weight: 600;
    color: #333;
    margin-bottom: 10px;
}

.cars-list-section {
    margin-bottom: 25px;
}

.cars-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.car-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 15px;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    transition: all 0.2s;
    cursor: pointer;
}

.car-item:hover {
    border-color: #4F5BDF;
}

.car-item--shared {
    background: #f8f9ff;
    border-color: #4F5BDF;
}

.car-selector {
    display: flex;
    align-items: center;
    gap: 12px;
}

.selector-checkbox {
    display: flex;
    align-items: center;
    justify-content: center;
}

.checkbox {
    width: 18px;
    height: 18px;
    border: 2px solid #e6e6e6;
    border-radius: 4px;
    transition: all 0.2s;
    position: relative;
}

.checkbox--checked {
    background: #4F5BDF;
    border-color: #4F5BDF;
}

.checkbox--checked::after {
    content: "✓";
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: white;
    font-size: 12px;
    font-weight: bold;
}

.car-info {
    display: flex;
    align-items: center;
    gap: 15px;
}

.car-number {
    font-weight: 600;
    color: #333;
    font-size: 14px;
}

.car-mark {
    color: #666;
    font-size: 13px;
}

.car-binding-status {
    font-size: 12px;
    font-weight: 500;
}

.status-shared {
    color: #4F5BDF;
    display: flex;
    align-items: center;
    gap: 5px;
}

.status-private {
    color: #666;
    display: flex;
    align-items: center;
    gap: 5px;
}

.status-icon {
    font-size: 14px;
}

.binding-options-section {
    margin-bottom: 20px;
    padding-top: 20px;
    border-top: 1px solid #e6e6e6;
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
    padding: 5px 0;
}

.binding-option input[type="checkbox"] {
    width: 14px;
    height: 14px;
    cursor: pointer;
}

.option-text {
    color: #333;
}

.warning-section {
     
}

.warning-text {
    font-size: 11px;
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

.cancel-btn {
    background: white;
    color: #666;
    border: 1px solid #e6e6e6;
    border-radius: 12px;
    padding: 10px 20px;
    font-size: 14px;
    cursor: pointer;
    transition: all 0.2s;
}

.cancel-btn:hover {
    background: #f5f5f5;
    border-color: #ccc;
}

.confirm-btn {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 12px;
    padding: 10px 20px;
    font-size: 14px;
    cursor: pointer;
    transition: background-color 0.2s;
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
    }
    
    .modal-actions {
        flex-direction: column;
    }
    
    .cancel-btn,
    .confirm-btn {
        width: 100%;
    }
    
    .car-item {
        flex-direction: column;
        align-items: flex-start;
        gap: 8px;
    }
    
    .car-selector {
        width: 100%;
        justify-content: space-between;
    }
}
</style>