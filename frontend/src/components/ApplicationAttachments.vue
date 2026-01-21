<template>
    <div class="application-attachments">
        <div class="attachments-header">
            <h4>Вложения заявки</h4>
        </div>
        
        <div v-if="attachments.length === 0" class="no-attachments">
            Вложения отсутствуют
        </div>
        
        <div v-else class="attachments-list">
            <div
                v-for="attachment in attachments"
                :key="attachment.id"
                class="attachment-item"
                :class="{ selected: selectedAttachment?.id === attachment.id }"
                @click="selectAttachment(attachment)"
            >
                <div class="attachment-header">
                    <span class="attachment-type">{{ getAttachmentTypeLabel(attachment.attachment_type) }}</span>
                    <span class="attachment-number">№{{ getAttachmentNumber(attachment) }}</span>
                </div>
                <div class="attachment-name" :title="attachment.attachment_display_name">
                    {{ truncateText(attachment.attachment_display_name, 20) }}
                </div>
                <div v-if="attachment.entry_date_from || attachment.entry_date_to" class="attachment-dates">
                    {{ formatAttachmentDates(attachment) }}
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'ApplicationAttachments',
    props: {
        applicationId: {
            type: Number,
            required: true
        },
        attachments: {
            type: Array,
            default: () => []
        }
    },
    data() {
        return {
            selectedAttachment: null
        }
    },
    methods: {
        getAttachmentTypeLabel(type) {
            const types = {
                'cars': 'Авто',
                'people': 'Люди',
                'items': 'ТМЦ'
            };
            return types[type] || type;
        },
        
        getAttachmentNumber(attachment) {
            const match = attachment.attachment_display_name?.match(/№(\d+)/);
            if (match) return match[1];
            
            const nameMatch = attachment.attachment_name?.match(/_(\d+)$/);
            if (nameMatch) return nameMatch[1];
            
            return '1';
        },
        
        truncateText(text, maxLength) {
            if (!text) return '';
            if (text.length <= maxLength) return text;
            return text.substring(0, maxLength) + '...';
        },
        
        formatAttachmentDates(attachment) {
            const from = attachment.entry_date_from ? this.formatDate(attachment.entry_date_from) : '';
            const to = attachment.entry_date_to ? this.formatDate(attachment.entry_date_to) : '';
            
            if (from && to) {
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `по ${to}`;
            }
            return '';
        },
        
        formatDate(date) {
            if (!date) return '';
            const d = new Date(date);
            return d.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit'
            });
        },
        
        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.$emit('attachment-selected', attachment);
        }
    },
    watch: {
        attachments: {
            immediate: true,
            handler(newAttachments) {
                if (newAttachments.length > 0 && !this.selectedAttachment) {
                    this.selectedAttachment = newAttachments[0];
                    this.$emit('attachment-selected', this.selectedAttachment);
                }
            }
        }
    }
}
</script>

<style scoped>
.application-attachments {
    height: 100%;
    display: flex;
    flex-direction: column;
}

.attachments-header {
    margin-bottom: 15px;
}

.attachments-header h4 {
    font-size: 16px;
    color: #333;
    margin: 0;
    font-weight: 600;
}

.no-attachments {
    text-align: center;
    color: #a2a2a2;
    padding: 40px 20px;
    font-size: 14px;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
}

.attachments-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    flex: 1;
    overflow-y: auto;
}

.attachment-item {
    padding: 12px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.2s ease;
}

.attachment-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.attachment-item.selected {
    border-color: #4F5BDF;
    background: rgba(79, 91, 223, 0.1);
}

.attachment-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 5px;
}

.attachment-type {
    font-size: 11px;
    font-weight: 600;
    color: #4F5BDF;
    text-transform: uppercase;
    background: rgba(79, 91, 223, 0.1);
    padding: 2px 6px;
    border-radius: 4px;
}

.attachment-number {
    font-size: 11px;
    color: #a2a2a2;
}

.attachment-name {
    font-size: 13px;
    font-weight: 500;
    color: #333;
    margin-bottom: 3px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.attachment-dates {
    font-size: 11px;
    color: #666;
}
</style>