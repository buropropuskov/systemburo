<script setup>
import { computed, ref, watch } from 'vue'
import { apiRequest } from '@/api/client'
import { getCompanies, getOrganizations } from '@/api/organizations'
import { useDeletionsStore } from '@/stores/deletions'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'

const props = defineProps({ templateId: { type: Number, required: true } })
const directories = [{ id: 'organization', name: 'Организация / отдел' }, { id: 'company', name: 'Компания' }]
const directory = ref('company')
const directoryId = ref(null)
const options = ref([])
const enabled = ref(false)
const ready = ref(false)
const loading = ref(false)
const saving = ref(false)
const notify = useDeletionsStore().notify
const endpoint = computed(() => `/attachments/${props.templateId}/document-rule?${new URLSearchParams({ directory: directory.value, directory_id: directoryId.value })}`)

watch(directory, async (value, _, onCleanup) => {
  let current = true
  onCleanup(() => { current = false })
  directoryId.value = null
  options.value = []
  try {
    const rows = await (value === 'company' ? getCompanies() : getOrganizations())
    if (current) options.value = rows.filter(row => row.is_active !== false)
  } catch {
    if (current) notify({ bold: 'Не удалось загрузить справочник', type: 'error' })
  }
}, { immediate: true })

watch([directory, directoryId], async ([, id], _, onCleanup) => {
  let current = true
  onCleanup(() => { current = false })
  ready.value = false
  enabled.value = false
  loading.value = id != null
  if (id == null) return
  try {
    const response = await apiRequest(endpoint.value)
    const result = await response.json()
    if (current) {
      enabled.value = result.enabled === true
      ready.value = true
    }
  } catch {
    if (current) notify({ bold: 'Не удалось загрузить правило', type: 'error' })
  } finally {
    if (current) loading.value = false
  }
})

async function save() {
  if (!ready.value || saving.value) return
  saving.value = true
  try {
    await apiRequest(endpoint.value, { method: 'PUT', body: JSON.stringify({ enabled: enabled.value }) })
    notify({ bold: 'Правило сохранено', type: 'success' })
  } catch {
    notify({ bold: 'Не удалось сохранить правило', type: 'error' })
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="document-rule" :inert="saving">
    <div class="rule-head">
      <h4>Проверка документа на посту</h4>
      <ToggleSwitch v-model="enabled" :disabled="!ready" aria-label="Проверка документа на посту" />
    </div>
    <p>Заявитель сможет указать, что сотрудник предъявит оригинал документа на посту.</p>
    <div class="rule-fields">
      <div><span>Справочник</span><BaseDropdown v-model="directory" :options="directories" aria-label="Справочник" /></div>
      <div><span>Организация или компания</span><BaseDropdown v-model="directoryId" :options="options" aria-label="Организация или компания" placeholder="Выбрать запись" searchable /></div>
    </div>
    <button type="button" class="lk-button" :disabled="loading || !ready || saving" @click="save">Сохранить правило</button>
  </section>
</template>

<style scoped>
.document-rule {
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}
.rule-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.rule-head h4 { margin: 0; font-size: 15px; }
.document-rule p { margin: 10px 0 14px; font-size: 13px; line-height: 1.45; color: var(--text-muted); }
.rule-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-bottom: 12px; }
.rule-fields span { display: block; margin-bottom: 4px; font-size: 12px; }
@media (max-width: 550px) { .rule-fields { grid-template-columns: 1fr; } }
</style>
