<!-- ForwardMessages.vue -->
<!-- Блок сопроводительных сообщений при пересылке заявки (#967). Виден всем получателям,
     собирает пересылы от разных согласующих. Пустой список -> блок скрыт. -->
<template>
  <section
    v-if="messages.length > 0"
    class="forward-messages-section"
    data-testid="forward-messages"
  >
    <div class="forward-messages-header">
      <h4>Сообщения при пересылке</h4>
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
        <p class="forward-message-text">
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
                // Блок сообщений - вспомогательный показ; при сбое сохраняем то, что уже
                // отрисовано, и не роняем деталь заявки.
                if (seq !== this.loadSeq) return;
            }
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
    gap: 12px;
}

.forward-message-item {
    background: var(--color-bg-secondary, #f7f7f9);
    border: 1px solid var(--color-border, #e6e6e6);
    border-radius: var(--radius-md, 15px);
    padding: 12px;
}

.forward-message-meta {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 6px;
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

.forward-message-text {
    margin: 0;
    font-size: 14px;
    line-height: 150%;
    color: #333;
    white-space: pre-wrap;
    word-break: break-word;
}
</style>
