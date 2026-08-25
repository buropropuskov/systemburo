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
        <header class="access-modal__head">
          <div class="access-modal__head-text">
            <div class="access-modal__title">
              Права доступа: {{ displayName }}
            </div>
            <div class="access-modal__sub">
              {{ subtitle }}
            </div>
          </div>
          <button
            class="close-btn"
            aria-label="Закрыть"
            @click="close"
          >
            ×
          </button>
        </header>

        <div class="access-modal__body">
          <!-- ЛЕВО: источники прав + блокировка -->
          <div class="col-left">
            <div class="admin-toggle">
              <div class="admin-toggle__txt">
                <b>Администратор</b>
                <span>Все права. Можно точечно выключать справа.</span>
                <small
                  v-if="adminLockReason"
                  class="admin-toggle__lock"
                  data-testid="admin-toggle-lock-reason"
                >
                  {{ adminLockReason }}
                </small>
              </div>
              <button
                type="button"
                class="tgl"
                data-testid="admin-toggle"
                :class="{ on: localIsAdmin || isSuper, locked: adminLocked }"
                :disabled="adminLocked || saving"
                :title="adminLockReason"
                :aria-pressed="localIsAdmin || isSuper"
                aria-label="Администратор"
                @click="toggleAdmin"
              />
            </div>

            <div class="field-block">
              <div class="field-label">
                Роль
              </div>
              <BaseDropdown
                v-model="form.role_id"
                data-testid="role-select"
                :options="roleOptions"
                placeholder="Без роли"
                :disabled="saving"
              />
            </div>

            <div class="field-block">
              <div class="field-label">
                Группы прав
              </div>
              <div class="chips">
                <span
                  v-for="g in selectedGroupsList"
                  :key="g.id"
                  class="chip"
                >
                  {{ g.name }}
                  <span
                    class="chip__x"
                    role="button"
                    aria-label="Убрать группу"
                    @click="removeGroup(g.id)"
                  >×</span>
                </span>
                <button
                  v-if="availableGroups.length"
                  type="button"
                  class="chip chip--add"
                  @click="showAddGroups = !showAddGroups"
                >
                  + добавить
                </button>
                <span
                  v-else-if="selectedGroupsList.length === 0"
                  class="field-empty"
                >
                  Нет групп
                </span>
              </div>
              <div
                v-if="showAddGroups && availableGroups.length"
                class="add-groups"
              >
                <button
                  v-for="g in availableGroups"
                  :key="g.id"
                  type="button"
                  class="add-groups__item"
                  @click="addGroup(g.id)"
                >
                  <span>{{ g.name }}</span>
                  <small>{{ (g.keys || []).length }} прав</small>
                </button>
              </div>
            </div>

            <hr class="divider">

            <div class="ban-box">
              <div class="ban-box__title">
                <svg
                  width="15"
                  height="15"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <rect
                    x="4"
                    y="11"
                    width="16"
                    height="10"
                    rx="2"
                  />
                  <path d="M8 11V7a4 4 0 0 1 8 0v4" />
                </svg>
                Блокировка
              </div>
              <template v-if="isBanned">
                <p class="ban-box__current">
                  Пользователь заблокирован.
                  <template v-if="banReasonCurrent">
                    Причина: «{{ banReasonCurrent }}»
                  </template>
                </p>
                <button
                  type="button"
                  class="lk-button lk-button--secondary ban-box__btn"
                  data-testid="unban-button"
                  :disabled="banActionLoading"
                  @click="handleUnban"
                >
                  Разблокировать
                </button>
              </template>
              <template v-else>
                <textarea
                  v-model="banReasonInput"
                  class="lk-textarea"
                  placeholder="Причина блокировки (увидит пользователь)"
                  :disabled="isSuper || banActionLoading"
                />
                <button
                  type="button"
                  class="lk-button lk-button--danger ban-box__btn"
                  data-testid="ban-button"
                  :disabled="isSuper || banActionLoading"
                  :title="isSuper ? 'Супер-администратора заблокировать нельзя' : ''"
                  @click="handleBan"
                >
                  {{ isSuper ? 'Невозможно' : 'Заблокировать' }}
                </button>
              </template>
            </div>
          </div>

          <!-- ПРАВО: точечные права -->
          <div class="col-right">
            <div class="legend">
              <span>Источник:</span>
              <span class="src src--role">роль</span>
              <span class="src src--group">группа</span>
              <span class="src src--override">лично</span>
            </div>

            <div class="perm-search">
              <input
                v-model="search"
                class="lk-input"
                type="text"
                placeholder="Поиск права..."
                data-testid="user-permissions-search"
              >
            </div>

            <LoaderSpinner
              v-if="loading"
              label="Загрузка прав..."
            />
            <EffectivePermissionsTree
              v-else
              :catalog="filteredCatalog"
              :state-by-key="stateByKey"
              :expand-all="searchActive"
              @toggle="onToggleKey"
            />
          </div>
        </div>

        <footer class="access-modal__foot">
          <span class="access-modal__foot-hint">
            Изменения вступят в силу в течение 30 секунд
          </span>
          <div class="access-modal__foot-actions">
            <button
              type="button"
              class="lk-button lk-button--ghost"
              data-testid="cancel-button"
              @click="close"
            >
              Отмена
            </button>
            <button
              type="button"
              class="lk-button lk-button--primary"
              data-testid="save-button"
              :disabled="saving || loading"
              @click="save"
            >
              {{ saving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useAuthStore } from '@/stores/auth';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useUiStore } from '@/stores/ui';
import {
  listRoles,
  listPermissionGroups,
  assignGroupToUser,
  unassignGroupFromUser,
  banUser,
  unbanUser,
  getUserPermissions,
  updateUserPermissions,
  getUserEffectivePermissions,
  getPermissionCatalog,
  setUserAdmin,
  setUserRole,
} from '@/api/permissions';
import { apiRequest } from '@/api/client';
import EffectivePermissionsTree from './EffectivePermissionsTree.vue';
import { filterCatalog, flattenCatalog } from '@/utils/permissionCatalog';
import LoaderSpinner from '../ui/LoaderSpinner.vue';
import BaseDropdown from '../ui/BaseDropdown.vue';

// Ключ, которым бэкенд закрывает PUT /users/:id/admin (services.KeyActionGrantAdmin).
const GRANT_ADMIN_KEY = 'action.grant.admin';

/**
 * Модалка «Права доступа» в две колонки: слева источники прав (флаг Администратор,
 * роль, группы прав, блокировка), справа -- дерево эффективных прав с бейджем
 * источника и тумблерами личных override. Один футер с unified-save (admin + роль +
 * группы + override одним нажатием); блокировка -- отдельное немедленное действие.
 */
export default {
  name: 'UserAccessModal',
  components: { EffectivePermissionsTree, LoaderSpinner, BaseDropdown },
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
      leaving: false,
      loading: false,
      saving: false,
      banActionLoading: false,
      catalog: [],
      roles: [],
      groups: [],
      currentGroupIds: new Set(),
      selectedGroupIds: new Set(),
      showAddGroups: false,
      form: { role_id: null },
      initialIsAdmin: false,
      localIsAdmin: false,
      effective: {},
      inheritedAllow: new Set(),
      inheritedSource: {},
      overrideInit: {},
      overrideMap: {},
      banReasonInput: '',
      search: '',
    };
  },
  computed: {
    roleOptions() {
      return [{ id: null, name: 'Без роли' }, ...this.roles];
    },
    isSuper() {
      return this.effective.mode === 'super' || !!this.user?.is_super_admin;
    },
    isBanned() {
      return this.effective.mode === 'banned' || !!this.effective.banned || !!this.user?.is_banned;
    },
    // Признак администратора выдаёт action.grant.admin, а он super-only. Одной
    // hasPermission тут мало: в режиме admin стор отвечает «да» на любой ключ, которого
    // нет в denied (stores/permissions.js), а super-only ключи туда не попадают -- бэкенд
    // же отказывает всем, кроме супера (PermissionSet.Has). Признак берём из каталога,
    // он приходит тем же запросом, что и права.
    canGrantAdmin() {
      const node = this.flatCatalog.find((n) => n.key === GRANT_ADMIN_KEY);
      // Каталог ещё не приехал -- считаем ключ закрытым, иначе тумблер успевает
      // побыть доступным, пока грузятся права.
      const superOnly = node ? !!node.super_only : true;
      if (superOnly && !useAuthStore().isSuperAdmin) return false;
      return usePermissionsStore().hasPermission(GRANT_ADMIN_KEY);
    },
    adminLocked() {
      return this.isSuper || !this.canGrantAdmin;
    },
    // Причина недоступности тумблера. Про целевого пользователя -- первой: она
    // держится даже у того, кому выдавать администраторов разрешено.
    adminLockReason() {
      if (this.isSuper) return 'У супер-администратора и так все права';
      if (!this.canGrantAdmin) return 'Выдать признак может только Системный администратор';
      return '';
    },
    banReasonCurrent() {
      return this.effective.ban_reason || this.user?.ban_reason || '';
    },
    displayName() {
      const fio = [this.user?.last_name, this.user?.first_name, this.user?.middle_name]
        .filter(Boolean)
        .join(' ')
        .trim();
      return fio || this.user?.username || '';
    },
    subtitle() {
      const parts = [];
      if (this.user?.username) parts.push(`@${this.user.username}`);
      if (this.user?.user_type) parts.push(`Тип: ${this.user.user_type}`);
      if (this.user?.organization) parts.push(this.user.organization);
      return parts.join(' · ');
    },
    selectedGroupsList() {
      return this.groups.filter((g) => this.selectedGroupIds.has(g.id));
    },
    availableGroups() {
      return this.groups.filter((g) => !this.selectedGroupIds.has(g.id));
    },
    // Плоский список и состояния считаются по ПОЛНОМУ каталогу, а поиск сужает
    // только то, что рисует дерево: иначе набранный запрос обнулял бы состояние
    // спрятанных прав, и сохранение уносило бы их вместе с собой.
    flatCatalog() {
      return flattenCatalog(this.catalog);
    },
    filteredCatalog() {
      return filterCatalog(this.catalog, this.search);
    },
    searchActive() {
      return this.search.trim().length > 0;
    },
    stateByKey() {
      const result = {};
      for (const node of this.flatCatalog) {
        result[node.key] = this.computeState(node);
      }
      return result;
    },
  },
  watch: {
    // Дерево тумблеров отражает наследованные права (роль + её default-группы +
    // назначенные группы). Пересчитываем сразу при смене роли/групп, чтобы не
    // требовалось переоткрывать модалку (#867 UX).
    'form.role_id'() {
      this.recomputeInherited();
    },
    selectedGroupIds() {
      this.recomputeInherited();
    },
    roles() {
      this.recomputeInherited();
    },
  },
  created() {
    this.overlay.close = () => this.close();
  },
  mounted() {
    document.addEventListener('keydown', this.onKeydown);
    this.load();
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
    // Наследованные allow/source = гранты выбранной роли (direct + её default-группы)
    // + ключи назначенных групп. Зовётся из load() и watch'ей роли/групп.
    recomputeInherited() {
      const allow = new Set();
      const source = {};
      const role = this.roles.find((r) => r.id === this.form.role_id) || null;
      if (role) {
        for (const g of role.default_groups || []) {
          for (const k of g.keys || []) {
            allow.add(k);
            source[k] = 'group';
          }
        }
        for (const k of role.direct_grants || []) {
          allow.add(k);
          source[k] = 'role';
        }
      }
      for (const gid of this.selectedGroupIds) {
        const g = this.groups.find((x) => x.id === gid);
        if (g) {
          for (const k of g.keys || []) {
            allow.add(k);
            source[k] = 'group';
          }
        }
      }
      this.inheritedAllow = allow;
      this.inheritedSource = source;
    },
    async load() {
      if (!this.user) return;
      this.loading = true;
      const uid = this.user.id;
      try {
        const [catalog, effective, overrides, roles, groups, userGroups] = await Promise.all([
          getPermissionCatalog(),
          getUserEffectivePermissions(uid),
          getUserPermissions(uid),
          listRoles(),
          listPermissionGroups(),
          this.fetchUserGroups(uid),
        ]);

        this.catalog = Array.isArray(catalog) ? catalog : [];
        this.roles = Array.isArray(roles) ? roles : [];
        this.groups = Array.isArray(groups) ? groups : [];
        this.currentGroupIds = new Set((userGroups || []).map((g) => g.id));
        this.selectedGroupIds = new Set(this.currentGroupIds);
        this.form.role_id = this.user.role_id ?? null;

        this.effective = effective && typeof effective === 'object' ? effective : {};
        this.initialIsAdmin = this.effective.mode === 'admin' || !!this.user.is_admin;
        this.localIsAdmin = this.initialIsAdmin;

        // inheritedAllow/inheritedSource теперь computed (от выбранной роли/групп) -
        // снимок из effective тут больше не нужен, иначе он бы "замораживал" дерево.

        const ov = {};
        if (Array.isArray(overrides)) {
          for (const p of overrides) ov[p.key] = p.value;
        }
        this.overrideInit = { ...ov };
        this.overrideMap = { ...ov };

        this.recomputeInherited();

        this.banReasonInput = this.banReasonCurrent;
      } catch (e) {
        console.error('Ошибка загрузки прав:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'права доступа', type: 'error' });
      } finally {
        this.loading = false;
      }
    },
    async fetchUserGroups(userId) {
      try {
        const res = await apiRequest(`/users/${userId}/permission-groups`);
        if (res.ok) return await res.json();
      } catch {
        // endpoint может отсутствовать -- считаем пустым
      }
      return [];
    },
    computeState(node) {
      const key = node.key;
      if (node.super_only) {
        return { on: this.isSuper, source: this.isSuper ? 'admin' : null, locked: true };
      }
      if (this.isSuper) {
        return { on: true, source: 'admin', locked: true };
      }
      if (this.isBanned) {
        return { on: false, source: null, locked: true };
      }
      if (Object.prototype.hasOwnProperty.call(this.overrideMap, key)) {
        return { on: this.overrideMap[key] === 'allow', source: 'override', locked: false };
      }
      if (this.localIsAdmin) {
        return { on: true, source: 'admin', locked: false };
      }
      if (this.inheritedAllow.has(key)) {
        return { on: true, source: this.inheritedSource[key], locked: false };
      }
      return { on: false, source: null, locked: false };
    },
    toggleAdmin() {
      if (this.adminLocked || this.saving) return;
      this.localIsAdmin = !this.localIsAdmin;
    },
    onToggleKey(key) {
      const st = this.stateByKey[key];
      if (!st || st.locked) return;
      const newOn = !st.on;
      // Явный override: PUT /permissions/user/:id делает upsert без удаления, поэтому
      // тумблер всегда задаёт явный allow/deny (источник права становится «лично»).
      this.overrideMap = { ...this.overrideMap, [key]: newOn ? 'allow' : 'deny' };
    },
    addGroup(id) {
      this.selectedGroupIds = new Set(this.selectedGroupIds).add(id);
      if (!this.availableGroups.length) this.showAddGroups = false;
    },
    removeGroup(id) {
      const next = new Set(this.selectedGroupIds);
      next.delete(id);
      this.selectedGroupIds = next;
    },
    async save() {
      if (!this.user) return;
      this.saving = true;
      const uid = this.user.id;
      try {
        if (!this.isSuper && this.localIsAdmin !== this.initialIsAdmin) {
          await setUserAdmin(uid, this.localIsAdmin);
        }
        if ((this.user.role_id ?? null) !== this.form.role_id) {
          await setUserRole(uid, this.form.role_id);
        }
        for (const id of this.selectedGroupIds) {
          if (!this.currentGroupIds.has(id)) await assignGroupToUser(uid, id);
        }
        for (const id of this.currentGroupIds) {
          if (!this.selectedGroupIds.has(id)) await unassignGroupFromUser(uid, id);
        }
        const changed = [];
        for (const key of Object.keys(this.overrideMap)) {
          if (this.overrideMap[key] !== this.overrideInit[key]) {
            changed.push({ key, value: this.overrideMap[key] });
          }
        }
        if (changed.length) {
          const result = await updateUserPermissions(uid, { permissions: changed });
          if (result && result.message) {
            useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: result.message, type: 'error' });
            return;
          }
        }
        useDeletionsStore().notify({ prefix: 'Сохранены ', bold: 'права доступа' });
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка сохранения прав:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить ', bold: 'права доступа', type: 'error' });
      } finally {
        this.saving = false;
      }
    },
    async handleBan() {
      if (this.isSuper) return;
      const ok = await useUiStore().confirm({
        title: 'Заблокировать пользователя?',
        message: 'Все активные сессии пользователя будут завершены.',
        confirmText: 'Заблокировать',
        cancelText: 'Отмена',
        danger: true,
      });
      if (!ok) return;
      this.banActionLoading = true;
      try {
        await banUser(this.user.id, this.banReasonInput.trim());
        useDeletionsStore().notify({ prefix: 'Пользователь ', bold: this.user.username, suffix: ' заблокирован' });
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка бана:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'заблокировать пользователя', type: 'error' });
      } finally {
        this.banActionLoading = false;
      }
    },
    async handleUnban() {
      const ok = await useUiStore().confirm({
        title: 'Снять блокировку?',
        message: `Пользователь ${this.user?.username || ''} получит доступ к системе.`,
        confirmText: 'Разблокировать',
        cancelText: 'Отмена',
        danger: false,
      });
      if (!ok) return;
      this.banActionLoading = true;
      try {
        await unbanUser(this.user.id);
        useDeletionsStore().notify({ prefix: 'Пользователь ', bold: this.user.username, suffix: ' разблокирован' });
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка разбана:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'разблокировать пользователя', type: 'error' });
      } finally {
        this.banActionLoading = false;
      }
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 12000;
  padding: 24px;
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
  to { transform: translateY(18px); opacity: 0; }
}

.access-modal {
  background: var(--surface);
  border-radius: 45px;
  width: 980px;
  max-width: 96vw;
  /* Не голый 92vh: на >1440 корень зумлен, vh считается от НЕзумленной высоты и
     завышает кап в zoom раз (на 2539x1440 кап был 1324px при layout-высоте 900 -
     модалка вылезала на 340px сверху и снизу). --app-vh нормирован на zoom. */
  max-height: calc(var(--app-vh, 1vh) * 92);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 60px rgba(20, 26, 80, 0.28);
  animation: slideUp 0.25s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(18px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.access-modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 24px 28px;
  border-bottom: 1px solid var(--color-border);
}

.access-modal__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text);
}

.access-modal__sub {
  font-size: 13px;
  color: var(--color-text-muted);
  margin-top: 2px;
  font-weight: 500;
}

.close-btn {
  width: 34px;
  height: 34px;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--color-text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.close-btn:hover {
  color: var(--color-text);
  background: var(--surface-2);
}

.access-modal__body {
  display: grid;
  grid-template-columns: 320px 1fr;
  min-height: 0;
  flex: 1;
}

.col-left {
  padding: 24px;
  border-right: 1px solid var(--color-border);
  background: var(--color-bg-secondary);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.col-right {
  padding: 22px 26px;
  overflow-y: auto;
}

/* --- Карточка «Администратор» --- */
.admin-toggle {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border-radius: var(--radius-md);
  background: linear-gradient(120deg, var(--accent-tint), var(--accent-tint));
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
}

.admin-toggle__txt {
  flex: 1;
}

.admin-toggle__txt b {
  font-size: 14px;
  font-weight: 700;
}

.admin-toggle__txt span {
  display: block;
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 2px;
  font-weight: 500;
}

.admin-toggle__lock {
  display: block;
  font-size: 11.5px;
  color: var(--color-text-muted);
  margin-top: 6px;
  font-weight: 600;
}

/* --- Поля --- */
.field-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-text-muted);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.field-empty {
  font-size: 12px;
  color: var(--color-text-muted);
}

.lk-select {
  width: 100%;
  padding: 11px 14px;
  font-family: inherit;
  font-size: 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--surface);
  cursor: pointer;
  color: var(--color-text);
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8' viewBox='0 0 12 8'%3E%3Cpath fill='%23999' d='M1 1l5 5 5-5'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 14px center;
}

.lk-select:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.18);
}

.lk-textarea {
  width: 100%;
  padding: 10px 14px;
  font-family: inherit;
  font-size: 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  min-height: 70px;
  resize: vertical;
  color: var(--color-text);
}

.lk-textarea:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.18);
}

/* --- Чипы групп --- */
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: var(--radius-pill);
  background: var(--color-primary-tint);
  color: var(--accent-text);
  font-size: 12.5px;
  font-weight: 600;
  border: none;
}

.chip__x {
  cursor: pointer;
  opacity: 0.6;
  font-size: 14px;
}

.chip__x:hover {
  opacity: 1;
}

.chip--add {
  background: var(--surface);
  border: 1px dashed var(--color-border);
  color: var(--color-text-muted);
  cursor: pointer;
  font-family: inherit;
}

.chip--add:hover {
  border-color: var(--accent);
  color: var(--accent-text);
}

.add-groups {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 4px;
  max-height: 180px;
  overflow-y: auto;
}

.add-groups__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border: none;
  background: none;
  border-radius: 10px;
  cursor: pointer;
  font-family: inherit;
  font-size: 13px;
  color: var(--color-text);
  text-align: left;
}

.add-groups__item:hover {
  background: var(--color-bg);
}

.add-groups__item small {
  color: var(--color-text-muted);
  font-size: 11px;
}

.divider {
  height: 1px;
  background: var(--color-border);
  margin: 0;
  border: none;
}

/* --- Блок блокировки --- */
.ban-box {
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  background: var(--danger-bg);
  border-radius: var(--radius-md);
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ban-box__title {
  font-size: 13px;
  font-weight: 700;
  color: var(--danger-text);
  display: flex;
  align-items: center;
  gap: 7px;
}

.ban-box__current {
  font-size: 12px;
  color: var(--color-text);
  margin: 0;
}

.ban-box__btn {
  width: 100%;
}

/* --- Поиск по правам --- */
.perm-search {
  margin-bottom: 12px;
}

.perm-search .lk-input {
  width: 100%;
  box-sizing: border-box;
}

/* --- Легенда правого столбца --- */
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  font-size: 11.5px;
  color: var(--color-text-muted);
  font-weight: 500;
  margin-bottom: 6px;
}

.src {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.02em;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  text-transform: lowercase;
}

.src--role { background: #eef0f6; color: #6b7280; }
.src--group { background: var(--color-primary-tint); color: var(--accent-text); }
.src--override { background: #fff4e3; color: #e8870c; }

/* --- Тумблер (флаг Администратор) --- */
.tgl {
  --w: 40px;
  --h: 23px;
  --d: 17px;
  width: var(--w);
  height: var(--h);
  flex: none;
  border-radius: var(--radius-pill);
  background: var(--border);
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
  background: var(--surface);
  transition: left 0.2s ease;
}

.tgl.on { background: var(--color-primary); }
.tgl.on::after { left: calc(var(--w) - var(--d) - 3px); }

.tgl.locked {
  cursor: not-allowed;
  opacity: 0.7;
}

/* --- Кнопки (.lk-button pill) --- */
.lk-button {
  font-family: inherit;
  font-size: 13px;
  font-weight: 600;
  padding: 9px 20px;
  border-radius: var(--radius-pill);
  border: 1px solid transparent;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.lk-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.lk-button--primary { background: var(--color-primary); color: #fff; }
.lk-button--primary:not(:disabled):hover { background: var(--color-primary-hover); }
.lk-button--ghost { background: transparent; color: var(--color-text); border-color: var(--color-border); }
.lk-button--ghost:not(:disabled):hover { border-color: var(--accent); color: var(--accent-text); }
.lk-button--secondary { background: #fff; color: var(--color-text); border-color: var(--color-border); }
.lk-button--secondary:not(:disabled):hover { border-color: var(--accent); color: var(--accent-text); }
.lk-button--danger { background: #fff; color: var(--danger-text); border-color: #fecaca; }
.lk-button--danger:not(:disabled):hover { background: var(--color-danger); color: #fff; border-color: var(--danger); }

/* --- Футер --- */
.access-modal__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 28px;
  border-top: 1px solid var(--color-border);
  background: var(--surface);
}

.access-modal__foot-hint {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 500;
}

.access-modal__foot-actions {
  display: flex;
  gap: 10px;
}

@media (max-width: 720px) {
  .access-modal__body {
    grid-template-columns: 1fr;
  }

  .col-left {
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }
}
</style>
