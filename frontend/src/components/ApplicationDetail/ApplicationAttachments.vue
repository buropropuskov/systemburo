<template>
  <div class="application-attachments">
    <div
      v-if="attachments.length === 0"
      class="no-attachments"
    >
      Вложения отсутствуют
    </div>
        
    <div
      v-else
      class="attachments-list"
    >
      <!-- Группируем вложения по unique_attachment_id -->
      <div 
        v-for="(group, groupIndex) in groupedAttachments" 
        :key="groupIndex"
        class="attachment-group"
      >
        <!-- Заголовок группы (title из unique_attachments) -->
        <div
          v-if="group.title"
          class="group-header"
        >
          <h5 class="group-title">
            {{ group.title }}
          </h5>
        </div>
                
        <!-- Вложения в этой группе -->
        <div class="group-items">
          <div
            v-for="attachment in group.items"
            :key="attachment.id"
            class="attachment-item"
            :class="{ selected: selectedAttachment?.id === attachment.id }"
            @click="selectAttachment(attachment)"
          >
            <div
              class="attachment-name"
              :title="attachment.attachment_display_name"
            >
              {{ attachment.attachment_display_name || truncateText(attachment.attachment_name, 25) }}
            </div>
            <div
              v-if="attachment.entry_date_from || attachment.entry_date_to"
              class="attachment-dates"
            >
              {{ formatAttachmentDates(attachment) }}
            </div>
          </div>
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
    emits: ['attachment-selected'],
    data() {
        return {
            selectedAttachment: null
        }
    },
    computed: {
        // Группируем вложения по unique_attachment_title
        groupedAttachments() {
            if (!this.attachments.length) {
                return [];
            }

            const groups = {};
            
            this.attachments.forEach(attachment => {
                // Определяем ключ группировки
                const groupKey = attachment.unique_attachment_title || 
                               attachment.unique_attachment_display_name || 
                               'Без группы';
                
                if (!groups[groupKey]) {
                    groups[groupKey] = {
                        title: attachment.unique_attachment_title || 
                               attachment.unique_attachment_display_name,
                        items: []
                    };
                }
                
                groups[groupKey].items.push(attachment);
            });

            // Преобразуем объект в массив
            return Object.values(groups);
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
            // Пытаемся извлечь номер из display_name
            if (attachment.attachment_display_name) {
                const match = attachment.attachment_display_name.match(/№\s*(\d+)/);
                if (match) return match[1];
            }
            
            // Или из имени
            const nameMatch = attachment.attachment_name?.match(/_(\d+)$/);
            if (nameMatch) return nameMatch[1];
            
            // Или используем ID
            return attachment.id;
        },
        
        truncateText(text, maxLength) {
            if (!text) return '';
            if (text.length <= maxLength) return text;
            return text.substring(0, maxLength) + '...';
        },
        
        formatAttachmentDates(attachment) {
            if (!attachment.entry_date_from && !attachment.entry_date_to) return '';
            
            const formatDate = (dateStr) => {
                if (!dateStr) return '';
                const date = new Date(dateStr);
                return date.toLocaleDateString('ru-RU', {
                    day: '2-digit',
                    month: '2-digit',
                    year: 'numeric'
                });
            };
            
            const from = formatDate(attachment.entry_date_from);
            const to = formatDate(attachment.entry_date_to);
            
            if (from && to) {
                // Проверяем, если даты одинаковые
                const fromDate = new Date(attachment.entry_date_from);
                const toDate = new Date(attachment.entry_date_to);
                
                if (fromDate.toDateString() === toDate.toDateString()) {
                    return from; // Показываем только одну дату с годом
                }
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `по ${to}`;
            }
            return '';
        },
        
        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.$emit('attachment-selected', attachment);
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
    color: var(--text);
    margin: 0;
    font-weight: 600;
}

.no-attachments {
    text-align: center;
    color: var(--text-muted);
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
    flex: 1;
    overflow-y: auto;
}

.attachment-group {
    margin-bottom: 15px;
}

.group-header {
    padding-bottom: 10px;
}

.group-title {
    font-size: 12px;
    color: var(--text-muted);
    margin: 0;
    font-weight: 600;
    text-transform: uppercase;
}

.group-items {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.attachment-item {
    padding: 9px 15px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 15px;
    cursor: pointer;
    transition: all 0.2s ease;
}

.attachment-item:hover {
    border-color: var(--accent);
    background: var(--accent-tint);
}

.attachment-item.selected {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 10%, var(--surface));
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
    color: var(--accent-text);
    text-transform: uppercase;
    background: color-mix(in srgb, var(--accent) 10%, var(--surface));
    padding: 2px 6px;
    border-radius: 4px;
}

.attachment-number {
    font-size: 11px;
    color: var(--text-muted);
}

.attachment-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
    margin-bottom: 3px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.attachment-dates {
    font-size: 11px;
    color: var(--text-muted);
}
</style>