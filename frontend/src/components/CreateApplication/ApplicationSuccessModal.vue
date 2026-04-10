<template>
  <BaseModal :show="show" title="Заявка успешно оформлена!" width="600px" @close="$emit('close')">
    <div class="success-content">
      <div class="application-number">
        <span class="application-number__label">Номер заявки</span>
        <span class="application-number__value">{{ applicationNumber }}</span>
      </div>

      <div class="progress-bar">
        <div
          v-for="(stage, index) in stages"
          :key="index"
          class="progress-stage"
          :class="{ 'progress-stage--active': index <= currentStep }"
        >
          <div class="progress-stage__circle">{{ index + 1 }}</div>
          <span class="progress-stage__label">{{ stage }}</span>
          <div v-if="index < stages.length - 1" class="progress-stage__line" />
        </div>
      </div>

      <div class="attachments-section">
        <h4 class="attachments-title">Вложения</h4>
        <ul v-if="attachmentsData.length" class="attachments-list">
          <li v-for="(item, i) in attachmentsData" :key="i" class="attachment-item">
            <span class="attachment-name">{{ item.display_name }}</span>
            <span class="attachment-meta">{{ item.period }}<template v-if="item.time">, {{ item.time }}</template></span>
          </li>
        </ul>
        <p v-else class="attachments-empty">Вложения отсутствуют</p>
      </div>
    </div>

    <template #actions>
      <button class="btn btn--primary" @click="$emit('close')">Закрыть</button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
  name: 'ApplicationSuccessModal',
  components: { BaseModal },

  props: {
    show: {
      type: Boolean,
      default: false,
    },
    applicationNumber: {
      type: String,
      default: '',
    },
    attachmentsData: {
      type: Array,
      default: () => [],
    },
  },

  emits: ['close'],

  data() {
    return {
      currentStep: 0,
      stages: ['Оформлена', 'В обработке', 'Согласование', 'В работе', 'Завершена'],
    }
  },
}
</script>

<style scoped>
.success-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.application-number {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.application-number__label {
  font-size: 13px;
  color: var(--color-text-muted);
}

.application-number__value {
  font-size: 22px;
  font-weight: 700;
  color: var(--color-primary);
  letter-spacing: 0.5px;
}

/* Progress bar */
.progress-bar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.progress-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  position: relative;
}

.progress-stage__circle {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  background: var(--color-border);
  color: var(--color-text-muted);
  position: relative;
  z-index: 1;
  transition: all 0.3s;
}

.progress-stage--active .progress-stage__circle {
  background: var(--color-primary);
  color: #fff;
}

.progress-stage__label {
  margin-top: 8px;
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: center;
  line-height: 1.3;
}

.progress-stage--active .progress-stage__label {
  color: var(--color-primary);
  font-weight: 600;
}

.progress-stage__line {
  position: absolute;
  top: 16px;
  left: calc(50% + 16px);
  width: calc(100% - 32px);
  height: 2px;
  background: var(--color-border);
}

.progress-stage--active .progress-stage__line {
  background: var(--color-primary);
}

/* Attachments */
.attachments-title {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
}

.attachments-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.attachment-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: var(--color-bg-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.attachment-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text);
}

.attachment-meta {
  font-size: 13px;
  color: var(--color-text-muted);
  white-space: nowrap;
  margin-left: 12px;
}

.attachments-empty {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-muted);
  font-style: italic;
}

.btn {
  padding: 10px 24px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.btn--primary {
  background-color: var(--color-primary);
  color: #fff;
}

.btn--primary:hover {
  background-color: var(--color-primary-hover);
}

@media (max-width: 480px) {
  .progress-stage__circle {
    width: 26px;
    height: 26px;
    font-size: 11px;
  }

  .progress-stage__label {
    font-size: 10px;
  }

  .progress-stage__line {
    top: 13px;
    left: calc(50% + 13px);
    width: calc(100% - 26px);
  }

  .attachment-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .attachment-meta {
    margin-left: 0;
  }
}
</style>
