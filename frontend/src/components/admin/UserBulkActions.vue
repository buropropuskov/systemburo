<template>
  <div
    class="bulk-bar"
    data-testid="users-bulk-bar"
  >
    <span class="bulk-count">Выбрано: {{ selectedCount }}</span>
    <div class="bulk-actions">
      <BaseDropdown
        ref="bulkActionsMenu"
        :options="[]"
        placeholder="Действия"
        :inline="!compactUsers"
        :menu-min-width="200"
        teleport
      >
        <template #content>
          <div
            class="user-action-items"
            @click="closeMenu"
          >
            <template v-if="!showArchive">
              <button
                class="pill pill-ghost"
                data-testid="users-bulk-type"
                @click="$emit('operation', 'type')"
              >
                Тип
              </button>
              <button
                class="pill pill-ghost"
                data-testid="users-bulk-organization"
                @click="$emit('operation', 'organization')"
              >
                Организация
              </button>
              <button
                class="pill pill-ghost"
                data-testid="users-bulk-company"
                @click="$emit('operation', 'company')"
              >
                Компания
              </button>
              <button
                class="pill pill-danger"
                data-testid="users-bulk-ban"
                @click="$emit('ban')"
              >
                Заблокировать
              </button>
              <button
                class="pill pill-ghost"
                data-testid="users-bulk-unban"
                @click="$emit('unban')"
              >
                Разблокировать
              </button>
            </template>
            <button
              v-if="!showArchive"
              class="pill pill-danger"
              data-testid="users-bulk-archive"
              @click="$emit('operation', 'archive')"
            >
              В архив
            </button>
            <button
              v-else
              class="pill pill-restore"
              data-testid="users-bulk-restore"
              @click="$emit('operation', 'restore')"
            >
              Восстановить
            </button>
          </div>
        </template>
      </BaseDropdown>
      <button
        class="pill pill-ghost bulk-clear"
        data-testid="users-bulk-clear"
        @click="$emit('clear')"
      >
        Снять выбор
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'

const props = defineProps({
  selectedCount: { type: Number, required: true },
  showArchive: Boolean,
  compactUsers: Boolean,
})
defineEmits(['operation', 'ban', 'unban', 'clear'])

const bulkActionsMenu = ref(null)
function closeMenu(event) {
  if (props.compactUsers && event.target.closest('button')) bulkActionsMenu.value?.close()
}
</script>

<style scoped>
.bulk-bar {
  /* Оверлей поверх .management-header (не reflow - список не прыгает при выборе,
     урок #510). Высота = высоте шапки (50px), карточка - position:relative. */
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint-solid);
  overflow-x: auto;
  overflow-y: hidden;
}
.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}
.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}
.pill {
  flex: 0 0 auto;
  white-space: nowrap;
}
.bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}
.bulk-clear:hover {
  background: var(--surface-2);
}
.user-action-items {
  display: flex;
  align-items: center;
  flex-wrap: inherit;
  gap: inherit;
}
@media (max-width: 899.98px) {
  .bulk-bar {
    position: static;
    display: grid;
    height: auto;
    padding: 8px 12px;
    gap: 6px;
    overflow: visible;
  }

  .bulk-actions {
    justify-content: space-between;
    margin-left: 0;
  }

  .user-action-items {
    flex-direction: column;
    align-items: stretch;
    overflow-y: auto;
    min-height: 0;
    gap: 6px;
    padding: 8px;
  }

}
</style>
