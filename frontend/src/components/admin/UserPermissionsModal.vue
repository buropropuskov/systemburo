<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="modal-overlay"
      @click.self="$emit('close')"
    >
      <div class="modal-content">
        <header class="modal-content__header">
          <h3>Права пользователя «{{ user?.username }}»</h3>
          <button
            class="close-btn"
            @click="$emit('close')"
          >
            ×
          </button>
        </header>

        <RolesGroupsPanel
          :key="user?.id"
          :user="user"
          @updated="$emit('updated')"
          @close="$emit('close')"
        />
      </div>
    </div>
  </Teleport>
</template>

<script>
import RolesGroupsPanel from './RolesGroupsPanel.vue';

/**
 * Тонкая обёртка-модалка вокруг RolesGroupsPanel. Контракт (show/user/close/updated)
 * сохранён для AdminUsers; вся логика роли/групп/бана живёт в RolesGroupsPanel
 * (он же используется во вкладке UserAccessModal).
 */
export default {
  name: 'UserPermissionsModal',
  components: { RolesGroupsPanel },
  props: {
    show: { type: Boolean, default: false },
    user: { type: Object, default: null },
  },
  emits: ['close', 'updated'],
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.modal-content {
  background: #fff;
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 580px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.2);
}

.modal-content__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border);
}

.modal-content__header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--color-text-muted);
  padding: 0 4px;
}
</style>
