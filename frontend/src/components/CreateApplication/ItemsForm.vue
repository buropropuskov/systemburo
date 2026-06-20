<template>
  <div
    class="data__completion"
    :class="{ 'data__completion--locked': disabled }"
    :inert="disabled ? '' : undefined"
  >
    <div class="completion__header">
      <h3>Новые ТМЦ</h3>
      <div class="completion__actions">
        <button 
          v-if="editingItem" 
          class="cancel-edit-btn" 
          @click="cancelEdit"
        >
          Отменить
        </button>
        <button 
          class="add-button" 
          :disabled="!canAddItems"
          @click="addItems"
          @mouseenter="showTooltip = true"
          @mouseleave="showTooltip = false"
        >
          {{ editingItem ? 'Применить' : 'Добавить' }}
        </button>
        <!-- Подсказка для кнопки -->
        <div
          v-if="showTooltip && !canAddItems"
          class="tooltip"
        >
          <div class="tooltip-content">
            {{ getTooltipMessage }}
          </div>
        </div>
      </div>
    </div>

    <!-- Форма для добавления новых ТМЦ -->
    <div class="items-form-container">
      <!-- Таблица для ввода ТМЦ -->
      <div class="items-table-wrapper">
        <div class="items-table">
          <div class="table-header">
            <div class="header-cell number-header">
              <label class="input__label">№</label>
            </div>
            <div
              v-if="fieldVisible('item_name')"
              class="header-cell name-header"
            >
              <label class="input__label">Наименование ТМЦ <span
                v-if="fieldRequired('item_name')"
                class="required"
              >*</span></label>
            </div>
            <div
              v-if="fieldVisible('quantity')"
              class="header-cell quantity-header"
            >
              <label class="input__label">Количество <span
                v-if="fieldRequired('quantity')"
                class="required"
              >*</span></label>
            </div>
            <div class="header-cell actions-header" />
          </div>
                    
          <div class="table-body">
            <div
              v-for="(item, index) in tempItems"
              :key="item.key"
              class="table-row"
            >
              <div class="table-cell number-cell">
                <span class="item-number">{{ index + 1 }}</span>
              </div>
              <div
                v-if="fieldVisible('item_name')"
                class="table-cell name-cell"
              >
                <input
                  v-model="item.itemName"
                  type="text"
                  placeholder="Введите наименование"
                  class="name__input"
                  :class="{ 'input--error': fieldRequired('item_name') && !item.itemName && submitted }"
                  @input="updateItem(index, $event.target.value, 'itemName')"
                >
              </div>
              <div
                v-if="fieldVisible('quantity')"
                class="table-cell quantity-cell"
              >
                <input
                  v-model.number="item.quantity"
                  type="number"
                  min="1"
                  placeholder="1"
                  class="name__input"
                  :class="{ 'input--error': fieldRequired('quantity') && (!item.quantity || item.quantity < 1) && submitted }"
                  @input="updateItem(index, parseInt($event.target.value) || 1, 'quantity')"
                >
              </div>
              <div class="table-cell actions-cell">
                <button 
                  class="remove-row-btn"
                  :disabled="tempItems.length <= 1"
                  @click="removeRow(index)"
                >
                  ×
                </button>
              </div>
            </div>
          </div>
        </div>
                
        <div class="table-actions">
          <button
            class="add-row-btn"
            @click="addRow"
          >
            + Добавить строку
          </button>
          <div class="total-items">
            Всего позиций: {{ tempItems.length }}
          </div>
        </div>
      </div>
    </div>

    <!-- Места разгрузки (#706): для ТМЦ-без-машин - единственный источник мест.
         Грид 1:1 повторяет форму авто (completion__unloading). -->
    <div
      v-if="showUnloadPlaces"
      class="completion__unloading"
    >
      <label class="input__label">Места разгрузки (выбор) <span class="required">*</span></label>
      <div
        v-if="allUnloadingPlaces.length > 0"
        class="unloading__grid"
      >
        <div
          v-for="place in allUnloadingPlaces"
          :key="place.id"
          class="unloading__item"
          :class="{
            'unloading__item--active': selectedUnloadPlaces.includes(place.id) && place.status === 'active',
            'unloading__item--inactive': place.status !== 'active'
          }"
          @click="togglePlace(place)"
          @mouseenter="showInactiveTooltip(place, $event)"
          @mouseleave="hideInactiveTooltip"
        >
          {{ place.name }}
        </div>
      </div>
      <div
        v-else
        class="no-places-message"
      >
        Нет доступных мест разгрузки
      </div>
    </div>

    <!-- Tooltip для неактивных мест -->
    <div
      v-if="inactiveTooltip.visible"
      class="inactive-tooltip"
      :style="{ top: inactiveTooltip.y + 'px', left: inactiveTooltip.x + 'px' }"
    >
      <div class="inactive-tooltip-content">
        {{ inactiveTooltip.text }}
      </div>
    </div>
  </div>
</template>

<script>
import { useFieldConfig } from '@/composables/useFieldConfig';

export default {
    name: 'ItemsForm',
    props: {
        existingItems: {
            type: Array,
            default: () => []
        },
        // Настройка полей шаблона (#529): { [fieldKey]: { visible, required, locked, requirable } }.
        // Раздаётся из CreateApplication; fieldVisible/fieldRequired через useFieldConfig.
        fieldConfig: {
            type: Object,
            default: () => ({})
        },
        // Гейт п.36: форма недоступна, пока не заполнены обязательные поля вложения.
        disabled: {
            type: Boolean,
            default: false
        },
        // Место разгрузки на уровне заявки (#706): для ТМЦ-без-машин это единственный
        // источник мест. Показываем грид только когда машин в заявке нет.
        showUnloadPlaces: {
            type: Boolean,
            default: false
        },
        allUnloadingPlaces: {
            type: Array,
            default: () => []
        },
        selectedUnloadPlaces: {
            type: Array,
            default: () => []
        }
    },
    emits: ['edit-cancelled', 'item-added', 'item-updated', 'items-added', 'update:unload-places'],
    setup(props) {
        return useFieldConfig(() => props.fieldConfig);
    },
    data() {
        return {
            tempItems: [
                { 
                    key: this.generateKey(),
                    itemName: '', 
                    quantity: 1 
                }
            ],
            editingItem: null,
            showTooltip: false,
            submitted: false,
            tempItemsBackup: null,
            isAddingRow: false,
            inactiveTooltip: {
                visible: false,
                text: '',
                x: 0,
                y: 0
            }
        }
    },
    computed: {
        canAddItems() {
            const nameVisible = this.fieldVisible('item_name');
            const quantityVisible = this.fieldVisible('quantity');
            const nameRequired = this.fieldRequired('item_name');
            const quantityRequired = this.fieldRequired('quantity');
            return this.tempItems.every(item => {
                const nameOk = !nameVisible || !nameRequired || (item.itemName && item.itemName.trim());
                const quantityOk = !quantityVisible || !quantityRequired || (item.quantity && item.quantity >= 1);
                return nameOk && quantityOk;
            }) && this.tempItems.length > 0;
        },
        getTooltipMessage() {
            if (this.tempItems.length === 0) {
                return 'Добавьте хотя бы одну ТМЦ';
            }
            const nameVisible = this.fieldVisible('item_name');
            const quantityVisible = this.fieldVisible('quantity');
            const nameRequired = this.fieldRequired('item_name');
            const quantityRequired = this.fieldRequired('quantity');
            const invalidItems = this.tempItems.filter(item => {
                const nameInvalid = nameVisible && nameRequired && (!item.itemName || !item.itemName.trim());
                const quantityInvalid = quantityVisible && quantityRequired && (!item.quantity || item.quantity < 1);
                return nameInvalid || quantityInvalid;
            });
            if (invalidItems.length > 0) {
                return 'Заполните все обязательные поля (Наименование ТМЦ и количество)';
            }
            return '';
        }
    },
    watch: {
        // Следим за изменениями в tempItems и сбрасываем submitted при изменении
        tempItems: {
            handler() {
                if (this.submitted) {
                    this.submitted = false;
                }
            },
            deep: true
        }
    },
    methods: {
        generateKey() {
            return Date.now() + Math.random().toString(36).substr(2, 9);
        },
        
        addRow() {
            this.isAddingRow = true;
            this.tempItems.push({ 
                key: this.generateKey(),
                itemName: '', 
                quantity: 1 
            });
            // Сбрасываем флаг после обновления DOM
            this.$nextTick(() => {
                this.isAddingRow = false;
            });
        },
        
        removeRow(index) {
            // Устанавливаем анимацию удаления для этой строки
            const row = document.querySelector(`.table-row:nth-child(${index + 1})`);
            if (row) {
                row.classList.add('removing');
                
                // Ждем окончания анимации перед удалением
                setTimeout(() => {
                    this.tempItems.splice(index, 1);
                }, 300);
            } else {
                this.tempItems.splice(index, 1);
            }
        },
        
        updateItem(index, value, field) {
            this.tempItems[index][field] = value;
        },
        
        addItems() {
            this.submitted = true;
            
            if (!this.canAddItems) {
                return;
            }
            
            // Валидность строки - тот же конфиг-aware критерий, что и canAddItems:
            // скрытое/необязательное поле не делает строку невалидной. Иначе при
            // скрытом item_name validItems пустел и addItems молча ничего не эмитил.
            const nameVisible = this.fieldVisible('item_name');
            const quantityVisible = this.fieldVisible('quantity');
            const nameRequired = this.fieldRequired('item_name');
            const quantityRequired = this.fieldRequired('quantity');
            const validItems = this.tempItems.filter(item => {
                const nameOk = !nameVisible || !nameRequired || (item.itemName && item.itemName.trim());
                const quantityOk = !quantityVisible || !quantityRequired || (item.quantity && item.quantity >= 1);
                return nameOk && quantityOk;
            });

            if (validItems.length === 0) {
                return;
            }
            
            // Подготовка данных без ключей
            const preparedItems = validItems.map(item => ({
                itemName: item.itemName,
                quantity: item.quantity
            }));
            
            if (this.editingItem) {
                // Редактирование существующей ТМЦ
                const updatedItem = {
                    ...preparedItems[0],
                    id: this.editingItem.id
                };
                this.$emit('item-updated', updatedItem);
                
                // После успешного редактирования ОЧИЩАЕМ backup
                // чтобы при следующем редактировании не восстанавливалось
                this.tempItemsBackup = null;
                this.editingItem = null;
            } else {
                // Добавление новых ТМЦ
                if (preparedItems.length === 1) {
                    this.$emit('item-added', preparedItems[0]);
                } else {
                    this.$emit('items-added', preparedItems);
                }
                this.clearForm();
            }
        },
        
        clearForm() {
            this.tempItems = [{ 
                key: this.generateKey(),
                itemName: '', 
                quantity: 1 
            }];
            this.submitted = false;
            this.tempItemsBackup = null;
        },
        
        editItem(item) {
            // Сохраняем ТЕКУЩИЕ временные данные
            this.tempItemsBackup = JSON.parse(JSON.stringify(this.tempItems));
            
            // Устанавливаем редактируемый элемент
            this.editingItem = item;
            this.tempItems = [{
                key: this.generateKey(),
                itemName: item.itemName || '',
                quantity: item.quantity || 1
            }];
        },
        
        cancelEdit() {
            // Восстанавливаем временные данные ИЗ backup
            if (this.tempItemsBackup && this.tempItemsBackup.length > 0) {
                this.tempItems = JSON.parse(JSON.stringify(this.tempItemsBackup));
            } else {
                // Если backup пустой, очищаем форму до одной строки
                this.clearForm();
            }
            
            // Очищаем редактирование
            this.editingItem = null;
            this.tempItemsBackup = null;
            
            // Эмитируем событие отмены
            this.$emit('edit-cancelled');
        },

        togglePlace(place) {
            if (place.status !== 'active') {
                return;
            }
            const current = this.selectedUnloadPlaces || [];
            const next = current.includes(place.id)
                ? current.filter(id => id !== place.id)
                : [...current, place.id];
            this.$emit('update:unload-places', next);
        },

        showInactiveTooltip(place, event) {
            if (place.status !== 'active') {
                this.inactiveTooltip.text = place.status_comment
                    ? `Недоступно: ${place.status_comment}`
                    : 'Недоступно';
                this.inactiveTooltip.visible = true;

                this.$nextTick(() => {
                    const rect = event.target.getBoundingClientRect();
                    this.inactiveTooltip.x = rect.left + rect.width / 2;
                    this.inactiveTooltip.y = rect.top - 10;
                });
            }
        },

        hideInactiveTooltip() {
            this.inactiveTooltip.visible = false;
        }
    }
}
</script>

<style scoped>
.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid #e6e6e6;
}

.data__completion--locked {
    position: relative;
}

.completion__header {
    padding-bottom: 15px;
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.completion__header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #333;
}

.completion__actions {
    display: flex;
    gap: 10px;
    align-items: center;
    position: relative;
}

.cancel-edit-btn {
    background: #f8f8f8;
    color: #333;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.cancel-edit-btn:hover {
    background: #e8e8e8;
}

.add-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
    position: relative;
}

.add-button:hover:not(:disabled) {
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.items-form-container {
    display: flex;
    flex-direction: column;
}

.items-table-wrapper {
    border-radius: 10px;
    overflow: hidden;
}

.items-table {
    width: 100%;
}

.table-header {
    display: flex;
    background: transparent;
    border-bottom: none;
    padding: 0;
    font-weight: normal;
    color: #a2a2a2;
    font-size: 13px;
    margin-bottom: 10px;
}

.header-cell {
    padding: 0;
    display: flex;
    align-items: center;
}

.number-header {
    width: 40px;
    min-width: 40px;
    max-width: 40px;
    margin-right: 10px;
    justify-content: center;
}

.name-header {
    flex: 1;
    margin-right: 10px;
}

.quantity-header {
    width: 90px;
    min-width: 90px;
    max-width: 90px;
    margin-right: 10px;
}

.actions-header {
    width: 40px;
    min-width: 40px;
    max-width: 40px;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}

.table-body {
    max-height: 300px;
    overflow: auto;
    transition: max-height 0.3s ease;
}

/* Стили для плавной прокрутки при добавлении элементов */
.table-body::-webkit-scrollbar {
    width: 4px;
}

.table-body::-webkit-scrollbar-track {
    background: #f1f1f1;
}

.table-body::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

.table-row {
    display: flex;
    margin-bottom: 10px;
    align-items: center;
    transition: all 0.3s ease;
}

.table-row:last-child {
    margin-bottom: 0;
}

/* Анимация удаления строк */
.table-row.removing {
    animation: slideOutLeft 0.3s ease forwards;
}

@keyframes slideOutLeft {
    0% {
        opacity: 1;
        transform: translateX(0);
        max-height: 40px;
        margin-bottom: 10px;
    }
    100% {
        opacity: 0;
        transform: translateX(-100%);
        max-height: 0;
        margin-bottom: 0;
        padding: 0;
    }
}

.table-cell {
    padding: 0;
    display: flex;
    align-items: center;
    height: 40px;
    transition: all 0.3s ease;
}

.number-cell {
    width: 40px;
    min-width: 40px;
    max-width: 40px;
    margin-right: 10px;
    justify-content: center;
}

.name-cell {
    flex: 1;
    margin-right: 10px;
}

.quantity-cell {
    width: 80px;
    min-width: 80px;
    max-width: 80px;
    margin-right: 10px;
}

.actions-cell {
    width: 40px;
    min-width: 40px;
    max-width: 40px;
    display: flex;
    justify-content: center;
}

.item-number {
    font-size: 13px;
    font-weight: 500;
    color: #666;
    background: #f8f9fa;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
}

.name__input {
    width: 100%;
    height: 40px;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    padding: 0 15px;
    outline: none;
    font-size: 14px;
    background: #FFF;
    transition: all 0.3s ease;
}

.name__input:focus {
    border-color: #4F5BDF;
}

.name__input.input--error {
    border-color: #ff4444;
}

.name__input[type="number"] {
    text-align: center;
    padding: 0 10px;
}

.table-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 0;
    margin-top: 5px;
}

.add-row-btn {
    background: white;
    color: #4F5BDF;
    border: 1px solid #4F5BDF;
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.2s;
}

.add-row-btn:hover {
    background: #f0f2ff;
}

.total-items {
    font-size: 12px;
    color: #666;
    font-weight: 500;
}

.remove-row-btn {
    background: none;
    border: none;
    color: #ff4444;
    cursor: pointer;
    font-size: 18px;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s;
}

.remove-row-btn:hover:not(:disabled) {
    background: #ffebee;
}

.remove-row-btn:disabled {
    color: #ccc;
    cursor: not-allowed;
}

/* Tooltip styles */
.tooltip {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 5px;
    z-index: 1000;
}

.tooltip-content {
    background: #333;
    color: white;
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 420px;
    min-width: 420px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.tooltip-content::before {
    content: '';
    position: absolute;
    bottom: 100%;
    right: 40px;
    border: 5px solid transparent;
    border-bottom-color: #333;
}

/* Места разгрузки (#706): грид 1:1 из VehicleForm (completion__unloading). */
.completion__unloading {
    margin-top: 15px;
}

.unloading__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
    position: relative;
}

.unloading__item {
    height: 30px;
    background: #F2F2F2;
    color: #a2a2a2;
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.unloading__item:hover:not(.unloading__item--active):not(.unloading__item--inactive) {
    background: #e8e8e8;
}

.unloading__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
}

.unloading__item--inactive {
    background: #ffe6e6;
    color: #ff6b6b;
    border-color: #ffcccc;
    cursor: not-allowed;
    opacity: 0.7;
}

.no-places-message {
    font-size: 12px;
    color: #ff6b6b;
    text-align: center;
    padding: 20px;
    background: #fff5f5;
    border-radius: 8px;
    margin-top: 10px;
}

.inactive-tooltip {
    position: fixed;
    transform: translateX(-50%) translateY(-100%);
    z-index: 10000;
    pointer-events: none;
}

.inactive-tooltip-content {
    background: #333;
    color: white;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: #333;
}
</style>