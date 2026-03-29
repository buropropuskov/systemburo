<template>
    <div class="selector">
        <div 
            v-for="category in uniqueCategories" 
            :key="category"
            class="category"
        >
            <div class="category-header">
                <div class="category-title">{{ category }}</div>
            </div>
            
            <div class="attachments-list">
                <div
                    v-for="attachment in getCategoryAttachments(category)"
                    :key="attachment.local_id || attachment.id"
                    class="attachment"
                    :class="{ selected: selectedAttachment && (selectedAttachment.local_id || selectedAttachment.id) === (attachment.local_id || attachment.id) }"
                    @click="selectAttachment(attachment)"
                    @mouseenter="handleMouseEnter(attachment, $event)"
                    @mouseleave="handleMouseLeave"
                >
                    <span class="attachment-name">{{ attachment.display_name }}</span>
                    
                    <button 
                        v-if="hoveredAttachment === (attachment.local_id || attachment.id)"
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
import { apiRequest } from '@/api/client'
import ConfirmationModal from './ConfirmationModal.vue';

export default {
    name: 'BlankSelector',
    components: {
        ConfirmationModal
    },
    props: {
        currentApplicationData: {
            type: Object,
            default: () => ({})
        }
    },
    data() {
        return {
            attachments: [],
            allTemplates: [],
            selectedAttachment: null,
            hoveredAttachment: null,
            showDeleteModal: false,
            attachmentToDelete: null,
            showTooltip: false,
            tooltipText: '',
            tooltipStyle: {},
            tooltipTimeout: null,
            // Для хранения данных форм вложений
            attachmentData: {}
        }
    },
    computed: {
        deleteMessage() {
            return `Вы точно хотите удалить "${this.attachmentToDelete?.display_name}"?`;
        },
        uniqueCategories() {
            const categories = new Set();
            this.allTemplates.forEach(template => {
                if (template.title) {
                    categories.add(template.title);
                }
            });
            return Array.from(categories);
        }
    },
    methods: {
        getCategoryAttachments(category) {
            return this.attachments.filter(attachment => attachment.title === category);
        },
        
        getNextAttachmentNumber(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            return categoryAttachments.length + 1;
        },
        
        async fetchTemplates() {
            try {
                const response = await apiRequest("/attachments", {
                });
                if (response.ok) {
                    const data = await response.json();
                    this.allTemplates = data;
                    
                    // После загрузки шаблонов пытаемся восстановить данные из localStorage
                    this.restoreFromLocalStorage();
                }
            } catch (error) {
                console.error("Error fetching attachment templates:", error);
            }
        },
        
        addAttachment(category) {
            const template = this.allTemplates.find(t => t.title === category);
            
            if (!template) {
                console.warn(`No template found for category: ${category}`);
                return;
            }
            
            const nextNumber = this.getNextAttachmentNumber(category);
            const newAttachment = {
                id: template.id, // ID уникального бланка из базы
                local_id: Date.now() + Math.random(), // Локальный ID для интерфейса
                template_id: template.id,
                title: category,
                name: `${template.name}_copy_${nextNumber}`,
                display_name: `${template.display_name} №${nextNumber}`,
                attachment_type: template.attachment_type,
                instruction: template.instruction,
                created_at: new Date().toISOString(),
                is_active: true
            };
            
            this.attachments.push(newAttachment);
            this.selectAttachment(newAttachment);
            this.$emit('attachment-added', newAttachment);
            
            // Сохраняем в localStorage
            this.saveToLocalStorage();
        },
        
        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
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
                // Удаляем данные вложения из хранилища
                const deleteId = this.attachmentToDelete.local_id || this.attachmentToDelete.id;
                if (this.attachmentData[deleteId]) {
                    delete this.attachmentData[deleteId];
                }
                
                // Remove attachment from local array
                this.attachments = this.attachments.filter(
                    attachment => (attachment.local_id || attachment.id) !== deleteId
                );
                
                // Clear selection if deleted attachment was selected
                if (this.selectedAttachment && (this.selectedAttachment.local_id || this.selectedAttachment.id) === deleteId) {
                    this.selectedAttachment = null;
                    this.$emit('attachment-selected', null);
                }
                
                this.$emit('attachment-removed', this.attachmentToDelete);
                this.showDeleteModal = false;
                this.attachmentToDelete = null;
                
                // Сохраняем в localStorage
                this.saveToLocalStorage();
            }
        },

        handleMouseEnter(attachment, event) {
            this.hoveredAttachment = attachment.local_id || attachment.id;
            
            if (this.tooltipTimeout) {
                clearTimeout(this.tooltipTimeout);
            }
            
            this.tooltipTimeout = setTimeout(() => {
                this.showTooltip = true;
                this.tooltipText = attachment.display_name;
                this.updateTooltipPosition(event);
            }, 800);
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
        },
        
        // Методы для работы с данными вложений
        saveAttachmentData(attachmentId, data) {
            this.attachmentData[attachmentId] = {
                ...data,
                savedAt: new Date().toISOString()
            };
            this.saveToLocalStorage();
        },
        
        getAttachmentData(attachmentId) {
            return this.attachmentData[attachmentId] || null;
        },
        
        clearAttachmentData(attachmentId) {
            if (this.attachmentData[attachmentId]) {
                delete this.attachmentData[attachmentId];
                this.saveToLocalStorage();
            }
        },
        
        // Сохранение в localStorage
        saveToLocalStorage() {
            try {
                const savedData = {
                    attachments: this.attachments,
                    attachmentData: this.attachmentData,
                    selectedAttachment: this.selectedAttachment,
                    savedAt: new Date().toISOString()
                };
                localStorage.setItem('draftApplication', JSON.stringify(savedData));
            } catch (error) {
                console.error('Ошибка сохранения в localStorage:', error);
            }
        },
        
        // Восстановление из localStorage
        restoreFromLocalStorage() {
            try {
                const savedData = localStorage.getItem('draftApplication');
                if (savedData) {
                    const parsedData = JSON.parse(savedData);
                    
                    // Восстанавливаем вложения
                    if (parsedData.attachments && Array.isArray(parsedData.attachments)) {
                        this.attachments = parsedData.attachments;
                    }
                    
                    // Восстанавливаем данные вложений
                    if (parsedData.attachmentData) {
                        this.attachmentData = parsedData.attachmentData;
                    }
                    
                    // Восстанавливаем выбранное вложение
                    if (parsedData.selectedAttachment && this.attachments.length > 0) {
                        const foundAttachment = this.attachments.find(
                            a => (a.local_id || a.id) === (parsedData.selectedAttachment.local_id || parsedData.selectedAttachment.id)
                        );
                        if (foundAttachment) {
                            this.selectedAttachment = foundAttachment;
                            this.$emit('attachment-selected', foundAttachment);
                        }
                    }
                }
            } catch (error) {
                console.error('Ошибка восстановления из localStorage:', error);
            }
        },
        
        // Метод для загрузки существующих вложений заявки
        loadAttachments(existingAttachments) {
            this.attachments = existingAttachments || [];
            if (this.attachments.length > 0) {
                this.selectedAttachment = this.attachments[0];
                this.$emit('attachment-selected', this.selectedAttachment);
            }
            this.saveToLocalStorage();
        },
        
        // Метод для очистки всех вложений
        clearAttachments() {
            this.attachments = [];
            this.selectedAttachment = null;
            this.attachmentData = {};
            this.$emit('attachment-selected', null);
            localStorage.removeItem('draftApplication');
        },
        
        // Получить текущее выбранное вложение
        getSelectedAttachment() {
            return this.selectedAttachment;
        },
        
        // Получить все вложения
        getAllAttachments() {
            return this.attachments;
        },
        
        // Получить данные всех вложений
        getAllAttachmentData() {
            return this.attachmentData;
        }
    },
    mounted() {
        this.fetchTemplates();
    }
}
</script>

<style scoped>
/* Стили остаются без изменений */
.selector {
    width: 200px;
    height: 490px;
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