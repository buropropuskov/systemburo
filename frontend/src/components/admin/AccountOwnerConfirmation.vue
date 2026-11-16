<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import { accountOwnerRequest, getAccountOwnerAccess } from '@/api/accountOwner'

const props = defineProps({ user: { type: Object, required: true } })
const allowed = ref(false)
const open = ref(false)
const busy = ref(false)
const error = ref('')
const saved = ref(false)
const conflict = ref(false)
const revision = ref(null)
const fullName = ref('')
const source = ref('')
const effective = ref('')
const confirmed = ref(false)
let request
const path = () => '/account-owner/users/' + props.user.id

onMounted(async () => { allowed.value = !!(await getAccountOwnerAccess()).confirm })
onUnmounted(() => request?.abort())

function close() {
  open.value = false
  busy.value = false
  request?.abort()
}

async function start() {
  request?.abort()
  const current = request = new AbortController()
  fullName.value = props.user.pd_hidden ? '' :
    [props.user.last_name, props.user.first_name, props.user.middle_name].filter(Boolean).join(' ')
  source.value = ''
  effective.value = ''
  confirmed.value = false
  revision.value = null
  error.value = ''
  saved.value = false
  conflict.value = false
  open.value = true
  busy.value = true
  try {
    const status = await accountOwnerRequest(path(), null, current.signal)
    if (current.signal.aborted) return
    revision.value = status.revision
    if (status.effective_from) {
      const date = new Date(status.effective_from)
      effective.value = new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
    }
  } catch (err) {
    if (!current.signal.aborted) error.value = err.message
  } finally {
    if (!current.signal.aborted) busy.value = false
  }
}

async function save() {
  if (busy.value || revision.value === null || !confirmed.value || saved.value || conflict.value ||
      !fullName.value.trim() || !source.value.trim() || !effective.value) return
  const current = request = new AbortController()
  error.value = ''
  busy.value = true
  try {
    await accountOwnerRequest(path(), {
      full_name: fullName.value.trim(),
      source_ref: source.value.trim(),
      effective_from: new Date(effective.value).toISOString(),
      expected_revision: revision.value,
      confirmed: true,
    }, current.signal)
    if (!current.signal.aborted) saved.value = true
  } catch (err) {
    if (!current.signal.aborted) {
      error.value = err.message
      conflict.value = err.message.startsWith('Подтверждение изменилось')
    }
  } finally {
    if (!current.signal.aborted) busy.value = false
  }
}
</script>

<template>
  <button
    v-if="allowed"
    type="button"
    class="lk-button lk-button--secondary owner-open"
    @click="start"
  >
    Подтверждение владельца
  </button>
  <BaseModal
    v-if="open"
    :show="open"
    title="Подтверждение владельца при выдаче учётки"
    width="540px"
    :z-index="20001"
    @close="close"
  >
    <form
      id="owner-confirm-form"
      class="owner-form"
      @submit.prevent="save"
    >
      <p
        v-if="saved"
        role="status"
      >
        Подтверждение сохранено.
      </p>
      <template v-else-if="revision !== null">
        <p
          v-if="revision"
          class="owner-help"
        >
          Будет сохранена новая версия подтверждения.
        </p>
        <label>Владелец по подтверждённому источнику
          <input
            v-model="fullName"
            class="lk-input"
            required
            maxlength="300"
            :disabled="busy"
          >
        </label>
        <label>Источник подтверждения
          <input
            v-model="source"
            class="lk-input"
            required
            maxlength="300"
            :disabled="busy"
            placeholder="Например, номер заявки на выдачу"
          >
        </label>
        <label>Дата и время выдачи по источнику (местное время)
          <input
            v-model="effective"
            class="lk-input"
            type="datetime-local"
            required
            :disabled="busy"
          >
        </label>
        <label class="owner-check">
          <input
            v-model="confirmed"
            type="checkbox"
            required
            :disabled="busy"
          >
          Подтверждаю, что учётка выдана указанному человеку
        </label>
        <p class="owner-help">
          Система сохранит дату подтверждения и сотрудника, который его выполнил.
        </p>
      </template>
      <p
        v-else-if="busy"
        role="status"
      >
        Загрузка…
      </p>
      <p
        v-if="error"
        role="alert"
      >
        {{ error }}
      </p>
    </form>
    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--secondary"
        @click="close"
      >
        Закрыть
      </button>
      <button
        v-if="!saved && revision !== null"
        class="lk-button lk-button--primary"
        type="submit"
        form="owner-confirm-form"
        :disabled="busy || conflict || !confirmed"
      >
        Сохранить подтверждение
      </button>
    </template>
  </BaseModal>
</template>

<style scoped>
.owner-open { align-self: flex-start; max-width: calc(100% - 40px); margin: 12px 20px 0; }
.owner-form { display: grid; gap: 12px; padding: 16px 20px; font-size: 14px; }
.owner-form label { display: grid; gap: 6px; }
.owner-form .lk-input { width: 100%; min-width: 0; box-sizing: border-box; }
.owner-form .owner-check { display: flex; align-items: flex-start; gap: 8px; }
.owner-form p { margin: 0; }
.owner-help { color: var(--text-muted); }
[role="alert"] { color: var(--danger-text); }
</style>
