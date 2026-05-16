<template>
  <section class="roles">
    <header class="page-header">
      <h2 class="page-title">
        Роли пользователей
      </h2>
      <button
        class="lk-button lk-button--primary"
        @click="openCreate"
      >
        + Создать роль
      </button>
    </header>

    <div
      v-if="loading"
      class="loading"
    >
      Загрузка...
    </div>

    <div
      v-else-if="roles.length === 0"
      class="empty"
    >
      <p>Пока нет ролей. Создайте, например, «Арендатор», «Охранник», «Руководитель».</p>
    </div>

    <div
      v-else
      class="cards"
    >
      <article
        v-for="role in roles"
        :key="role.id"
        class="card"
      >
        <header class="card__header">
          <div>
            <h3 class="card__title">
              {{ role.name }}
            </h3>
            <code class="card__code">{{ role.code }}</code>
          </div>
        </header>
        <p
          v-if="role.description"
          class="card__desc"
        >
          {{ role.description }}
        </p>

        <div class="card__section">
          <span class="card__section-label">Дефолтные группы:</span>
          <div class="card__chips">
            <span
              v-for="g in role.default_groups || []"
              :key="g.id"
              class="chip"
            >
              {{ g.name }}
            </span>
            <span
              v-if="(role.default_groups || []).length === 0"
              class="card__empty-text"
            >не настроены</span>
          </div>
        </div>

        <footer class="card__footer">
          <button
            class="lk-button lk-button--ghost"
            @click="openEditGroups(role)"
          >
            Настроить группы
          </button>
          <button
            class="lk-button lk-button--ghost"
            @click="openEditMeta(role)"
          >
            Изменить
          </button>
          <button
            v-permission-scope="'permission.audit.manage'"
            class="lk-button lk-button--danger"
            @click="confirmDelete(role)"
          >
            Удалить
          </button>
        </footer>
      </article>
    </div>

    <Teleport to="body">
      <div
        v-if="metaOpen"
        class="modal-overlay"
        @click.self="metaOpen = false"
      >
        <div class="form-modal">
          <h3>{{ metaMode === 'create' ? 'Новая роль' : 'Редактировать роль' }}</h3>
          <label class="lk-label">
            Название
            <input
              v-model="metaForm.name"
              class="lk-input"
              type="text"
              placeholder="Арендатор"
            >
          </label>
          <label
            v-if="metaMode === 'create'"
            class="lk-label"
          >
            Код (латиницей, неизменный)
            <input
              v-model="metaForm.code"
              class="lk-input"
              type="text"
              placeholder="tenant"
            >
          </label>
          <label class="lk-label">
            Описание
            <textarea
              v-model="metaForm.description"
              class="lk-textarea"
              rows="2"
            />
          </label>
          <div class="form-modal__footer">
            <button
              class="lk-button lk-button--ghost"
              @click="metaOpen = false"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="saving || !metaForm.name.trim() || (metaMode === 'create' && !metaForm.code.trim())"
              @click="saveMeta"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="groupsOpen"
        class="modal-overlay"
        @click.self="groupsOpen = false"
      >
        <div class="form-modal form-modal--wide">
          <h3>Дефолтные группы для «{{ groupsRole?.name }}»</h3>
          <p class="form-modal__hint">
            Юзеры с этой ролью получают права из всех выбранных групп.
          </p>
          <div class="groups-list">
            <label
              v-for="g in allGroups"
              :key="g.id"
              class="group-row"
            >
              <input
                type="checkbox"
                :checked="selectedGroupIds.has(g.id)"
                @change="toggleGroupId(g.id)"
              >
              <span class="group-row__name">{{ g.name }}</span>
              <span class="group-row__count">{{ g.keys.length }} прав</span>
            </label>
            <p
              v-if="allGroups.length === 0"
              class="card__empty-text"
            >
              Нет ни одной группы. Сначала создайте группы прав.
            </p>
          </div>
          <div class="form-modal__footer">
            <button
              class="lk-button lk-button--ghost"
              @click="groupsOpen = false"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="saving"
              @click="saveDefaultGroups"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </section>
</template>

<script>
import {
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  setRoleDefaultGroups,
  listPermissionGroups,
} from '@/api/permissions';

export default {
  name: 'AdminRoles',
  data() {
    return {
      roles: [],
      allGroups: [],
      loading: false,
      saving: false,
      metaOpen: false,
      metaMode: 'create',
      metaForm: { name: '', code: '', description: '' },
      metaEditing: null,
      groupsOpen: false,
      groupsRole: null,
      selectedGroupIds: new Set(),
    };
  },
  mounted() {
    this.fetchAll();
  },
  methods: {
    async fetchAll() {
      this.loading = true;
      try {
        const [roles, groups] = await Promise.all([listRoles(), listPermissionGroups()]);
        this.roles = Array.isArray(roles) ? roles : [];
        this.allGroups = Array.isArray(groups) ? groups : [];
      } finally {
        this.loading = false;
      }
    },
    openCreate() {
      this.metaMode = 'create';
      this.metaForm = { name: '', code: '', description: '' };
      this.metaEditing = null;
      this.metaOpen = true;
    },
    openEditMeta(role) {
      this.metaMode = 'edit';
      this.metaForm = { name: role.name, code: role.code, description: role.description || '' };
      this.metaEditing = role;
      this.metaOpen = true;
    },
    async saveMeta() {
      this.saving = true;
      try {
        if (this.metaMode === 'create') {
          await createRole({
            name: this.metaForm.name.trim(),
            code: this.metaForm.code.trim(),
            description: this.metaForm.description.trim() || null,
          });
        } else {
          await updateRole(this.metaEditing.id, {
            name: this.metaForm.name.trim(),
            description: this.metaForm.description.trim() || null,
          });
        }
        await this.fetchAll();
        this.metaOpen = false;
      } finally {
        this.saving = false;
      }
    },
    openEditGroups(role) {
      this.groupsRole = role;
      this.selectedGroupIds = new Set((role.default_groups || []).map(g => g.id));
      this.groupsOpen = true;
    },
    toggleGroupId(id) {
      if (this.selectedGroupIds.has(id)) this.selectedGroupIds.delete(id);
      else this.selectedGroupIds.add(id);
      this.selectedGroupIds = new Set(this.selectedGroupIds);
    },
    async saveDefaultGroups() {
      if (!this.groupsRole) return;
      this.saving = true;
      try {
        await setRoleDefaultGroups(this.groupsRole.id, Array.from(this.selectedGroupIds));
        await this.fetchAll();
        this.groupsOpen = false;
      } finally {
        this.saving = false;
      }
    },
    async confirmDelete(role) {
      if (!confirm(`Удалить роль «${role.name}»? Если у неё есть юзеры, удаление будет отклонено бэкендом.`)) return;
      try {
        await deleteRole(role.id);
        await this.fetchAll();
      } catch {
        alert('Ошибка удаления роли. Возможно, у неё есть привязанные пользователи.');
      }
    },
  },
};
</script>

<style scoped>
.roles {
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

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.loading,
.empty {
  text-align: center;
  padding: 40px;
  color: var(--color-text-muted);
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 14px;
}

.card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: var(--shadow-sm);
}

.card__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.card__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.card__code {
  display: inline-block;
  margin-top: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.card__desc {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.card__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.card__section-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.card__chips {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.chip {
  font-size: 11px;
  padding: 3px 8px;
  background: #eef0ff;
  color: #3a45c0;
  border-radius: var(--radius-pill);
}

.card__empty-text {
  font-size: 12px;
  color: var(--color-text-muted);
  font-style: italic;
}

.card__footer {
  display: flex;
  gap: 8px;
  margin-top: auto;
  flex-wrap: wrap;
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

.form-modal {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 20px;
  width: 100%;
  max-width: 480px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-modal--wide {
  max-width: 560px;
  max-height: 80vh;
}

.form-modal h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.form-modal__hint {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.lk-label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
}

.form-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.groups-list {
  max-height: 50vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
}

.group-row {
  display: grid;
  grid-template-columns: 18px 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s ease;
}

.group-row:hover {
  background: var(--color-bg);
}

.group-row__count {
  font-size: 11px;
  color: var(--color-text-muted);
}
</style>
