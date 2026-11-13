<script setup>
import { computed, ref, watch } from 'vue'
import { apiRequest } from '@/api/client'
import { useDeletionsStore } from '@/stores/deletions'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'

const props = defineProps({ userId: { type: Number, required: true } })
const templateId = ref(null)
const options = ref([])
const enabled = ref(false)
const ready = ref(false)
const loading = ref(false)
const saving = ref(false)
const notify = useDeletionsStore().notify
const endpoint = computed(() => `/users/${props.userId}/document-rule/${templateId.value}`)

watch(() => props.userId, async (_, __, onCleanup) => {
  let current = true
  onCleanup(() => { current = false })
  templateId.value = null
  try {
    const response = await apiRequest('/attachments')
    const rows = await response.json()
    if (current) options.value = rows.filter(row => row.attachment_type === 'people' && row.is_active !== false)
  } catch {
    if (current) notify({ bold: 'Не удалось загрузить виды заявок', type: 'error' })
  }
}, { immediate: true })

watch(templateId, async (id, _, onCleanup) => {
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
    const response = await apiRequest(endpoint.value, { method: 'PUT', body: JSON.stringify({ enabled: enabled.value }) })
    if (!response.ok) throw new Error('Не удалось сохранить правило')
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
    <p>Разрешить этому аккаунту выбрать «Документ на посту» для указанного вида заявки.</p>
    <div class="rule-fields">
      <BaseDropdown v-model="templateId" :options="options" label-key="display_name" aria-label="Вид заявки" placeholder="Выбрать вид заявки" searchable />
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
.rule-fields { margin-bottom: 12px; }
</style>
