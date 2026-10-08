<template>
  <BaseModal :show="show" :title="kind === 'employee' ? 'Незакрытые проходы' : 'Незакрытые проезды'" width="900px" radius="30px"
    :closable="!saving" :close-on-overlay="!saving" content-testid="open-passages-modal" @close="close">
    <div class="open-passages">
      <p class="open-passages__note">Незакрытая отметка не доказывает присутствие на территории. Исправление учёта отличается от наблюдавшегося выхода.</p>
      <div class="open-passages__filters">
        <BaseDropdown v-model="mode" :options="modeOptions" label-key="label" value-key="id" :disabled="saving" data-testid="open-passages-mode" />
        <BaseDropdown v-model="organizationID" :options="organizationOptions" label-key="label" value-key="id" :disabled="saving || !organizations.length" searchable placeholder="Все организации" data-testid="open-passages-organization" />
        <input v-model="search" class="lk-input" type="search" placeholder="Поиск" aria-label="Поиск незакрытых отметок" :disabled="saving" @input="searchChanged">
        <button type="button" class="lk-button lk-button--ghost lk-button--sm" :disabled="loading || saving" @click="refresh">Обновить</button>
      </div>
      <p class="open-passages__note" data-testid="open-passages-counts">Незакрытых: {{ counts.all_open }} · более 48 часов: {{ counts.attention }} · время неизвестно: {{ counts.unknown_time }}</p>
      <p v-if="loading" role="status">Загрузка…</p>
      <p v-if="error" class="open-passages__error" role="alert">{{ error }}</p>
      <p v-if="corrected && mode !== 'corrections'" role="status">Учёт исправлен. <button class="lk-button lk-button--ghost lk-button--sm" @click="mode = 'corrections'">Недавние исправления</button></p>
      <div v-if="!loading && items.length" class="open-passages__list">
        <article v-for="row in items" :key="row.entity_id" class="open-passages__row" :data-testid="`open-passage-${row.entity_id}`">
          <div class="open-passages__identity">
            <strong>{{ row.display_name }}</strong>
            <span>{{ row.organization || 'Организация не указана' }} · {{ row.application_number || 'Ручное основание' }} · основание № {{ row.attachment_id }}</span>
          </div>
          <div class="open-passages__facts">
            <span>Вход: {{ row.passage_state.entry_time_known ? dateText(row.passage_state.entry_at) : 'время неизвестно' }} · пост {{ row.passage_state.entry_table_id || 'неизвестен' }}</span>
            <span>Длительность: {{ duration(row) }} · срок: {{ periodText(row.effective_period) }}</span>
            <span v-if="mode === 'corrections'">Исправление зарегистрировано: {{ dateText(row.passage_state.exit_recorded_at) }}; фактический выход мог быть неизвестен.</span>
          </div>
          <button v-if="canCorrect && row.passage_state.can_correct && mode !== 'corrections'" class="lk-button lk-button--ghost lk-button--sm" :disabled="saving" @click="select(row, false)">Исправить учёт</button>
          <button v-if="canCorrect && row.passage_state.can_revert_correction && revertWithinWindow(row)" class="lk-button lk-button--ghost lk-button--sm" :disabled="saving" @click="select(row, true)">Отменить исправление</button>
        </article>
      </div>
      <p v-else-if="!loading && !error">{{ mode === 'corrections' ? 'Недавних исправлений нет.' : 'Незакрытых отметок по выбранным условиям нет.' }}</p>
      <section v-if="selected" class="open-passages__correction" data-testid="passage-correction-form">
        <strong>{{ reverting ? 'Отменить исправление учёта' : 'Исправить учёт' }}: {{ selected.display_name }}</strong>
        <p class="open-passages__note">{{ reverting ? 'Восстановится прежняя открытая отметка. Отмена разрешена только в течение 15 минут, если после исправления не было другого события.' : 'Это исправление незакрытой отметки, а не подтверждение наблюдавшегося выхода.' }}</p>
        <FormField label="Причина" required><textarea v-model="reason" class="lk-textarea" rows="2" maxlength="1000" :disabled="saving || conflict" data-testid="passage-correction-reason" /></FormField>
        <template v-if="!reverting">
          <label class="open-passages__choice"><input v-model="actualKnown" type="checkbox" :disabled="saving || conflict"> Фактическое время выхода известно</label>
          <FormField v-if="actualKnown" label="Фактическое время выхода (Москва)" required><input v-model="actualExit" class="lk-input" type="datetime-local" :disabled="saving || conflict" data-testid="passage-correction-actual" /></FormField>
          <p v-else class="open-passages__note">Фактическое время выхода будет записано как неизвестное; время регистрации исправления сохранит сервер.</p>
        </template>
        <div class="open-passages__actions">
          <button class="lk-button lk-button--ghost lk-button--sm" :disabled="saving" @click="selected = null">Отмена</button>
          <button class="lk-button lk-button--primary lk-button--sm" :disabled="!canSave" data-testid="passage-correction-save" @click="save">{{ saving ? 'Сохранение…' : reverting ? 'Отменить исправление' : 'Сохранить исправление' }}</button>
        </div>
      </section>
    </div>
    <template #actions>
      <span class="open-passages__note">{{ total }} записей · страница {{ page }}</span>
      <button class="lk-button lk-button--ghost lk-button--sm" :disabled="page <= 1 || loading || saving" @click="page--">Назад</button>
      <button class="lk-button lk-button--ghost lk-button--sm" :disabled="page * perPage >= total || loading || saving" @click="page++">Далее</button>
      <button class="lk-button lk-button--ghost" :disabled="saving" @click="close">Закрыть</button>
    </template>
  </BaseModal>
</template>
<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import FormField from '@/components/ui/FormField.vue';
import { listOpenPassages, closeOpenPassage, revertPassageCorrection } from '@/api/openPassages';
import { usePassageClock } from '@/composables/usePassageClock';
import { usePermissionsStore } from '@/stores/permissions';
import { useDeletionsStore } from '@/stores/deletions';
import { formatMoscowDateTime } from '@/utils/serverTime';
import { formatPeriod } from '@/utils/entryWindow';

const props = defineProps({ show: Boolean, kind: { type: String, required: true }, source: { type: String, default: 'table', validator: value => ['table', 'admin_summary'].includes(value) }, tableID: { type: Number, default: null }, organizations: { type: Array, default: () => [] } });
const emit = defineEmits(['close', 'changed', 'open-entity']);
const permissions = usePermissionsStore();
const mode = ref('attention'), organizationID = ref(null), search = ref(''), page = ref(1), perPage = 25;
const items = ref([]), counts = ref({ all_open: 0, attention: 0, unknown_time: 0 }), total = ref(0), serverNow = ref(null);
const loading = ref(false), saving = ref(false), error = ref(''), selected = ref(null), reverting = ref(false), reason = ref('');
const actualKnown = ref(false), actualExit = ref(''), conflict = ref(false), corrected = ref(false);
let version = 0, searchTimer = null, pollTimer = null;
const canCorrect = computed(() => permissions.hasPermission('detail.passage.correct'));
const modeOptions = computed(() => [{ id: 'attention', label: 'Более 48 часов' }, { id: 'all', label: 'Все незакрытые' },
  ...(canCorrect.value ? [{ id: 'corrections', label: 'Недавние исправления' }] : [])]);
const organizationOptions = computed(() => [{ id: null, label: 'Все организации' }, ...props.organizations.map(o => ({ id: o.id, label: o.name }))]);
const active = computed(() => props.show);
const deadlines = computed(() => items.value.flatMap(row => {
  const state = row.passage_state;
  return [state.grace_until, state.open && state.entry_time_known && state.entry_at ? Date.parse(state.entry_at) + 48 * 3600000 + 1 : null,
    state.can_revert_correction && state.exit_recorded_at ? Date.parse(state.exit_recorded_at) + 15 * 60000 + 1 : null].filter(Boolean);
}));
const clock = usePassageClock(serverNow, { active, deadlines, onBoundary: () => { if (!loading.value && !saving.value) load(); } });
function dateText(stamp) {
  const instant = stamp instanceof Date ? stamp.getTime() : typeof stamp === 'string' ? Date.parse(stamp) : NaN;
  return Number.isFinite(instant) ? formatMoscowDateTime(new Date(instant)) : 'неизвестно';
}
const periodText = period => period.bounded ? formatPeriod(period) : 'Без ограничения';
function duration(row) {
  if (!row.passage_state.entry_time_known) return 'неизвестна';
  const elapsed = clock.elapsedSince(row.passage_state.entry_at);
  if (elapsed === null) return 'неизвестна';
  const minutes = Math.floor(elapsed / 60000);
  return `${Math.floor(minutes / 60)} ч ${minutes % 60} мин`;
}
function revertWithinWindow(row) {
  const recorded = Date.parse(row.passage_state.exit_recorded_at || '');
  return clock.now.value !== null && Number.isFinite(recorded) && clock.now.value <= recorded + 15 * 60000;
}
const actualISO = computed(() => {
  if (!actualKnown.value) return null;
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(actualExit.value)) return '';
  const instant = Date.parse(`${actualExit.value}:00+03:00`);
  return Number.isFinite(instant) ? new Date(instant).toISOString() : '';
});
const canSave = computed(() => {
  if (!selected.value || !canCorrect.value || saving.value || loading.value || conflict.value || !reason.value.trim() || reason.value.trim().length > 1000) return false;
  if (reverting.value) return selected.value.passage_state.can_revert_correction && revertWithinWindow(selected.value);
  if (!selected.value.passage_state.can_correct) return false;
  if (!actualKnown.value) return true;
  const time = Date.parse(actualISO.value || '');
  const entry = Date.parse(selected.value.passage_state.entry_at || '');
  return Number.isFinite(time) && clock.now.value !== null && time <= clock.now.value && (!Number.isFinite(entry) || time >= entry);
});
async function load() {
  if (!props.show || saving.value) return;
  const seq = ++version;
  loading.value = true;
  error.value = '';
  items.value = [];
  try {
    const data = await listOpenPassages(props.kind, props.tableID, { source: props.source, attentionOnly: mode.value === 'attention',
      view: mode.value === 'corrections' ? 'corrections' : 'open', search: search.value, organizationID: organizationID.value, page: page.value, perPage });
    if (seq !== version) return;
    items.value = data.items; counts.value = data.counts; total.value = data.total; serverNow.value = data.server_now;
    if (selected.value) {
      const fresh = data.items.find(row => row.entity_id === selected.value.entity_id);
      if (fresh && fresh.passage_state.last_event_id !== selected.value.passage_state.last_event_id) {
        selected.value = null; reason.value = ''; actualKnown.value = false; actualExit.value = '';
        error.value = 'Последняя отметка изменилась. Выбери запись заново и проверь причину и фактическое время; прежнее исправление не будет применено к новому событию.';
      } else selected.value = fresh || null;
    }
  } catch (failure) { if (seq === version) { error.value = failure.message || 'Не удалось загрузить отметки'; selected.value = null; } }
  finally { if (seq === version) loading.value = false; }
}
function refresh() { selected.value = null; conflict.value = false; clearTimeout(searchTimer); load(); }
function close() { if (!saving.value) { version++; clearTimeout(searchTimer); clearInterval(pollTimer); emit('close'); } }
function select(row, revert) { selected.value = row; reverting.value = revert; reason.value = ''; actualKnown.value = false; actualExit.value = ''; conflict.value = false; error.value = ''; }
function searchChanged() { clearTimeout(searchTimer); version++; selected.value = null; searchTimer = setTimeout(() => { if (page.value !== 1) page.value = 1; else load(); }, 300); }
async function save() {
  if (!canSave.value) return;
  const seq = ++version, row = selected.value;
  saving.value = true;
  error.value = '';
  try {
    const command = reverting.value ? revertPassageCorrection : closeOpenPassage;
    const result = await command(props.kind, row.entity_id, props.tableID, { source: props.source, expected_last_event_id: row.passage_state.last_event_id,
      reason: reason.value, ...(!reverting.value ? { actual_exit_at: actualISO.value } : {}) });
    if (seq !== version) return;
    corrected.value = !reverting.value;
    selected.value = null;
    emit('changed', result);
    useDeletionsStore().notify({ prefix: reverting.value ? 'Исправление учёта отменено.' : 'Учёт прохода исправлен.', type: 'success' });
  } catch (failure) {
    if (seq === version) { conflict.value = true; error.value = `${failure.message || 'Не удалось сохранить'}. Обновите данные и проверьте отметку заново; автоматического повтора нет.`; }
  } finally { if (seq === version) saving.value = false; }
  if (seq === version && !conflict.value) await load();
}
watch(() => [props.show, props.kind, props.tableID, props.source], () => {
  version++; clearTimeout(searchTimer); clearInterval(pollTimer); saving.value = loading.value = false; selected.value = null;
  items.value = []; counts.value = { all_open: 0, attention: 0, unknown_time: 0 }; total.value = 0; serverNow.value = null;
  mode.value = 'attention'; organizationID.value = null; search.value = ''; page.value = 1; conflict.value = corrected.value = false;
  if (props.show) {
    load();
    // Invisible rows may cross 48h while the current attention page is empty.
    // Keep the scoped server list/count authoritative, with at most 30s delay.
    pollTimer = setInterval(() => { if (!loading.value && !saving.value) load(); }, 30000);
  }
}, { immediate: true });
watch([mode, organizationID], () => { selected.value = null; if (page.value !== 1) page.value = 1; else load(); });
watch(page, load);
watch(canCorrect, allowed => { if (!allowed) { selected.value = null; if (mode.value === 'corrections') mode.value = 'attention'; } });
onBeforeUnmount(() => { version++; clearTimeout(searchTimer); clearInterval(pollTimer); });
</script>
<style scoped>
.open-passages { display: flex; flex-direction: column; gap: 12px; padding: 16px 20px; min-width: 0; }
.open-passages__note { margin: 0; font-size: 12px; color: var(--text-muted); line-height: 1.5; }
.open-passages__filters { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.open-passages__filters :deep(.base-dropdown) { flex: 1; min-width: 160px; }
.open-passages__filters .lk-input { flex: 1; min-width: 140px; }
.open-passages__list { display: flex; flex-direction: column; gap: 10px; }
.open-passages__row, .open-passages__correction { border: 1px solid var(--border); border-radius: var(--radius-md); padding: 12px; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
.open-passages__identity, .open-passages__facts { display: flex; flex-direction: column; gap: 4px; font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
.open-passages__identity > span { color: var(--text-muted); }
.open-passages__entity { padding: 0; border: 0; background: none; color: var(--accent); font: inherit; font-weight: 600; text-align: left; cursor: pointer; }
.open-passages__correction { align-items: stretch; }
.open-passages__choice { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.open-passages__actions { display: flex; justify-content: flex-end; gap: 8px; }
.open-passages__error { margin: 0; color: var(--danger-text); font-size: 13px; }
</style>
