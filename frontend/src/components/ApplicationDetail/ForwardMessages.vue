<!-- ForwardMessages.vue -->
<!-- Ветка заявки (#967): хронология пересылок (как ветка письма в Outlook). Каждый пункт -
     кто переслал, кому и когда, плюс сопроводительный текст если он был. Виден всем
     получателям. Пустой список -> блок скрыт. -->
<template>
  <section
    v-if="messages.length > 0"
    class="forward-messages-section"
    data-testid="forward-messages"
  >
    <div class="forward-messages-header">
      <h4>Ветка заявки</h4>
    </div>
    <ul class="forward-messages-list">
      <li
        v-for="msg in messages"
        :key="msg.id"
        class="forward-message-item"
        data-testid="forward-message-item"
      >
        <div class="forward-message-meta">
          <span class="forward-message-author">{{ msg.author_name || 'Пользователь' }}</span>
          <span class="forward-message-date">{{ formatDateTime(msg.created_at) }}</span>
        </div>
        <p class="forward-message-action">
          {{ actionText(msg) }}
        </p>
        <p
          v-if="recipientsText(msg)"
          class="forward-message-recipients"
        >
          <span class="forward-message-recipients-label">Кому:</span>
          {{ recipientsText(msg) }}
        </p>
        <p
          v-if="msg.message"
          class="forward-message-text"
        >
          {{ msg.message }}
        </p>
      </li>
    </ul>
  </section>
</template>

<script>
import { getForwardMessages } from '@/api/applications'

export default {
    name: 'ForwardMessages',
    props: {
        applicationId: {
            type: Number,
            required: true
        }
    },
    data() {
        return {
            messages: [],
            loadSeq: 0
        }
    },
    watch: {
        // Переключение детали на другую заявку без размонтирования -> перезагрузка.
        applicationId() {
            this.load();
        }
    },
    mounted() {
        this.load();
    },
    methods: {
        // Seq-токен: при быстрой смене заявки медленный ответ предыдущей не затирает
        // актуальный (урок гонки авто-fetch по watch, #632).
        async load() {
            if (!this.applicationId) return;
            const seq = ++this.loadSeq;
            try {
                const data = await getForwardMessages(this.applicationId);
                if (seq !== this.loadSeq) return;
                this.messages = Array.isArray(data) ? data : [];
            } catch {
                // Ветка заявки - вспомогательный показ; при сбое сохраняем то, что уже
                // отрисовано, и не роняем деталь заявки.
                if (seq !== this.loadSeq) return;
            }
        },

        recipientsText(msg) {
            return Array.isArray(msg.recipients) ? msg.recipients.join(', ') : '';
        },

        // Действие пересылки теми же словами, что в блоке История (getActionText).
        actionText(msg) {
            const names = msg.attachments;
            if (msg.whole || !Array.isArray(names) || names.length === 0) {
                return 'Переслал(-а) всю заявку';
            }
            return `Переслал(-а) вложения: ${names.join(', ')}`;
        },

        formatDateTime(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        }
    }
}
</script>

<style scoped>
.forward-messages-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    overflow: hidden;
}

.forward-messages-header {
    margin: -15px -15px 12px;
    padding: 12px 15px;
    border-bottom: 1px solid #e6e6e6;
}

.forward-messages-header h4 {
    margin: 0;
    font-size: 14px;
    color: #a2a2a2;
    font-weight: 400;
}

.forward-messages-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
}

/* Плоская ветка (как переписка): разделители между пунктами, без карточек/фона. */
.forward-message-item {
    padding: 12px 0;
    border-bottom: 1px solid #eee;
}

.forward-message-item:first-child {
    padding-top: 0;
}

.forward-message-item:last-child {
    padding-bottom: 0;
    border-bottom: none;
}

.forward-message-meta {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 4px;
}

.forward-message-author {
    font-size: 14px;
    font-weight: 600;
    color: #000;
}

.forward-message-date {
    font-size: 12px;
    color: #a2a2a2;
    flex-shrink: 0;
}

.forward-message-action {
    margin: 0 0 4px;
    font-size: 13px;
    font-weight: 500;
    color: #333;
    line-height: 140%;
    word-break: break-word;
}

.forward-message-recipients {
    margin: 0 0 4px;
    font-size: 13px;
    color: #666;
    line-height: 140%;
    word-break: break-word;
}

.forward-message-recipients-label {
    color: #a2a2a2;
}

.forward-message-text {
    margin: 4px 0 0;
    font-size: 14px;
    line-height: 150%;
    color: #333;
    white-space: pre-wrap;
    word-break: break-word;
}
</style>
