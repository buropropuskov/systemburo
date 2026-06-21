<template>
  <section class="roles">
    <header class="page-header">
      <h2 class="page-title">
        Роли пользователей
      </h2>
      <div class="page-header__actions">
        <button
          class="lk-button lk-button--primary"
          @click="openCreate"
        >
          + Создать роль
        </button>
        <RefreshButton
          :loading="loading"
          @refresh="fetchAll"
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
      <transition name="modal-fade">
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
      </transition>
    </Teleport>

    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="groupsOpen"
          class="modal-overlay"
          @click.self="groupsOpen = false"
        >
          <div class="form-modal roles-groups-modal">
            <header class="rgm-head">
              <h3>Дефолтные группы для «{{ groupsRole?.name }}»</h3>
              <p class="form-modal__hint">
                Юзеры с этой ролью получают права из всех выбранных групп. Справа — итоговый набор прав роли.
              </p>
            </header>
            <div class="rgm-body">
              <div class="rgm-col rgm-col--left">
                <div class="rgm-col__title">
                  Группы прав
                </div>
                <div class="groups-list">
                  <div
                    v-for="g in allGroups"
                    :key="g.id"
                    class="group-row"
                    @click="toggleGroupId(g.id)"
                  >
                    <span class="group-row__name">{{ g.name }}</span>
                    <span class="group-row__count">{{ g.keys.length }} прав</span>
                    <button
                      type="button"
                      class="tgl"
                      :class="{ on: selectedGroupIds.has(g.id) }"
                      :aria-pressed="selectedGroupIds.has(g.id)"
                      :aria-label="g.name"
                      :data-group-id="g.id"
                      @click.stop="toggleGroupId(g.id)"
                    />
                  </div>
                  <p
                    v-if="allGroups.length === 0"
                    class="card__empty-text"
                  >
                    Нет ни одной группы. Сначала создайте группы прав.
                  </p>
                </div>
              </div>
              <div class="rgm-col rgm-col--right">
                <div class="rgm-col__title">
                  Итоговые права роли
                </div>
                <EffectivePermissionsTree
                  :catalog="catalog"
                  :state-by-key="previewStateByKey"
                />
              </div>
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
      </transition>
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
  getPermissionCatalog,
} from '@/api/permissions';
import RefreshButton from '@/components/RefreshButton.vue';
import EffectivePermissionsTree from '@/components/admin/EffectivePermissionsTree.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

export default {
  name: 'AdminRoles',
  components: { RefreshButton, EffectivePermissionsTree },
  data() {
    return {
      roles: [],
      allGroups: [],
      catalog: [],
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
  computed: {
    // Итоговый набор прав роли = объединение ключей всех выбранных дефолтных групп.
    // Read-only превью (все locked, источник «группа») в стиле карточки прав.
    previewStateByKey() {
      const union = new Set();
      for (const g of this.allGroups) {
        if (this.selectedGroupIds.has(g.id)) {
          for (const k of g.keys || []) union.add(k);
        }
      }
      const result = {};
      for (const node of this.catalog) {
        result[node.key] = { on: union.has(node.key), source: 'group', locked: true };
        for (const child of node.children || []) {
          result[child.key] = { on: union.has(child.key), source: 'group', locked: true };
        }
      }
      return result;
    },
  },
  mounted() {
    this.fetchAll();
  },
  methods: {
    async fetchAll() {
      this.loading = true;
      try {
        const [roles, groups, catalog] = await Promise.all([
          listRoles(),
          listPermissionGroups(),
          getPermissionCatalog(),
        ]);
        this.roles = Array.isArray(roles) ? roles : [];
        this.allGroups = Array.isArray(groups) ? groups : [];
        this.catalog = Array.isArray(catalog) ? catalog : [];
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
      const ok = await useUiStore().confirm({
        title: 'Удалить роль?',
        message: `Роль «${role.name}» будет удалена. Если у неё есть привязанные пользователи, бэкенд откажет.`,
        confirmText: 'Удалить',
        cancelText: 'Отмена',
        danger: true,
      });
      if (!ok) return;
      try {
        await deleteRole(role.id);
        await this.fetchAll();
        useDeletionsStore().notify({ prefix: 'Роль ', bold: role.name, suffix: ' удалена' });
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось удалить ', bold: role.name, suffix: ': возможно, есть привязанные пользователи', type: 'error' });
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

.page-header__actions {
  display: flex;
  align-items: center;
  gap: 12px;
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

.roles-groups-modal {
  max-width: 880px;
  max-height: 86vh;
  border-radius: 30px;
  padding: 24px;
}

.rgm-head h3 {
  margin: 0 0 4px;
}

.rgm-body {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 18px;
  margin-top: 8px;
  min-height: 0;
  overflow: hidden;
}

.rgm-col {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.rgm-col--right {
  border-left: 1px solid var(--color-border);
  padding-left: 18px;
}

.rgm-col__title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--color-text-muted);
  margin-bottom: 8px;
}

.rgm-col--right :deep(.ep-tree) {
  overflow-y: auto;
  max-height: 56vh;
  padding-right: 4px;
}

@media (max-width: 720px) {
  .rgm-body {
    grid-template-columns: 1fr;
  }
  .rgm-col--right {
    border-left: none;
    padding-left: 0;
    border-top: 1px solid var(--color-border);
    padding-top: 14px;
  }
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
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s ease;
}

.group-row:hover {
  background: var(--color-bg);
}

.group-row__name {
  font-weight: 500;
}

.group-row__count {
  font-size: 11px;
  color: var(--color-text-muted);
}

.tgl {
  --w: 40px;
  --h: 23px;
  --d: 17px;
  width: var(--w);
  height: var(--h);
  flex: none;
  border-radius: var(--radius-pill);
  background: #d3d6e4;
  position: relative;
  cursor: pointer;
  border: none;
  padding: 0;
  transition: background 0.2s ease;
}

.tgl::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: var(--d);
  height: var(--d);
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
  transition: left 0.2s ease;
}

.tgl.on { background: var(--color-primary); }
.tgl.on::after { left: calc(var(--w) - var(--d) - 3px); }

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
