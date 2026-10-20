<template>
  <BaseModal :show="show" title="Файлы-черновики" width="700px" radius="30px" :closable="!deleting" :close-on-overlay="!deleting" @close="close">
    <div class="draft-content">
      <p class="draft-intro">Файлы, которые ещё не отправлены с заявкой. Список общий для всех вкладок.</p>
      <div v-if="snapshot" class="draft-quota" :class="{'draft-quota--full': full}">
        <div><strong>{{ count }} <span>из {{ snapshot.limits.max_count }} файлов</span></strong><span>{{ formatBytes(bytes) }} из {{ formatBytes(snapshot.limits.max_bytes) }}</span></div>
        <div class="draft-track"><div :style="{width: `${percent}%`}" /></div>
      </div>
      <p v-if="full" class="draft-notice">Достигнут лимит файлов-черновиков. Для новой загрузки нужно удалить выбранные файлы.</p>
      <p v-if="snapshot?.usage.cleanup_count" class="draft-notice">Ожидают очистки: {{ snapshot.usage.cleanup_count }}. Место освободится после её завершения.</p>
      <p v-if="snapshot && snapshot.usage.reserved_count > snapshot.usage.cleanup_count" class="draft-notice">Есть выполняющиеся загрузки. Их объём уже учитывается в лимите.</p>
      <div v-if="error" class="draft-empty" role="status"><strong>Не удалось загрузить список</strong><p>{{ error }}</p><button class="lk-button lk-button--secondary lk-button--sm" @click="load">Повторить</button></div>
      <div v-else-if="loading" class="draft-empty" role="status">Загрузка файлов…</div>
      <div v-else-if="!snapshot?.items.length" class="draft-empty"><strong>Файлов-черновиков нет</strong><p>Здесь появятся файлы после загрузки и до подачи заявки.</p></div>
      <div v-else class="draft-list">
        <div v-for="file in snapshot.items" :key="file.id" class="draft-file" :class="{'draft-file--selected': selected?.id === file.id}">
          <span class="draft-icon">{{ extension(file.file_name) }}</span>
          <div class="draft-meta"><span class="draft-name">{{ file.file_name }}</span><span class="draft-detail">{{ formatBytes(file.file_size) }} · {{ formatDateTime(file.created_at) }}</span></div>
          <button class="lk-button lk-button--ghost lk-button--sm draft-remove" :disabled="deleting" @click="selected = file">Удалить</button>
        </div>
      </div>
      <div v-if="snapshot && snapshot.meta.total > perPage" class="draft-pages"><button class="lk-button lk-button--ghost lk-button--sm" :disabled="page <= 1 || loading || deleting" @click="changePage(-1)">Назад</button><span>{{ page }} из {{ Math.ceil(snapshot.meta.total / perPage) }}</span><button class="lk-button lk-button--ghost lk-button--sm" :disabled="page * perPage >= snapshot.meta.total || loading || deleting" @click="changePage(1)">Далее</button></div>
      <div v-if="selected" class="draft-confirm"><span>Удалить «{{ selected.file_name }}»? Он может использоваться в другой вкладке.</span><div><button class="lk-button lk-button--secondary lk-button--sm" :disabled="deleting" @click="selected = null">Отмена</button><button class="lk-button lk-button--danger lk-button--sm" :disabled="deleting" @click="removeSelected">{{ deleting ? 'Удаление…' : 'Удалить файл' }}</button></div></div>
      <p v-else class="draft-footnote">Файл может использоваться в другой незавершённой заявке. Удаление затронет все вкладки.</p>
    </div>
    <template #actions><RefreshButton :loading="loading || deleting" @refresh="load" /><button class="lk-button lk-button--secondary" :disabled="deleting" @click="close">Закрыть</button></template>
  </BaseModal>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import { fetchApplicationDraftFiles, deleteApplicationDraftFile } from '@/api/applicationFiles';
import { formatBytes } from '@/utils/download';
import { formatDateTime } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

const props = defineProps({ show: { type: Boolean, required: true } });
const emit = defineEmits(['close', 'deleted', 'usage']);
const notifications = useDeletionsStore();
const snapshot = ref(null), loading = ref(false), error = ref(''), selected = ref(null), deleting = ref(false), page = ref(1);
const perPage = 20;
let sequence = 0;
const count = computed(() => snapshot.value ? snapshot.value.usage.saved_count + snapshot.value.usage.reserved_count : 0);
const bytes = computed(() => snapshot.value ? snapshot.value.usage.saved_bytes + snapshot.value.usage.reserved_bytes : 0);
const full = computed(() => snapshot.value && (count.value >= snapshot.value.limits.max_count || bytes.value >= snapshot.value.limits.max_bytes));
const percent = computed(() => snapshot.value ? Math.min(100, Math.max(count.value / snapshot.value.limits.max_count, bytes.value / snapshot.value.limits.max_bytes) * 100) : 0);
function extension(name) { const ext = (name.split('.').pop() || '').toUpperCase(); return ext.length <= 4 ? ext : 'ФАЙЛ'; }
function close() { if (!deleting.value) emit('close'); }
async function load() {
  if (deleting.value) return;
  const current = ++sequence;
  loading.value = true; error.value = ''; selected.value = null;
  try {
    const data = await fetchApplicationDraftFiles(page.value, perPage);
    if (current !== sequence || !props.show) return;
    snapshot.value = data; emit('usage', data);
    if (page.value > 1 && !data.items.length) { page.value--; await load(); }
  } catch (failure) { if (current === sequence) error.value = failure.message; }
  finally { if (current === sequence) loading.value = false; }
}
async function changePage(delta) { page.value += delta; await load(); }
async function removeSelected() {
  if (!selected.value || deleting.value) return;
  deleting.value = true;
  try {
    const id = selected.value.id;
    await deleteApplicationDraftFile(id); emit('deleted', id);
    notifications.notify({ bold: 'Файл убран из черновиков', type: 'success' });
    selected.value = null;
  } catch (failure) { notifications.notify({ bold: failure.message, type: 'error' }); }
  finally { deleting.value = false; await load(); }
}
watch(() => props.show, (show) => { ++sequence; if (show) { page.value = 1; load(); } }, { immediate: true });
onBeforeUnmount(() => { ++sequence; });
</script>

<style scoped>
.draft-content { padding: 20px; }
.draft-intro { margin: 0 0 18px; line-height: 1.55; color: var(--text-muted); font-size: 13px; }
.draft-quota { padding: 14px 16px; border: 1px solid var(--border); border-radius: var(--radius-md); margin-bottom: 16px; background: var(--surface-2); }
.draft-quota > div:first-child { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 13px; }
.draft-quota strong { font-weight: 600; }
.draft-quota strong span { font-weight: 400; color: var(--text-muted); }
.draft-track { height: 4px; background: var(--border); border-radius: 10px; overflow: hidden; margin-top: 13px; }
.draft-track > div { height: 100%; background: var(--accent); border-radius: 10px; }
.draft-quota--full { border-color: var(--warning); }
.draft-notice { font-size: 13px; line-height: 1.5; border-left: 3px solid var(--warning); padding: 8px 12px; margin: 0 0 16px; background: var(--surface-2); }
.draft-list { border-top: 1px solid var(--border); }
.draft-file { display: flex; align-items: center; gap: 12px; padding: 16px 0; border-bottom: 1px solid var(--border); }
.draft-file--selected { background: var(--surface-2); }
.draft-icon { font-size: 10px; font-weight: 600; color: var(--text-muted); border: 1px solid var(--border); border-radius: 9px; width: 40px; height: 44px; display: flex; justify-content: center; align-items: center; flex: none; }
.draft-meta { display: flex; flex-direction: column; min-width: 0; gap: 6px; flex: 1; }
.draft-name { overflow-wrap: anywhere; line-height: 1.4; font-size: 13px; font-weight: 500; }
.draft-detail { color: var(--text-muted); font-size: 11px; line-height: 1.5; }
.draft-remove { flex: none; }
.draft-footnote { color: var(--text-muted); font-size: 11px; line-height: 1.5; margin: 16px 0 0; }
.draft-empty { text-align: center; padding: 28px 12px; }
.draft-empty p { color: var(--text-muted); line-height: 1.5; font-size: 13px; margin: 12px 0; }
.draft-confirm { margin-top: 16px; padding: 14px; border: 1px solid var(--border); border-radius: var(--radius-md); font-size: 12px; line-height: 1.5; }
.draft-confirm > div { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.draft-pages { display: flex; justify-content: space-between; align-items: center; padding-top: 12px; font-size: 11px; }
@media (max-width: 768px) {
  .draft-content { padding: 16px; }
  .draft-quota > div:first-child { flex-wrap: wrap; gap: 8px; }
  .draft-file { gap: 10px; padding: 14px 0; }
  .draft-remove { padding: 7px 10px; }
  .draft-detail { font-size: 10px; }
  .draft-icon { width: 34px; height: 40px; }
  .draft-intro { font-size: 12px; }
}
</style>
