<template>
  <TableRowRemoveMenu v-if="admin && posts.length > 1" :disabled="disabled" @remove-current="confirm([tableId])" @remove-selected="selecting = true" @remove-all="confirm(posts.map(post => post.id), true)" />
  <button v-else type="button" class="delete-btn rt-pass__act rt-pass__act--danger" title="Убрать" :disabled="disabled || !tableId" @click="confirm([tableId])">
    <AppIcon name="trashcan" class="delete-icon rt-pass__act-icon" /><span class="rt-pass__act-label">Убрать</span>
  </button>
  <BaseModal :show="selecting" title="Выбрать посты" width="520px" @close="selecting = false">
    <div class="post-removal-selection">
      <p><b>{{ label }}</b></p>
      <p>Отметьте посты, с которых нужно убрать запись.</p>
      <TargetTablesGrid v-model="selected" :tables="options" multiple />
      <p>Выбрано: {{ selected.length }} из {{ posts.length }}</p>
    </div>
    <template #actions>
      <button type="button" class="lk-button lk-button--ghost" @click="selecting = false">Отмена</button>
      <button type="button" class="lk-button lk-button--primary" :disabled="!selected.length" @click="confirm(selected)">Убрать</button>
    </template>
  </BaseModal>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import TableRowRemoveMenu from './TableRowRemoveMenu.vue';
import AppIcon from './icons/AppIcon.vue';
import BaseModal from './ui/BaseModal.vue';
import TargetTablesGrid from './CreateApplication/TargetTablesGrid.vue';
import { useUiStore } from '@/stores/ui';

const props = defineProps({
  tableId: { type: Number, default: null },
  tableTitle: { type: String, default: '' },
  label: { type: String, required: true },
  tables: { type: Array, default: () => [] },
  admin: Boolean,
  disabled: Boolean,
});
const emit = defineEmits(['remove']);
const selecting = ref(false), selected = ref([]);
const posts = computed(() => {
  const unique = new Map(props.tables.filter(post => Number.isInteger(post.id) && post.id > 0).map(post => [post.id, post]));
  if (props.tableId && !unique.has(props.tableId)) unique.set(props.tableId, { id: props.tableId, name: props.tableTitle || 'Текущий пост' });
  return [...unique.values()];
});
const options = computed(() => posts.value.map(post => ({ table: { ...post, display_name: post.name, status: 'active' } })));
watch(selecting, value => { if (value) selected.value = [props.tableId].filter(Boolean); });
async function confirm(tableIds, all = false) {
  if (props.disabled) return;
  const ids = [...new Set(tableIds)];
  if (!ids.length || ids.some(id => !posts.value.some(post => post.id === id)) || (!props.admin && (ids.length !== 1 || ids[0] !== props.tableId))) return;
  selecting.value = false;
  const names = posts.value.filter(post => ids.includes(post.id)).map(post => post.name).join(', ');
  const last = ids.length === posts.value.length;
  const accepted = await useUiStore().confirm({
    title: ids.length === 1 ? 'Убрать с поста?' : 'Убрать с постов?',
    message: `${props.label} · ${names}. ${last ? 'После снятия с последнего поста запись перестанет действовать.' : 'На остальных постах запись останется.'} Заявка и история сохранятся.`,
    confirmText: 'Убрать', cancelText: 'Отмена', danger: true,
  });
  if (accepted) emit('remove', { tableIds: ids, all });
}
</script>

<style scoped>
.post-removal-selection { padding: 24px; display: grid; gap: 16px; }
.post-removal-selection p { margin: 0; font-size: 14px; line-height: 1.5; }
.delete-btn { background: none; border: none; cursor: pointer; padding: 4px; }
.delete-btn:disabled { cursor: not-allowed; opacity: .5; }
.delete-icon { width: 16px; height: 16px; color: var(--text); opacity: .7; }
</style>
