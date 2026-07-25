<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="close"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">
              Информация о машине
            </h3>
            <button
              class="modal-close"
              aria-label="Закрыть"
              @click="close"
            >
              ×
            </button>
          </div>

          <div class="modal-body">
            <div class="details-grid">
              <div class="detail-item">
                <span class="detail-label">Номер:</span>
                <span class="detail-value">{{ car.number || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Марка:</span>
                <span class="detail-value">{{ car.mark || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Формат номера:</span>
                <span class="detail-value">{{ car.format_name || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Организация:</span>
                <span class="detail-value">{{ car.organization_name || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Компания:</span>
                <span class="detail-value">{{ car.company_name || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Владелец:</span>
                <span class="detail-value">{{ car.user_name || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Статус:</span>
                <StatusBadge :status="car.status ? 'Активна' : 'Неактивна'" />
              </div>
              <div
                v-if="car.created_at"
                class="detail-item"
              >
                <span class="detail-label">Создана:</span>
                <span class="detail-value">{{ formatDate(car.created_at) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import StatusBadge from '@/components/ui/StatusBadge.vue';

export default {
  name: 'CarDetailsViewModal',
  components: { StatusBadge },
  props: {
    show: { type: Boolean, required: true },
    car: { type: Object, default: () => ({}) }
  },
  emits: ['close'],
  mounted() {
    document.addEventListener('keydown', this.onKey);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKey);
  },
  methods: {
    onKey(e) {
      if (e.key === 'Escape' && this.show) this.close();
    },
    close() { this.$emit('close'); },
    formatDate(dateStr) {
      if (!dateStr) return '-';
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return '-';
      return d.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit'
      });
    }
  }
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  backdrop-filter: blur(0.1px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 16px;
}

.modal-content {
  width: min(560px, 100%);
  background: var(--surface);
  border-radius: 18px;
  box-shadow: 0 12px 40px var(--shadow-drop);
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 22px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--text);
}

.modal-close {
  background: transparent;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-muted);
  padding: 4px 10px;
  border-radius: 8px;
  transition: background 0.15s ease, color 0.15s ease;
}

.modal-close:hover {
  background: var(--accent-tint);
  color: var(--text);
}

.modal-body {
  padding: 18px 22px;
  max-height: calc(var(--app-vh, 1vh) * 70);
  overflow-y: auto;
}

.details-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 18px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.detail-label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.detail-value {
  font-size: 14px;
  color: var(--text);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

@media (max-width: 600px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
