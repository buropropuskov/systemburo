<!-- ForwardMessages.vue -->
<!-- Ветка заявки (#967): хронология пересылок (как ветка письма в Outlook). Каждый пункт -
     кто переслал, кому и когда, плюс сопроводительный текст если он был. Виден всем
     получателям. Пустой список -> блок скрыт. Заголовок сворачивает/разворачивает список
     (свёрнут по умолчанию, выбор запоминается в localStorage). -->
<template>
  <section
    v-if="messages.length > 0"
    class="forward-messages-section"
    :class="{ collapsed }"
    data-testid="forward-messages"
  >
    <button
      type="button"
      class="forward-messages-header"
      :aria-expanded="!collapsed"
      :aria-controls="bodyId"
      data-testid="forward-messages-toggle"
      @click="toggleCollapse"
    >
      <span
        class="fm-chevron"
        aria-hidden="true"
      >▾</span>
      <h4>Ветка заявки</h4>
      <span class="fm-count">{{ messages.length }}</span>
    </button>
    <div
      :id="bodyId"
      class="forward-messages-body"
    >
      <div class="forward-messages-body-inner">
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
            <div class="forward-message-recipients">
              <span class="forward-message-recipients-label">Получатели:</span>
              <div
                v-for="recipient in forwardRecipients(msg)"
                :key="recipient.user_id"
                class="forward-message-recipient"
              >
                <strong>{{ recipient.display_name }}</strong><span> — {{ forwardRecipientPurpose(recipient) }}</span>
              </div>
              <div v-if="!forwardRecipients(msg).length">
                {{ forwardRecipientsUnavailable }}
              </div>
            </div>
            <p class="forward-message-materials">
              <span class="forward-message-recipients-label">Материалы: </span>{{ forwardMaterialsText(msg) }}
            </p>
            <p
              v-if="msg.message"
              class="forward-message-text"
            >
              {{ msg.message }}
            </p>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

<script>
import { getForwardMessages } from '@/api/applications'
import { forwardRecipients, forwardRecipientPurpose, forwardActionText, forwardMaterialsText, FORWARD_RECIPIENTS_UNAVAILABLE } from '@/utils/applicationForwardHistory';

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
            loadSeq: 0,
            // Свёрнут по умолчанию; выбор пользователя запоминаем в localStorage.
            collapsed: this.readCollapsed()
        }
    },
    computed: {
        forwardRecipientsUnavailable() {
            return FORWARD_RECIPIENTS_UNAVAILABLE;
        },
        // id тела для aria-controls кнопки-заголовка (disclosure widget).
        bodyId() {
            return `forward-messages-body-${this.applicationId}`;
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
        forwardRecipients,
        forwardRecipientPurpose,
        forwardMaterialsText,
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

        // Дефолт свёрнут: сохранённое 'false' раскрывает, любое другое/отсутствие -> свёрнуто.
        readCollapsed() {
            try {
                return localStorage.getItem('forwardThread.collapsed') !== 'false';
            } catch {
                // localStorage может быть недоступен (приватный режим) - не критично для показа.
                return true;
            }
        },

        toggleCollapse() {
            this.collapsed = !this.collapsed;
            try {
                localStorage.setItem('forwardThread.collapsed', String(this.collapsed));
            } catch {
                // Персист необязателен - молча пропускаем, если localStorage недоступен.
            }
        },

        // Действие пересылки теми же словами, что в блоке История (getActionText).
        actionText(msg) {
            return forwardActionText(msg);
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
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    box-shadow: 0 2px 12px var(--shadow-drop);
    overflow: hidden;
}

.forward-messages-header {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 15px;
    background: none;
    border: none;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    font-family: inherit;
    text-align: left;
    transition: border-color 0.2s ease;
}

.forward-messages-section.collapsed .forward-messages-header {
    border-bottom-color: transparent;
}

.fm-chevron {
    color: var(--text-muted);
    font-size: 12px;
    transition: transform 0.2s ease;
}

.forward-messages-section.collapsed .fm-chevron {
    transform: rotate(-90deg);
}

.forward-messages-header h4 {
    margin: 0;
    flex: 1;
    font-size: 14px;
    color: var(--text-muted);
    font-weight: 400;
}

.fm-count {
    font-size: 12px;
    color: var(--color-primary, var(--accent-text));
    background: color-mix(in srgb, var(--accent) 8%, var(--surface));
    padding: 2px 9px;
    border-radius: 999px;
    font-weight: 600;
}

/* Плавное сворачивание высотой (grid-rows 1fr<->0fr + min-height:0, урок #510). */
.forward-messages-body {
    display: grid;
    grid-template-rows: 1fr;
    transition: grid-template-rows 0.25s ease;
}

.forward-messages-section.collapsed .forward-messages-body {
    grid-template-rows: 0fr;
}

.forward-messages-body-inner {
    min-height: 0;
    overflow: hidden;
}

.forward-messages-list {
    list-style: none;
    margin: 0;
    padding: 15px;
    display: flex;
    flex-direction: column;
}

/* Плоская ветка (как переписка): разделители между пунктами, без карточек/фона. */
.forward-message-item {
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
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
    color: var(--text);
}

.forward-message-date {
    font-size: 12px;
    color: var(--text-muted);
    flex-shrink: 0;
}

.forward-message-action {
    margin: 0 0 4px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
    line-height: 140%;
    word-break: break-word;
}

.forward-message-recipients,
.forward-message-materials {
    margin: 0 0 4px;
    font-size: 13px;
    color: var(--text-muted);
    line-height: 140%;
    word-break: break-word;
}

.forward-message-recipient {
    margin: 6px 0;
}

.forward-message-recipients-label {
    color: var(--text-muted);
}

.forward-message-text {
    margin: 4px 0 0;
    font-size: 14px;
    line-height: 150%;
    color: var(--text);
    white-space: pre-wrap;
    word-break: break-word;
}
</style>
