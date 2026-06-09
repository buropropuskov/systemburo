<template>
  <section class="permission-groups">
    <header class="page-header">
      <h2 class="page-title">
        Группы прав доступа
      </h2>
      <div class="page-header__actions">
        <button
          class="lk-button lk-button--primary"
          @click="openCreateModal"
        >
          + Создать группу
        </button>
        <RefreshButton
          :loading="loading"
          @refresh="fetch"
        />
      </div>
    </header>

    <div
      v-if="loading"
      class="loading"
    >
      Загрузка...
    </div>

    <div
      v-else-if="groups.length === 0"
      class="empty"
    >
      <p>Пока нет групп прав. Создайте первую — например, «Доступ ко всем таблицам».</p>
    </div>

    <div
      v-else
      class="cards"
    >
      <article
        v-for="group in groups"
        :key="group.id"
        class="card"
      >
        <header class="card__header">
          <h3 class="card__title">
            {{ group.name }}
          </h3>
          <span class="card__count">{{ group.keys.length }} прав</span>
        </header>
        <p
          v-if="group.description"
          class="card__desc"
        >
          {{ group.description }}
        </p>
        <ul class="card__keys">
          <li
            v-for="key in group.keys.slice(0, 5)"
            :key="key"
            class="card__key"
          >
            {{ key }}
          </li>
          <li
            v-if="group.keys.length > 5"
            class="card__key card__key--more"
          >
            +{{ group.keys.length - 5 }} ещё
          </li>
        </ul>
        <footer class="card__footer">
          <button
            class="lk-button lk-button--ghost"
            @click="openEditModal(group)"
          >
            Редактировать
          </button>
          <button
            v-permission-scope="'permission.audit.manage'"
            class="lk-button lk-button--danger"
            @click="confirmDelete(group)"
          >
            Удалить
          </button>
        </footer>
      </article>
    </div>

    <PermissionTreeModal
      :show="modal.show"
      :title="modal.title"
      :initial-keys="modal.initialKeys"
      :available-keys="availableKeys"
      @close="closeModal"
      @save="handleSave"
    />

    <Teleport to="body">
      <div
        v-if="renameOpen"
        class="modal-overlay"
        @click.self="renameOpen = false"
      >
        <div class="rename-modal">
          <h3>{{ renameMode === 'create' ? 'Новая группа прав' : 'Имя и описание' }}</h3>
          <label class="lk-label">
            Название
            <input
              v-model="renameForm.name"
              class="lk-input"
              type="text"
              placeholder="Например, Доступ ко всем таблицам"
            >
          </label>
          <label class="lk-label">
            Описание (опционально)
            <textarea
              v-model="renameForm.description"
              class="lk-textarea"
              rows="2"
            />
          </label>
          <div class="rename-modal__footer">
            <button
              class="lk-button lk-button--ghost"
              @click="renameOpen = false"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="!renameForm.name.trim()"
              @click="confirmRename"
            >
              {{ renameMode === 'create' ? 'Создать и редактировать ключи' : 'Сохранить' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </section>
</template>

<script>
import {
  listPermissionGroups,
  createPermissionGroup,
  updatePermissionGroup,
  deletePermissionGroup,
} from '@/api/permissions';
import PermissionTreeModal from '@/components/admin/PermissionTreeModal.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import { ALL_PERMISSION_KEYS } from '@/constants/permissionKeys';

export default {
  name: 'AdminPermissionGroups',
  components: { PermissionTreeModal, RefreshButton },
  data() {
    return {
      groups: [],
      loading: false,
      modal: { show: false, title: '', initialKeys: [], editingGroup: null },
      renameOpen: false,
      renameMode: 'create',
      renameForm: { name: '', description: '' },
      pendingGroupAfterRename: null,
    };
  },
  computed: {
    availableKeys() {
      return ALL_PERMISSION_KEYS;
    },
  },
  mounted() {
    this.fetch();
  },
  methods: {
    async fetch() {
      this.loading = true;
      try {
        const json = await listPermissionGroups();
        this.groups = Array.isArray(json) ? json : [];
      } catch (e) {
        console.error('Ошибка загрузки групп:', e);
      } finally {
        this.loading = false;
      }
    },
    openCreateModal() {
      this.renameMode = 'create';
      this.renameForm = { name: '', description: '' };
      this.renameOpen = true;
    },
    openEditModal(group) {
      this.modal = {
        show: true,
        title: `Права группы «${group.name}»`,
        initialKeys: group.keys || [],
        editingGroup: group,
      };
    },
    closeModal() {
      this.modal = { show: false, title: '', initialKeys: [], editingGroup: null };
    },
    async handleSave(keys) {
      const group = this.modal.editingGroup;
      if (!group) return;
      try {
        await updatePermissionGroup(group.id, {
          name: group.name,
          description: group.description,
          keys,
        });
        await this.fetch();
        this.closeModal();
      } catch (e) {
        console.error('Ошибка сохранения:', e);
      }
    },
    async confirmRename() {
      if (this.renameMode === 'create') {
        try {
          const created = await createPermissionGroup({
            name: this.renameForm.name.trim(),
            description: this.renameForm.description.trim() || null,
            keys: [],
          });
          this.renameOpen = false;
          await this.fetch();
          if (created && created.id) {
            const group = this.groups.find(g => g.id === created.id);
            if (group) this.openEditModal(group);
          }
        } catch (e) {
          console.error('Ошибка создания:', e);
        }
      }
    },
    async confirmDelete(group) {
      if (!confirm(`Удалить группу «${group.name}»? Это действие нельзя отменить.`)) return;
      try {
        await deletePermissionGroup(group.id);
        await this.fetch();
      } catch (e) {
        console.error('Ошибка удаления:', e);
      }
    },
  },
};
</script>

<style scoped>
.permission-groups {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.page-header__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.loading,
.empty {
  text-align: center;
  padding: 40px;
  color: var(--color-text-muted);
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 14px;
}

.card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  box-shadow: var(--shadow-sm);
}

.card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.card__count {
  font-size: 11px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-pill);
}

.card__desc {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.card__keys {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.card__key {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: var(--color-text);
}

.card__key--more {
  color: var(--color-primary);
  font-weight: 500;
}

.card__footer {
  display: flex;
  gap: 8px;
  margin-top: auto;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
  padding: 20px;
}

.rename-modal {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 20px;
  width: 100%;
  max-width: 480px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rename-modal h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.lk-label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
}

.rename-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
