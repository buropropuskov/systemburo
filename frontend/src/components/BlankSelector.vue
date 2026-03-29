<template>
    <div class="selector">
        <div class="categories-container">
            <div 
                v-for="category in uniqueCategories" 
                :key="category"
                class="category"
            >
                <div class="category-header">
                    <div class="category-title">{{ category }}</div>
                    <span class="attachment-count">{{ getCategoryAttachments(category).length }}/10</span>
                </div>
                
                <transition-group name="attachment" tag="div" class="attachments-list">
                    <div
                        v-for="attachment in getCategoryAttachments(category)"
                        :key="getAttachmentKey(attachment)"
                        class="attachment"
                        :class="{ selected: isSelected(attachment) }"
                        @click="selectAttachment(attachment)"
                        @mouseenter="handleMouseEnter(attachment, $event)"
                        @mouseleave="handleMouseLeave"
                    >
                        <input 
                            type="checkbox" 
                            :value="getAttachmentKey(attachment)"
                            v-model="selectedAttachments"
                            @click.stop
                            class="attachment-checkbox"
                        >
                        <span class="attachment-name">{{ attachment.display_name }}</span>
                        
                        <button 
                            v-if="hoveredAttachment === getAttachmentKey(attachment)"
                            class="delete-btn"
                            @click.stop="confirmDelete(attachment)"
                        >
                            ×
                        </button>
                    </div>
                </transition-group>

                <button 
                    class="add-btn"
                    :disabled="getCategoryAttachments(category).length >= 10"
                    @click="addAttachment(category)"
                >
                    Добавить
                </button>
            </div>
        </div>

        <div class="actions">
            <button 
                class="action-btn delete-selected" 
                :disabled="selectedAttachments.length === 0"
                @click="confirmDeleteMultiple"
            >
                Удалить выбранные
            </button>
            <button 
                class="action-btn delete-all" 
                :disabled="attachments.length === 0"
                @click="confirmDeleteAll"
            >
                Удалить все
            </button>
        </div>

        <ConfirmationModal
            :show="showDeleteModal"
            title="Подтверждение удаления"
            :message="deleteMessage"
            confirm-text="Удалить"
            cancel-text="Отмена"
            :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
            @confirm="deleteAttachments"
            @cancel="cancelDelete"
        />

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
    name: 'BlankSelector',
    components: {
        ConfirmationModal
    },
    props: {
        attachments: {
            type: Array,
            default: () => []
        },
        currentApplicationData: {
            type: Object,
            default: () => ({})
        }
    },
    data() {
        return {
            allTemplates: [],
            selectedAttachment: null,
            hoveredAttachment: null,
            showDeleteModal: false,
            attachmentsToDelete: [],
            showTooltip: false,
            tooltipText: '',
            tooltipStyle: {},
            tooltipTimeout: null,
            selectedAttachments: []
        }
    },
    computed: {
        deleteMessage() {
            if (this.attachmentsToDelete.length === 1) {
                return `Вы точно хотите удалить "${this.attachmentsToDelete[0]?.display_name}"?`;
            } else if (this.attachmentsToDelete.length === this.attachments.length && this.attachments.length > 0) {
                return `Вы точно хотите удалить ВСЕ бланки (${this.attachments.length})?`;
            } else {
                return `Вы точно хотите удалить выбранные бланки (${this.attachmentsToDelete.length})?`;
            }
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
    watch: {
        attachments: {
            handler(newAttachments) {
                const existingIds = new Set(newAttachments.map(a => this.getAttachmentKey(a)));
                this.selectedAttachments = this.selectedAttachments.filter(id => existingIds.has(id));
                
                if (this.selectedAttachment && !existingIds.has(this.getAttachmentKey(this.selectedAttachment))) {
                    this.selectedAttachment = null;
                    this.$emit('attachment-selected', null);
                } else if (this.selectedAttachment && existingIds.has(this.getAttachmentKey(this.selectedAttachment))) {
                    const currentAttachment = newAttachments.find(a => 
                        this.getAttachmentKey(a) === this.getAttachmentKey(this.selectedAttachment)
                    );
                    if (currentAttachment && currentAttachment !== this.selectedAttachment) {
                        this.selectedAttachment = currentAttachment;
                    }
                }
            },
            deep: true,
            immediate: true
        }
    },
    methods: {
        getAttachmentKey(attachment) {
            return attachment.local_id || attachment.id;
        },
        
        isSelected(attachment) {
            if (!this.selectedAttachment) return false;
            return this.getAttachmentKey(this.selectedAttachment) === this.getAttachmentKey(attachment);
        },
        
        setSelectedAttachment(attachment) {
            if (attachment) {
                const currentAttachment = this.attachments.find(a => 
                    this.getAttachmentKey(a) === this.getAttachmentKey(attachment)
                );
                this.selectedAttachment = currentAttachment || null;
            } else {
                this.selectedAttachment = null;
            }
        },
        
        getCategoryAttachments(category) {
            return this.attachments.filter(attachment => attachment.title === category);
        },
        
        getNextAttachmentNumber(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            const existingNumbers = new Set();
            
            categoryAttachments.forEach(attachment => {
                const match = attachment.display_name.match(/\d+$/);
                if (match) {
                    existingNumbers.add(parseInt(match[0]));
                }
            });
            
            let number = 1;
            while (existingNumbers.has(number)) {
                number++;
            }
            return number;
        },
        
        async fetchTemplates() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/attachments", {
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
                });
                if (response.ok) {
                    const data = await response.json();
                    this.allTemplates = data;
                }
            } catch (error) {
                console.error("Error fetching attachment templates:", error);
            }
        },
        
        addAttachment(category) {
            const categoryAttachments = this.getCategoryAttachments(category);
            if (categoryAttachments.length >= 10) {
                alert(`Максимальное количество бланков в категории "${category}" — 10.`);
                return;
            }

            const template = this.allTemplates.find(t => t.title === category);
            
            if (!template) {
                console.warn(`No template found for category: ${category}`);
                return;
            }
            
            const nextNumber = this.getNextAttachmentNumber(category);
            const newAttachment = {
                id: template.id,
                local_id: Date.now() + Math.random(),
                template_id: template.id,
                title: category,
                name: `${template.name}_${nextNumber}`,
                display_name: `${template.display_name} №${nextNumber}`,
                attachment_type: template.attachment_type,
                instruction: template.instruction,
                created_at: new Date().toISOString(),
                is_active: true
            };
            
            this.$emit('attachment-added', newAttachment);
        },
        
        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.$emit('attachment-selected', attachment);
        },
        
        confirmDelete(attachment) {
            this.attachmentsToDelete = [attachment];
            this.showDeleteModal = true;
        },
        
        confirmDeleteMultiple() {
            if (this.selectedAttachments.length === 0) return;
            
            this.attachmentsToDelete = this.attachments.filter(
                a => this.selectedAttachments.includes(this.getAttachmentKey(a))
            );
            this.showDeleteModal = true;
        },
        
        confirmDeleteAll() {
            if (this.attachments.length === 0) return;
            this.attachmentsToDelete = [...this.attachments];
            this.showDeleteModal = true;
        },
        
        cancelDelete() {
            this.showDeleteModal = false;
            this.attachmentsToDelete = [];
        },
        
        deleteAttachments() {
            if (this.attachmentsToDelete.length === 0) return;
            
            const keysToDelete = new Set(this.attachmentsToDelete.map(a => this.getAttachmentKey(a)));
            
            // Эмитим событие удаления для каждого вложения
            this.attachmentsToDelete.forEach(attachment => {
                this.$emit('attachment-removed', attachment);
            });
            
            if (this.selectedAttachment && keysToDelete.has(this.getAttachmentKey(this.selectedAttachment))) {
                this.selectedAttachment = null;
                this.$emit('attachment-selected', null);
            }
            
            this.selectedAttachments = this.selectedAttachments.filter(id => !keysToDelete.has(id));
            this.showDeleteModal = false;
            this.attachmentsToDelete = [];
        },
        
        handleMouseEnter(attachment, event) {
            this.hoveredAttachment = this.getAttachmentKey(attachment);
            
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
                    top: `${rect.bottom + 7}px`,
                    transform: 'translateX(-50%)'
                };
            }
        },
        
        clearSelection() {
            this.selectedAttachments = [];
            this.selectedAttachment = null;
        }
    },
    mounted() {
        this.fetchTemplates();
    }
}
</script>

<style scoped>
.selector {
    width: 200px;
    height:490px;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    padding: 15px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    position: relative;
    transition: 0.4s;
}

.categories-container {
    flex: 1;
    overflow-y: auto;
    scrollbar-width: none;
    -ms-overflow-style: none;
    margin-bottom: 10px;
    border-bottom: 1px solid #e6e6e6;
}

.categories-container::-webkit-scrollbar {
    display: none;
}

.actions {
    flex-shrink: 0;
    display: flex;
    gap: 8px;
    flex-direction: column;
    margin-top: 10px;
    background: white;
    z-index: 10;
}

.action-btn {
    flex: 1;
    padding: 4px 8px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.3s ease;
    border: 1px solid #e6e6e6;
    background: white;
    color: #333;
}

.action-btn.delete-selected:hover:not(:disabled) {
    background: #ff4444;
    color: white;
    border-color: #ff4444;
}

.action-btn.delete-all:hover:not(:disabled) {
    background: #ff4444;
    color: white;
    border-color: #ff4444;
}

.action-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.category {
    margin-bottom: 20px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    transition: 0.4s;
}

.category-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 5px;
}

.category-title {
    font-size: 10px;
    font-weight: 700;
    color: #a2a2a2;
    text-transform: uppercase;
}

.attachment-count {
    font-size: 9px;
    color: #a2a2a2;
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

.attachment-enter-active,
.attachment-leave-active {
    transition: all 0.2s ease;
}

.attachment-enter-from {
    opacity: 0;
    transform: scale(0.8);
}

.attachment-leave-to {
    opacity: 0;
    transform: scale(0.8);
}

.attachment-move {
    transition: transform 0.2s ease;
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
    transition:  0.4s ease;
    margin-top: 8px;
    flex-shrink: 0;
}

.add-btn:hover:not(:disabled) {
    background: rgba(79, 91, 223, 1);
}

.add-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.attachment {
    width: 139px;
    min-height: 25px;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 500;
    color: #000;
    display: flex;
    align-items: center;
    gap: 4px;
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

.attachment-checkbox {
    margin: 0;
    width: 12px;
    height: 12px;
    cursor: pointer;
    flex-shrink: 0;
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
    flex-shrink: 0;
}

.delete-btn:hover {
    background-color: rgba(255, 68, 68, 0.1);
}

.tooltip {
    position: fixed;
    background: rgba(0, 0, 0, 0.8);
    color: white;
    padding: 6px 10px;
    border-radius: 15px;
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