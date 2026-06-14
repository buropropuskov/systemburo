<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список ТМЦ</h4>
      <span class="items-badge">{{ items.length }}</span>
    </div>
    <div class="items-table">
      <div class="table-header">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'number' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col name-col"
          @click="$emit('sort', 'name')"
        >
          <p :class="{ 'active-sort': sortField === 'name' }">
            Наименование
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'name' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div
          class="header-col quantity-col"
          @click="$emit('sort', 'quantity')"
        >
          <p :class="{ 'active-sort': sortField === 'quantity' }">
            Количество
          </p>
          <img 
            src="@/assets/icons/sort.png" 
            class="sort-icon" 
            :class="{ 
              'desc': sortField === 'quantity' && sortDirection === 'desc'
            }" 
          >
        </div>
        <div class="header-col actions-col">
          Действия
        </div>
      </div>
      <div class="table-body">
        <transition-group
          name="fade"
          tag="div"
        >
          <div 
            v-for="(item, index) in items" 
            :key="item.id"
            class="table-row"
          >
            <div class="table-col number-col">
              {{ index + 1 }}
            </div>
            <div class="table-col name-col">
              {{ item.itemName || 'Не указано' }}
            </div>
            <div class="table-col quantity-col">
              {{ item.quantity || 0 }}
            </div>
            <div class="table-col actions-col">
              <button 
                class="edit-btn"
                title="Редактировать"
                @click="$emit('edit-item', item)"
              >
                <img 
                  src="@/assets/icons/edit.png" 
                  alt="Редактировать" 
                  class="edit-icon"
                >
              </button>
              <button 
                class="delete-btn"
                @click="deleteItemWithAnimation(item.id)"
              >
                <img 
                  src="@/assets/icons/trashcan.png" 
                  alt="Удалить" 
                  class="delete-icon"
                >
              </button>
            </div>
          </div>
        </transition-group>
        <div
          v-if="items.length === 0"
          class="no-items"
        >
          Нет добавленных ТМЦ
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
    name: 'ItemsList',
    props: {
        items: { type: Array, default: () => [] },
        sortField: { type: String, default: null },
        sortDirection: { type: String, default: null }
    },
    emits: ['sort', 'edit-item', 'delete-item'],
    methods: {
        deleteItemWithAnimation(itemId) {
            // Создаем анимацию удаления
            const itemElement = document.querySelector(`[data-item-id="${itemId}"]`);
            if (itemElement) {
                itemElement.classList.add('deleting');
                
                // Ждем окончания анимации перед удалением
                setTimeout(() => {
                    this.$emit('delete-item', itemId);
                }, 300);
            } else {
                this.$emit('delete-item', itemId);
            }
        }
    }
}
</script>

<style scoped>
.data__list {
    padding: 12px;
    flex: 1;
}

.header-with-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
}

.items-badge {
    background: #1976d2;
    color: white;
    padding: 2px 6px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    min-width: 18px;
    text-align: center;
    line-height: 1.2;
}

/* Items table styles */
.items-table {
    width: 100%;
    border: 1px solid #e0e0e0;
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.table-header {
    display: flex;
    background: #f5f5f5;
    border-bottom: 1px solid #e0e0e0;
    padding: 10px 12px;
    font-weight: 500;
    color: #666;
    font-size: 13px;
}

.header-col {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    transition: all 0.2s ease;
    user-select: none;
}

.header-col:hover,
.header-col.active-sort {
    color: #333;
}

.header-col:hover .sort-icon,
.header-col.active-sort .sort-icon {
    opacity: 0.8;
}

.sort-icon {
    width: 10px;
    height: 10px;
    transition: all 0.2s ease;
    opacity: 0.4;
    transform: rotate(0deg);
}

.sort-icon.desc {
    transform: rotate(180deg);
    opacity: 0.8;
}

.table-body {
    max-height: 300px;
    overflow-y: auto;
    background: #fff;
    position: relative;
}

/* Анимации для списка */
.fade-enter-active, .fade-leave-active {
    transition: all 0.3s ease;
}

.fade-enter-from, .fade-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.fade-leave-active {
    position: absolute;
    width: 100%;
}

/* Анимация удаления */
.table-row.deleting {
    animation: deleteAnimation 0.3s ease forwards;
}

@keyframes deleteAnimation {
    0% {
        opacity: 1;
        transform: scale(1);
    }
    50% {
        opacity: 0.5;
        transform: scale(0.95);
    }
    100% {
        opacity: 0;
        transform: scale(0.9);
        height: 0;
        margin: 0;
        padding: 0;
        border: none;
    }
}

.table-row {
    display: flex;
    padding: 8px 12px;
    border-bottom: 1px solid #f5f5f5;
    align-items: center;
    font-size: 13px;
    transition: all 0.2s ease;
    position: relative;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: #f8f9fa;
}

.header-col, .table-col {
    padding: 0 4px;
}

.number-col {
    width: 8%;
    text-align: center;
}

.name-col {
    width: 55%;
}

.quantity-col {
    width: 25%;
    text-align: center;
}

.actions-col {
    width: 12%;
    text-align: center;
    display: flex;
    justify-content: center;
    gap: 4px;
}

.edit-btn, .delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.2s ease;
}

.edit-btn:hover {
    background: #e8f5e8;
}

.delete-btn:hover {
    background: #ffebee;
}

.edit-icon, .delete-icon {
    width: 14px;
    height: 14px;
    opacity: 0.6;
    transition: opacity 0.2s ease;
}

.edit-btn:hover .edit-icon {
    opacity: 0.9;
}

.delete-btn:hover .delete-icon {
    opacity: 0.9;
}

.no-items {
    text-align: center;
    padding: 16px;
    color: #666;
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: #333;
    font-weight: 600;
    margin: 0;
}

/* Scrollbar styling */
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
</style>