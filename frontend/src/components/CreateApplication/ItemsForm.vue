<template>
  <div
    class="data__completion"
    :class="{ 'data__completion--locked': disabled }"
    :inert="disabled ? '' : undefined"
  >
    <div class="completion__header">
      <div
        class="completion__actions"
        @click="revealBlockedHint($event)"
      >
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
      <!-- В DOM заголовок после кнопок: на мобилке шапка разворачивается в поток,
           на десктопе order возвращает его влево. -->
      <h3 class="completion__title">
        Новые ТМЦ
      </h3>
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
          @click="togglePlace(place, $event)"
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
import { useFieldConfig } from '@/composables/useFieldConfig'
import { useNarrowScreen } from '@/composables/useNarrowScreen';
import { getViewportZoom } from '@/utils/viewportScale';

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
        // Причина блокировки кнопки живёт на hover - на телефоне его нет,
        // поэтому там показываем её сразу под кнопкой.
        const { isNarrow } = useNarrowScreen();
        return { ...useFieldConfig(() => props.fieldConfig), isNarrow };
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
    beforeUnmount() {
        if (this.hintTimer) clearTimeout(this.hintTimer);
        if (this.inactiveTooltipTimer) clearTimeout(this.inactiveTooltipTimer);
    },
    methods: {
        /**
         * Причина блокировки на телефоне показывается по тапу на зону кнопки
         * (сама кнопка disabled и события не даёт - на мобилке она прозрачна для
         * тапа через pointer-events) и гаснет сама.
         */
        revealBlockedHint(event) {
            // Тап по «Отменить» в режиме редактирования - не повод объяснять,
            // почему заблокировано добавление.
            if (event && event.target.closest('.cancel-edit-btn')) return;
            if (!this.isNarrow || this.canAddItems) return;
            this.showTooltip = true;
            if (this.hintTimer) clearTimeout(this.hintTimer);
            this.hintTimer = setTimeout(() => { this.showTooltip = false; }, 3000);
        },

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

        togglePlace(place, event) {
            if (place.status !== 'active') {
                // На телефоне hover не наступает, и причина недоступности была недостижима:
                // показываем её по тапу и гасим сама через пару секунд.
                this.showInactiveTooltip(place, event);
                if (this.inactiveTooltipTimer) clearTimeout(this.inactiveTooltipTimer);
                this.inactiveTooltipTimer = setTimeout(() => this.hideInactiveTooltip(), 2500);
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
                    // Тултип position:fixed внутри зазумленного <html>: rect в device-px,
                    // а inline left/top трактуются как layout-px - делим на zoom, иначе
                    // подсказка уезжает вправо-вниз. Отступ -10 уже в layout-px.
                    const z = getViewportZoom();
                    const rect = event.target.getBoundingClientRect();
                    this.inactiveTooltip.x = (rect.left + rect.width / 2) / z;
                    this.inactiveTooltip.y = rect.top / z - 10;
                });
            }
        },

        hideInactiveTooltip() {
            if (this.inactiveTooltipTimer) {
                clearTimeout(this.inactiveTooltipTimer);
                this.inactiveTooltipTimer = null;
            }
            this.inactiveTooltip.visible = false;
        }
    }
}
</script>

<style scoped>
.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid var(--border);
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

.completion__title {
    order: -1;
}

.completion__header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
}

.completion__actions {
    display: flex;
    gap: 10px;
    align-items: center;
    position: relative;
}

.cancel-edit-btn {
    background: var(--surface-2);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.cancel-edit-btn:hover {
    background: var(--row-hover);
}

.add-button {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
    position: relative;
}

.add-button:hover:not(:disabled) {
    background: var(--accent-hover);
}

.add-button:disabled {
    background: var(--text-muted);
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
    color: var(--text-muted);
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
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
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
    background: var(--surface-2);
}

.table-body::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 2px;
}

.table-body::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
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
    color: var(--text-muted);
    background: var(--surface-2);
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
    border: 1px solid var(--border);
    border-radius: 15px;
    padding: 0 15px;
    outline: none;
    font-size: 14px;
    background: var(--surface);
    transition: all 0.3s ease;
}

.name__input:focus {
    border-color: var(--accent);
}

.name__input.input--error {
    border-color: var(--danger);
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
    background: var(--surface);
    color: var(--accent-text);
    border: 1px solid var(--accent);
    border-radius: 15px;
    padding: 6px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.2s;
}

.add-row-btn:hover {
    background: var(--accent-tint);
}

.total-items {
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
}

.remove-row-btn {
    background: none;
    border: none;
    color: var(--danger-text);
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
    background: var(--danger-bg);
}

.remove-row-btn:disabled {
    color: var(--text-muted);
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
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 420px;
    min-width: 420px;
    box-shadow: 0 2px 8px var(--shadow-drop);
}

.tooltip-content::before {
    content: '';
    position: absolute;
    bottom: 100%;
    right: 40px;
    border: 5px solid transparent;
    border-bottom-color: var(--hint-bg);
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
    background: var(--surface-2);
    color: var(--text-muted);
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: background-color 0.2s, color 0.2s, border-color 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.unloading__item:hover:not(.unloading__item--active):not(.unloading__item--inactive) {
    background: var(--row-hover);
}

.unloading__item--active {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.unloading__item--inactive {
    background: var(--danger-bg);
    color: var(--danger-text);
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
    cursor: not-allowed;
    opacity: 0.7;
}

.no-places-message {
    font-size: 12px;
    color: var(--danger-text);
    text-align: center;
    padding: 20px;
    background: var(--danger-bg);
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
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;
    box-shadow: 0 2px 8px var(--shadow-drop);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: var(--hint-bg);
}

/* Форма (450px) + список ТМЦ рядом не влезают на планшете - стекаем в колонку
   (form__data в CreateApplication.vue делает то же на этом же брейкпоинте). */
@media (max-width: 1024px) {
    .data__completion {
        width: 100%;
        border-right: none;
        border-bottom: 1px solid var(--border);
    }
}

@media (max-width: 768px) {
    /* Подсказка поверх НАД кнопкой: в потоке она двигала форму. Контейнер
       кнопок - её positioned-родитель. */
    .tooltip {
        position: absolute;
        top: auto;
        bottom: calc(100% + 10px);
        right: 0;
        left: auto;
        margin: 0;
        width: min(320px, calc(100vw - 44px));
        z-index: 1100;
    }

    .tooltip-content::before {
        display: none;
    }

    /* Тап по заблокированной кнопке уходит контейнеру и показывает причину. */
    .add-button:disabled {
        pointer-events: none;
    }


    /* Кнопка «Добавить» - внизу формы, куда пользователь приходит, заполнив
       таблицу; заголовок «Новые ТМЦ» остаётся в потоке сверху. */
    .data__completion {
        display: flex;
        flex-direction: column;
    }

    .completion__header {
        display: contents;
    }

    /* order:-1 нужен только десктопной строке шапки: во флексе всей формы он
       утаскивал заголовок в самое начало. */
    .completion__title {
        order: 0;
        display: block;
        margin-bottom: 6px;
    }

    .completion__actions {
        order: 999;
        justify-content: stretch;
        margin-top: 4px;
        padding: 4px 0;
    }

    .completion__actions .add-button {
        flex: 1;
        min-height: 44px;
    }

    .completion__actions .cancel-edit-btn {
        min-height: 44px;
    }

    .tooltip-content {
        max-width: 100%;
        white-space: pre-line;
    }

    .tooltip-content {
        min-width: 0;
        max-width: calc(100vw - 40px);
    }

    /* Сетка мест перестраивалась только с 480. */
    .unloading__grid {
        grid-template-columns: repeat(2, 1fr);
        max-width: 100%;
    }

    /* Названия мест обрезались многоточием - пускаем в две строки. */
    .unloading__item {
        height: auto;
        min-height: 36px;
        padding: 6px 10px;
        white-space: normal;
        overflow: visible;
        text-overflow: clip;
        line-height: 1.25;
    }

    /* Таблица ввода: фиксированные колонки съедали 190px из 320, и поле
       наименования схлопывалось. Порядковый номер строки убираем - строк
       единицы, а подписи колонок остаются на месте. */
    .number-header,
    .number-cell {
        display: none;
    }

    .name-header,
    .name-cell {
        margin-right: 8px;
        min-width: 0;
    }

    .quantity-header,
    .quantity-cell {
        width: 64px;
        min-width: 64px;
        max-width: 64px;
        margin-right: 8px;
    }

    .actions-header,
    .actions-cell {
        width: 36px;
        min-width: 36px;
        max-width: 36px;
    }

    .remove-row-btn {
        width: 36px;
        height: 36px;
    }

    .add-button {
        min-height: 40px;
        padding: 8px 18px;
        font-size: 13px;
    }

    .completion__button,
    .add-row-btn {
        min-height: 36px;
    }
}
</style>
