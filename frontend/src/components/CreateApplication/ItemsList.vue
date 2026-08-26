<template>
  <div class="data__list">
    <div class="header-with-badge">
      <h4>Список ТМЦ</h4>
      <span class="items-badge">{{ items.length }}</span>
    </div>
    <div class="items-table rt-table">
      <div class="table-header rt-head-row">
        <div
          class="header-col number-col"
          @click="$emit('sort', 'number')"
        >
          <p :class="{ 'active-sort': sortField === 'number' }">
            №
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'number' && sortDirection === 'desc'
            }"
          />
        </div>
        <div
          class="header-col name-col"
          @click="$emit('sort', 'name')"
        >
          <p :class="{ 'active-sort': sortField === 'name' }">
            Наименование
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'name' && sortDirection === 'desc'
            }"
          />
        </div>
        <div
          class="header-col quantity-col"
          @click="$emit('sort', 'quantity')"
        >
          <p :class="{ 'active-sort': sortField === 'quantity' }">
            Количество
          </p>
          <AppIcon
            name="sort"
            class="sort-icon"
            :class="{
              'desc': sortField === 'quantity' && sortDirection === 'desc'
            }"
          />
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
            class="table-row rt-row"
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
            <!-- Разметка одна на обе раскладки: на десктопе действия остаются иконками
                 в колонке, на телефоне показываются подписи, иконки прячутся, а ряд
                 уходит подвалом карточки (см. @media ниже). -->
            <div class="table-col actions-col">
              <button
                class="edit-btn"
                title="Редактировать"
                @click="$emit('edit-item', item)"
              >
                <AppIcon
                  name="edit"
                  class="edit-icon"
                />
                <span class="act-label">Изменить</span>
              </button>
              <button
                class="delete-btn"
                title="Удалить"
                @click="deleteItemWithAnimation(item.id)"
              >
                <AppIcon
                  name="trashcan"
                  class="delete-icon"
                />
                <span class="act-label">Удалить</span>
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
import AppIcon from '@/components/icons/AppIcon.vue';
export default {
    name: 'ItemsList',
    components: { AppIcon },
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
    background: var(--accent);
    color: var(--accent-contrast);
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
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 1px 3px var(--shadow-drop);
}

.table-header {
    display: flex;
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    padding: 10px 12px;
    font-weight: 500;
    color: var(--text-muted);
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
    color: var(--text);
}

.header-col:hover .sort-icon,
.header-col.active-sort .sort-icon {
    opacity: 0.8;
}

.sort-icon {
    color: var(--text-muted);
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

/* Высота и прокрутка как в списке машин: не меньше 180px, дальше внутренняя прокрутка.
   Прежние 300px делали список ТМЦ выше остальных на той же форме. */
.table-body {
    flex: 1;
    min-height: 180px;
    overflow-y: auto;
    background: var(--surface);
    border-bottom-left-radius: 20px;
    border-bottom-right-radius: 20px;
    scrollbar-width: thin;
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
    border-bottom: 1px solid var(--surface-2);
    align-items: center;
    font-size: 13px;
    transition: all 0.2s ease;
    position: relative;
}

.table-row:last-child {
    border-bottom: none;
}

.table-row:hover {
    background: var(--surface-2);
}

.header-col, .table-col {
    padding: 0 4px;
}

.number-col {
    /* 10% - как столбец нумерации в списках машин и сотрудников на той же форме */
    width: 10%;
    text-align: center;
}

.name-col {
    width: 53%;
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

/* Подпись действия видна только на телефоне - на десктопе кнопка остаётся иконкой. */
.act-label {
    display: none;
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
    background: var(--success-bg);
}

.delete-btn:hover {
    background: var(--danger-bg);
}

.edit-icon, .delete-icon {
  /* Значок мельче 16px: общая обводка 1.7 садится в волосок, здесь плотнее. */
  stroke-width: 2.2;
    color: var(--text);
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
    color: var(--text-muted);
    font-size: 13px;
    font-style: italic;
}

h4 {
    font-size: 16px;
    color: var(--text);
    font-weight: 600;
    margin: 0;
}

/* Scrollbar styling */
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
/* Мобилка: строки становятся карточками (rt-* из responsive-tables.css). Подписи
   полей не выводим - решение по эпику: карточки без лейблов, как в Центре; голое
   количество читалось бы двусмысленно, поэтому у него знак умножения.
   Брейкпоинт 767.98 - как у инфраструктуры. */
@media (max-width: 767.98px) {
    .items-table {
        border: none;
        border-radius: 0;
        box-shadow: none;
        /* Только по Y: по X инфраструктура держит свой overflow-x: hidden. */
        overflow-y: visible;
    }

    /* Список больше не скроллится внутри 300px - страница скроллит сама. */
    .table-body {
        max-height: none;
        overflow-y: visible;
        background: transparent;
    }

    .table-row.rt-row {
        position: relative;
        flex-direction: row !important;
        flex-wrap: wrap;
        align-items: center;
        gap: 2px 8px;
        min-height: 56px;
        /* Резерва справа больше нет: действия уехали в подвал карточки, и наименование
           занимает всю ширину (было 92px под две иконки поперёк строки). */
        padding: 10px 12px !important;
        font-size: 14px;
    }

    .table-col {
        width: auto !important;
        padding: 0;
        text-align: left;
    }

    .number-col {
        color: var(--text-muted);
        font-size: 12px;
    }

    .name-col {
        font-weight: 600;
        font-size: 15px;
    }

    .quantity-col {
        flex-basis: 100%;
        color: var(--text-muted);
        font-size: 13px;
    }

    .quantity-col::before {
        content: 'x ';
    }

    /* Подвал карточки: действия бейджами под данными, а не поперёк строки. */
    .actions-col {
        position: static;
        transform: none;
        flex-basis: 100%;
        width: auto !important;
        justify-content: flex-start;
        gap: 6px;
        margin-top: 8px;
        padding-top: 8px;
        border-top: 1px solid color-mix(in srgb, var(--border) 60%, var(--surface));
    }

    /* Высота 28px как у бейджа, зона нажатия 44px невидимым ::before (мокап .act). */
    .edit-btn,
    .delete-btn {
        position: relative;
        width: auto;
        height: 28px;
        padding: 0 10px;
        border: 1px solid var(--border);
        border-radius: var(--radius-pill, 999px);
        background: var(--surface);
        font-size: 12.5px;
        font-weight: 600;
        line-height: 1;
        white-space: nowrap;
    }

    .edit-btn::before,
    .delete-btn::before {
        content: '';
        position: absolute;
        inset: -8px -2px;
    }

    .edit-btn {
        border-color: var(--accent);
        color: var(--accent-text);
    }

    .delete-btn {
        border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
        color: var(--danger-text);
    }

    .edit-btn:hover,
    .delete-btn:hover {
        background: var(--surface-2);
    }

    /* Подпись вместо иконки: текст в кнопке читается без догадок. */
    .act-label {
        display: inline;
    }

    .edit-icon,
    .delete-icon {
        display: none;
    }
}
</style>