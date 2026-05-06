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

        <div class="modal-content__body">
          <section class="section">
            <label class="section__label">Роль</label>
            <select
              v-model="form.role_id"
              class="lk-select"
            >
              <option :value="null">
                Без роли
              </option>
              <option
                v-for="r in roles"
                :key="r.id"
                :value="r.id"
              >
                {{ r.name }} ({{ r.code }})
              </option>
            </select>
            <p class="section__hint">
              Юзер получит дефолтные группы выбранной роли. Дополнительные группы можно назначить ниже.
            </p>
          </section>

          <section class="section">
            <label class="section__label">Дополнительные группы прав</label>
            <div class="groups-list">
              <label
                v-for="g in groups"
                :key="g.id"
                class="group-row"
              >
                <input
                  type="checkbox"
                  :checked="selectedGroupIds.has(g.id)"
                  @change="toggleGroup(g.id)"
                >
                <span class="group-row__name">{{ g.name }}</span>
                <span class="group-row__count">{{ g.keys.length }} прав</span>
              </label>
              <p
                v-if="groups.length === 0"
                class="section__hint"
              >
                Нет ни одной группы. Создайте на странице «Группы прав».
              </p>
            </div>
            <p
              v-if="selectedGroupIds.size > 1"
              class="section__hint"
            >
              Несколько групп = «Составная группа» (визуально). Кнопка «Слить в новую» создаёт обычную группу с union прав.
            </p>
            <button
              v-if="selectedGroupIds.size > 1"
              type="button"
              class="lk-button lk-button--ghost merge-btn"
              @click="openMerge"
            >
              Слить выбранные в новую группу
            </button>
          </section>

          <section class="section">
            <label class="section__label section__label--danger">
              Блокировка
            </label>
            <div
              v-if="user?.is_banned"
              class="ban-status ban-status--banned"
            >
              <p>Пользователь заблокирован.</p>
              <button
                class="lk-button lk-button--secondary"
                :disabled="actionLoading"
                @click="handleUnban"
              >
                Разблокировать
              </button>
            </div>
            <div
              v-else
              class="ban-status"
            >
              <p>Заблокированный юзер теряет доступ ко всем функциям, кроме ЛК и выхода.</p>
              <button
                class="lk-button lk-button--danger"
                :disabled="actionLoading || user?.is_super_admin"
                @click="handleBan"
              >
                {{ user?.is_super_admin ? 'Нельзя забанить super-admin' : 'Заблокировать' }}
              </button>
            </div>
          </section>
        </div>

        <footer class="modal-content__footer">
          <button
            class="lk-button lk-button--ghost"
            @click="$emit('close')"
          >
            Отмена
          </button>
          <button
            class="lk-button lk-button--primary"
            :disabled="saving"
            @click="save"
          >
            {{ saving ? 'Сохранение...' : 'Сохранить' }}
          </button>
        </footer>

        <Teleport to="body">
          <div
            v-if="mergeOpen"
            class="modal-overlay modal-overlay--nested"
            @click.self="mergeOpen = false"
          >
            <div class="merge-modal">
              <h3>Слияние групп</h3>
              <p class="section__hint">
                Создаётся новая обычная группа с объединением прав. Исходные группы остаются и снова доступны.
              </p>
              <label class="lk-label">
                Имя новой группы
                <input
                  v-model="mergeName"
                  class="lk-input"
                  type="text"
                  placeholder="Например, Менеджер по работе с клиентами"
                >
              </label>
              <div class="merge-modal__footer">
                <button
                  class="lk-button lk-button--ghost"
                  @click="mergeOpen = false"
                >
                  Отмена
                </button>
                <button
                  class="lk-button lk-button--primary"
                  :disabled="!mergeName.trim() || saving"
                  @click="confirmMerge"
                >
                  Слить
                </button>
              </div>
            </div>
          </div>
        </Teleport>
      </div>
    </div>
  </Teleport>
</template>

<script>
import {
  listRoles,
  listPermissionGroups,
  assignGroupToUser,
  unassignGroupFromUser,
  mergePermissionGroups,
  banUser,
  unbanUser,
} from '@/api/permissions';
import { apiRequest } from '@/api/client';

export default {
  name: 'UserPermissionsModal',
  props: {
    show: { type: Boolean, default: false },
    user: { type: Object, default: null },
  },
  emits: ['close', 'updated'],
  data() {
    return {
      roles: [],
      groups: [],
      currentGroupIds: new Set(),
      selectedGroupIds: new Set(),
      form: { role_id: null },
      saving: false,
      actionLoading: false,
      mergeOpen: false,
      mergeName: '',
    };
  },
  watch: {
    async show(v) {
      if (v) await this.fetchAll();
    },
  },
  methods: {
    async fetchAll() {
      try {
        const [rolesJson, groupsJson, currentGroupsJson] = await Promise.all([
          listRoles(),
          listPermissionGroups(),
          this.user ? this.fetchUserGroups(this.user.id) : Promise.resolve([]),
        ]);
        this.roles = rolesJson.data || [];
        this.groups = groupsJson.data || [];
        this.currentGroupIds = new Set(currentGroupsJson.map(g => g.id));
        this.selectedGroupIds = new Set(this.currentGroupIds);
        this.form.role_id = this.user?.role_id ?? null;
      } catch (e) {
        console.error('Ошибка загрузки прав:', e);
      }
    },
    async fetchUserGroups(userId) {
      // Backend пока не предоставляет /users/:id/permission-groups GET; делаем
      // best-effort через /permission-groups (получить все) и проверять
      // membership через server-side bind не будем -- assignToUser
      // FirstOrCreate идемпотентен, поэтому мы можем безопасно вызывать assign
      // на сохранении для каждой выбранной группы. Если backend позже
      // добавит endpoint -- читаем оттуда.
      try {
        const res = await apiRequest(`/users/${userId}/permission-groups`);
        if (res.ok) return await res.json();
      } catch {
        // endpoint может отсутствовать -- считаем пустым
      }
      return [];
    },
    toggleGroup(id) {
      if (this.selectedGroupIds.has(id)) this.selectedGroupIds.delete(id);
      else this.selectedGroupIds.add(id);
      this.selectedGroupIds = new Set(this.selectedGroupIds);
    },
    async save() {
      if (!this.user) return;
      this.saving = true;
      try {
        // Update role
        if ((this.user.role_id ?? null) !== this.form.role_id) {
          await apiRequest(`/users/${this.user.id}/role`, {
            method: 'PUT',
            body: JSON.stringify({ role_id: this.form.role_id }),
          });
        }
        // Diff groups
        for (const id of this.selectedGroupIds) {
          if (!this.currentGroupIds.has(id)) {
            await assignGroupToUser(this.user.id, id);
          }
        }
        for (const id of this.currentGroupIds) {
          if (!this.selectedGroupIds.has(id)) {
            await unassignGroupFromUser(this.user.id, id);
          }
        }
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка сохранения:', e);
      } finally {
        this.saving = false;
      }
    },
    async handleBan() {
      if (!confirm('Заблокировать пользователя? Все его активные сессии будут завершены.')) return;
      this.actionLoading = true;
      try {
        await banUser(this.user.id);
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка бана:', e);
      } finally {
        this.actionLoading = false;
      }
    },
    async handleUnban() {
      if (!confirm('Снять блокировку?')) return;
      this.actionLoading = true;
      try {
        await unbanUser(this.user.id);
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка разбана:', e);
      } finally {
        this.actionLoading = false;
      }
    },
    openMerge() {
      this.mergeName = '';
      this.mergeOpen = true;
    },
    async confirmMerge() {
      this.saving = true;
      try {
        await mergePermissionGroups({
          user_id: this.user.id,
          source_group_ids: Array.from(this.selectedGroupIds),
          new_group_name: this.mergeName.trim(),
        });
        await this.fetchAll();
        this.mergeOpen = false;
      } catch (e) {
        console.error('Ошибка слияния:', e);
      } finally {
        this.saving = false;
      }
    },
  },
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

.modal-overlay--nested {
  z-index: 1100;
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

.modal-content__body {
  padding: 16px 20px;
  overflow-y: auto;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.section__label--danger {
  color: var(--color-danger);
}

.section__hint {
  font-size: 11px;
  color: var(--color-text-muted);
  margin: 0;
}

.groups-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 240px;
  overflow-y: auto;
  padding: 4px 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.group-row {
  display: grid;
  grid-template-columns: 18px 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  cursor: pointer;
  font-size: 13px;
}

.group-row:hover {
  background: var(--color-bg);
}

.group-row__count {
  font-size: 11px;
  color: var(--color-text-muted);
}

.merge-btn {
  align-self: flex-start;
}

.ban-status {
  background: var(--color-bg);
  padding: 12px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.ban-status p {
  margin: 0;
  font-size: 12px;
  color: var(--color-text);
  flex: 1;
}

.ban-status--banned {
  background: #fee2e2;
}

.modal-content__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--color-border);
}

.merge-modal {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 20px;
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.merge-modal h3 {
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

.merge-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
