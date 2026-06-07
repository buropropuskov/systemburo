<template>
  <div class="ip-panel">
    <div class="ip-panel__body">
      <LoaderSpinner
        v-if="loading"
        label="Загрузка прав..."
      />
      <PermissionTree
        v-else
        :tree="tree"
        :selected="permissions"
        @change="onChange"
      />
    </div>

    <footer class="ip-panel__footer">
      <button
        class="lk-button lk-button--ghost"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="lk-button lk-button--primary"
        :disabled="saving || loading"
        @click="save"
      >
        {{ saving ? 'Сохранение...' : 'Сохранить' }}
      </button>
    </footer>
  </div>
</template>

<script>
import PermissionTree from '../PermissionTree.vue';
import LoaderSpinner from '../ui/LoaderSpinner.vue';
import { getPermissionTree, getUserPermissions, updateUserPermissions } from '@/api/permissions';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Контентная панель «Индивидуальные права» (дерево ключей прав конкретного юзера).
 * Логика вынесена из UserControl, чтобы жить во вкладке UserAccessModal. Не модалка.
 */
export default {
  name: 'IndividualPermissionsPanel',
  components: { PermissionTree, LoaderSpinner },
  props: {
    user: { type: Object, default: null },
  },
  emits: ['close', 'updated'],
  data() {
    return {
      tree: [],
      permissions: {},
      loading: false,
      saving: false,
    };
  },
  mounted() {
    this.load();
  },
  methods: {
    async load() {
      if (!this.user) return;
      this.loading = true;
      try {
        const [treeData, permsData] = await Promise.all([
          getPermissionTree(),
          getUserPermissions(this.user.id),
        ]);
        this.tree = Array.isArray(treeData) ? treeData : [];
        const map = {};
        if (Array.isArray(permsData)) {
          permsData.forEach((p) => { map[p.key] = p.value; });
        }
        this.permissions = map;
      } catch (e) {
        console.error('Ошибка загрузки прав:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'индивидуальные права', type: 'error' });
      } finally {
        this.loading = false;
      }
    },
    onChange(key, value) {
      this.permissions[key] = value;
    },
    async save() {
      if (!this.user) return;
      this.saving = true;
      const permissions = Object.entries(this.permissions).map(([key, value]) => ({ key, value }));
      try {
        const result = await updateUserPermissions(this.user.id, { permissions });
        if (result && result.message) {
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: result.message, type: 'error' });
          return;
        }
        useDeletionsStore().notify({ prefix: 'Сохранены ', bold: 'индивидуальные права' });
        this.$emit('updated');
        this.$emit('close');
      } catch (e) {
        console.error('Ошибка сохранения прав:', e);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить ', bold: 'индивидуальные права', type: 'error' });
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.ip-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.ip-panel__body {
  padding: 16px 20px;
  overflow-y: auto;
  flex: 1;
}

.ip-panel__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--color-border);
}
</style>
