<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      :class="{ 'modal-leaving': leaving }"
      data-testid="user-access-modal"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <div
        class="access-modal"
        @mousedown.stop
      >
        <header class="access-modal__header">
          <h3>Права доступа «{{ user?.username }}»</h3>
          <button
            class="close-btn"
            aria-label="Закрыть"
            @click="close"
          >
            ×
          </button>
        </header>

        <div class="access-modal__tabs">
          <button
            class="access-tab"
            :class="{ 'access-tab--active': activeTab === 'roles' }"
            data-testid="access-tab-roles"
            @click="activeTab = 'roles'"
          >
            Роль и группы
          </button>
          <button
            class="access-tab"
            :class="{ 'access-tab--active': activeTab === 'individual' }"
            data-testid="access-tab-individual"
            @click="activeTab = 'individual'"
          >
            Индивидуальные права
          </button>
        </div>

        <RolesGroupsPanel
          v-if="activeTab === 'roles'"
          :user="user"
          @updated="$emit('updated')"
          @close="close"
        />
        <IndividualPermissionsPanel
          v-else
          :user="user"
          @updated="$emit('updated')"
          @close="close"
        />
      </div>
    </div>
  </Teleport>
</template>

<script>
import { useOverlayClose } from '@/composables/useOverlayClose';
import RolesGroupsPanel from './RolesGroupsPanel.vue';
import IndividualPermissionsPanel from './IndividualPermissionsPanel.vue';

/**
 * Единая модалка «Права доступа» с двумя вкладками: роль/группы/бан и индивидуальные
 * права (дерево ключей). Заменяет две отдельные кнопки в UserControl.
 */
export default {
  name: 'UserAccessModal',
  components: { RolesGroupsPanel, IndividualPermissionsPanel },
  props: {
    user: { type: Object, default: null },
  },
  emits: ['close', 'updated'],
  setup() {
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      activeTab: 'roles',
      leaving: false,
    };
  },
  created() {
    this.overlay.close = () => this.close();
  },
  mounted() {
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.close();
    },
    close() {
      if (this.leaving) return;
      this.leaving = true;
      setTimeout(() => this.$emit('close'), 250);
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 12000;
  padding: 20px;
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-overlay.modal-leaving {
  animation: fadeOut 0.25s ease-in forwards;
}

.modal-overlay.modal-leaving .access-modal {
  animation: slideDown 0.25s ease-in forwards;
}

@keyframes fadeOut {
  from { opacity: 1; }
  to { opacity: 0; }
}

@keyframes slideDown {
  from { transform: translateY(0); opacity: 1; }
  to { transform: translateY(20px); opacity: 0; }
}

.access-modal {
  background: #fff;
  border-radius: 30px;
  width: 100%;
  max-width: 600px;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.access-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
  border-bottom: 1px solid var(--color-border);
}

.access-modal__header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.close-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: #999;
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.close-btn:hover {
  color: #333;
  background: #f5f5f5;
}

.access-modal__tabs {
  display: flex;
  gap: 4px;
  padding: 10px 24px 0;
  border-bottom: 1px solid var(--color-border);
}

.access-tab {
  padding: 8px 16px;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-muted);
  border-bottom: 2px solid transparent;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.access-tab:hover {
  color: var(--color-text);
}

.access-tab--active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
}
</style>
