<template>
    <div class="selector">
        <div 
            v-for="category in categories" 
            :key="category.id"
            class="category"
        >
            <div class="category-header">
                <div class="category-title">{{ category.title }}</div>
            </div>
            
            <div class="attachments-list">
                <div
                    v-for="attachment in getCategoryAttachments(category.id)"
                    :key="attachment.id"
                    class="attachment"
                    :class="{ selected: selectedAttachment?.id === attachment.id }"
                    @click="selectAttachment(attachment)"
                    @mouseenter="handleMouseEnter(attachment, $event)"
                    @mouseleave="handleMouseLeave"
                >
                    <span class="attachment-name">{{ attachment.name }}</span>
                    
                    <button 
                        v-if="hoveredAttachment === attachment.id"
                        class="delete-btn"
                        @click.stop="confirmDelete(attachment)"
                    >
                        ×
                    </button>
                </div>
            </div>

            <button 
                class="add-btn"
                @click="addAttachment(category)"
            >
                Добавить
            </button>
        </div>

        <!-- Modal for delete confirmation -->
        <ConfirmationModal
            :show="showDeleteModal"
            title="Подтверждение удаления"
            :message="deleteMessage"
            confirm-text="Удалить"
            cancel-text="Отмена"
            :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
            @confirm="deleteAttachment"
            @cancel="cancelDelete"
        />

        <!-- Tooltip -->
        <div 
            v-if="showTooltip" 
            class="tooltip"
            :style="tooltipStyle"
        >
            {{ tooltipText }}
        </div>
    </div>
</template>

<script>
import ConfirmationModal from './ConfirmationModal.vue';

export default {
    name: 'AttachmentsSelector',
    components: {
        ConfirmationModal
    },
    data() {
        return {
            categories: [
                { id: 1, title: 'АВТОЗАЯВКИ', template: 'Автозаявка №' },
                { id: 2, title: 'ВВОЗ', template: 'Заявка на ввоз №' },
                { id: 3, title: 'ВЫВОЗ', template: 'Заявка на вывоз №' },
                { id: 4, title: 'ПРОВЕДЕНИЕ РАБОТ', template: 'Заявка на работы №' },
                { id: 5, title: 'РАЗОВЫЕ СПИСКИ', template: 'Разовый пропуск №' }
            ],
            attachments: [],
            selectedAttachment: null,
            hoveredAttachment: null,
            showDeleteModal: false,
            attachmentToDelete: null,
            showTooltip: false,
            tooltipText: '',
            tooltipStyle: {},
            tooltipTimeout: null
        }
    },
    computed: {
        deleteMessage() {
            return `Вы точно хотите удалить "${this.attachmentToDelete?.name}"?`;
        }
    },
    methods: {
        getCategoryAttachments(categoryId) {
            return this.attachments.filter(attachment => attachment.categoryId === categoryId);
        },
        
        getNextAttachmentNumber(categoryId) {
            const categoryAttachments = this.getCategoryAttachments(categoryId);
            return categoryAttachments.length + 1;
        },
        
        addAttachment(category) {
            const nextNumber = this.getNextAttachmentNumber(category.id);
            const newAttachment = {
                id: Date.now() + Math.random(),
                categoryId: category.id,
                name: `${category.template}${nextNumber}`,
                type: category.title.toLowerCase()
            };
            
            this.attachments.push(newAttachment);
            
            // Auto-select the newly created attachment
            this.selectAttachment(newAttachment);
        },
        
        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            // Emit event for parent component
            this.$emit('attachment-selected', attachment);
        },
        
        confirmDelete(attachment) {
            this.attachmentToDelete = attachment;
            this.showDeleteModal = true;
        },
        
        cancelDelete() {
            this.showDeleteModal = false;
            this.attachmentToDelete = null;
        },
        
        deleteAttachment() {
            if (this.attachmentToDelete) {
                // Remove attachment from array
                this.attachments = this.attachments.filter(
                    attachment => attachment.id !== this.attachmentToDelete.id
                );
                
                // Clear selection if deleted attachment was selected
                if (this.selectedAttachment?.id === this.attachmentToDelete.id) {
                    this.selectedAttachment = null;
                    this.$emit('attachment-selected', null);
                }
                
                this.showDeleteModal = false;
                this.attachmentToDelete = null;
            }
        },

        handleMouseEnter(attachment, event) {
            this.hoveredAttachment = attachment.id;
            
            // Очищаем предыдущий таймаут
            if (this.tooltipTimeout) {
                clearTimeout(this.tooltipTimeout);
            }
            
            // Устанавливаем новый таймаут для показа тултипа
            this.tooltipTimeout = setTimeout(() => {
                this.showTooltip = true;
                this.tooltipText = attachment.name;
                this.updateTooltipPosition(event);
            }, 800); // 800ms задержка перед показом тултипа
        },

        handleMouseLeave() {
            this.hoveredAttachment = null;
            this.showTooltip = false;
            this.tooltipText = '';
            
            if (this.tooltipTimeout) {
                clearTimeout(this.tooltipTimeout);
                this.tooltipTimeout = null;
            }
        },

        updateTooltipPosition(event) {
            if (event && event.target) {
                const rect = event.target.getBoundingClientRect();
                this.tooltipStyle = {
                    left: `${rect.left + rect.width / 2}px`,
                    top: `${rect.bottom + 5}px`,
                    transform: 'translateX(-50%)'
                };
            }
        }
    }
}
</script>

<style scoped>
.selector {
    width: 200px;
    height: 500px;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    padding: 15px;
    overflow-y: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.selector::-webkit-scrollbar {
    display: none;
}

.category {
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.category:last-child {
    margin-bottom: 0;
}

.category-header {
    margin-bottom: 5px;
}

.category-title {
    font-size: 10px;
    font-weight: bold;
    color: #a2a2a2;
    text-transform: uppercase;
}

.attachments-list {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
    overflow-y: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    min-height: 0;
}

.attachments-list::-webkit-scrollbar {
    display: none;
}

.add-btn {
    width: 85px;
    height: 25px;
    background: rgba(79, 91, 223, 0.4);
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: #fff;
    cursor: pointer;
    transition: background-color 0.3s ease;
    margin-top: 8px;
    flex-shrink: 0;
}

.add-btn:hover {
    background: rgba(79, 91, 223, 1);
}

.attachment {
    width: 125px;
    height: 25px;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: #000;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
    cursor: pointer;
    transition: all 0.3s ease;
    position: relative;
    flex-shrink: 0;
}

.attachment:hover {
    border-color: #4F5BDF;
}

.attachment.selected {
    border-color: #4F5BDF;
    background-color: rgba(79, 91, 223, 0.1);
}

.attachment-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
}

.delete-btn {
    background: none;
    border: none;
    font-size: 16px;
    color: #ff4444;
    cursor: pointer;
    padding: 0;
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: background-color 0.3s ease;
}

.delete-btn:hover {
    background-color: rgba(255, 68, 68, 0.1);
}

/* Tooltip styles */
.tooltip {
    position: fixed;
    background: rgba(0, 0, 0, 0.8);
    color: white;
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 11px;
    white-space: nowrap;
    z-index: 1001;
    pointer-events: none;
    animation: fadeIn 0.2s ease-in-out;
}

.tooltip::before {
    content: '';
    position: absolute;
    top: -5px;
    left: 50%;
    transform: translateX(-50%);
    width: 0;
    height: 0;
    border-left: 5px solid transparent;
    border-right: 5px solid transparent;
    border-bottom: 5px solid rgba(0, 0, 0, 0.8);
}

@keyframes fadeIn {
    from {
        opacity: 0;
        transform: translateX(-50%) translateY(-5px);
    }
    to {
        opacity: 1;
        transform: translateX(-50%) translateY(0);
    }
}
</style>