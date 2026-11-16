<script setup>
import { onUnmounted, ref } from 'vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import { accountOwnerRequest } from '@/api/accountOwner'
import { formatMoscowDateTime } from '@/utils/serverTime'

const props = defineProps({
  applicationId: { type: Number, required: true },
  applicationNumber: { type: String, required: true },
  event: { type: Object, required: true },
})
defineEmits(['close'])
const reason = ref('')
const busy = ref(false)
const error = ref('')
const result = ref(null)
const approvalDate = new Date(props.event.created_at)
const approvalTime = Number.isNaN(approvalDate.getTime()) ? '' : formatMoscowDateTime(approvalDate)
const request = new AbortController()
onUnmounted(() => request.abort())

async function verify() {
  if (busy.value || !reason.value.trim()) return
  busy.value = true
  error.value = ''
  try {
    result.value = await accountOwnerRequest(
      '/applications/' + props.applicationId + '/history/' + props.event.id + '/account-owner',
      { reason: reason.value.trim() }, request.signal
    )
  } catch (err) {
    if (!request.signal.aborted) error.value = err.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <BaseModal
    :show="true"
    title="Проверка владельца учётной записи"
    width="540px"
    :z-index="20001"
    @close="$emit('close')"
  >
    <section class="owner-lookup">
      <div class="owner-context">
        <span>Заявка {{ applicationNumber }}</span>
        <strong>{{ approvalTime ? 'Согласование · ' + approvalTime : 'Согласование' }}</strong>
      </div>
      <form
        v-if="!result"
        id="owner-lookup-form"
        @submit.prevent="verify"
      >
        <label for="owner-lookup-reason">Основание запроса</label>
        <textarea
          id="owner-lookup-reason"
          v-model="reason"
          required
          maxlength="300"
          class="lk-textarea"
          rows="2"
          placeholder="Укажи номер обращения или причину проверки"
          :disabled="busy"
        />
        <p class="owner-note">
          Запрос фиксируется в журнале аудита.
        </p>
        <p
          v-if="error"
          role="alert"
        >
          {{ error }}
        </p>
      </form>
      <template v-else>
        <strong
          class="owner-status"
          :class="{ missing: !result.evidence }"
        >
          {{ result.evidence ? 'Владелец подтверждён' : 'Владелец не подтверждён' }}
        </strong>
        <dl class="owner-data">
          <div><dt>Учётная запись исполнителя</dt><dd>№ {{ result.user_id }}</dd></div>
          <div v-if="result.on_behalf_of">
            <dt>Выполнено от имени учётной записи</dt><dd>№ {{ result.on_behalf_of }}</dd>
          </div>
          <template v-if="result.evidence">
            <div><dt>Владелец на дату согласования</dt><dd>{{ result.evidence.full_name }}</dd></div>
            <div><dt>Подтверждение</dt><dd>{{ result.evidence.source_ref }}</dd></div>
          </template>
        </dl>
        <p
          v-if="!result.evidence"
          class="owner-note"
        >
          Подтверждённой записи о владельце учётки на указанную дату нет.
        </p>
      </template>
    </section>
    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--secondary"
        @click="$emit('close')"
      >
        {{ result ? 'Закрыть' : 'Отмена' }}
      </button>
      <button
        v-if="!result"
        form="owner-lookup-form"
        type="submit"
        class="lk-button lk-button--primary"
        :disabled="busy || !reason.trim()"
      >
        {{ busy ? 'Проверяем…' : 'Проверить' }}
      </button>
    </template>
  </BaseModal>
</template>

<style scoped>
.owner-lookup { padding: 18px 20px; font-size: 14px; overflow-wrap: anywhere; }
.owner-context { display: grid; gap: 6px; padding: 12px 14px; margin-bottom: 17px; border: 1px solid var(--border); border-radius: 15px; }
.owner-context span, .owner-data dt, .owner-note { color: var(--text-muted); }
form label { display: block; font-weight: 600; margin-bottom: 7px; }
textarea { box-sizing: border-box; width: 100%; resize: vertical; }
.owner-note { margin: 12px 0 0; line-height: 1.55; }
.owner-status { display: inline-block; padding: 6px 11px; border-radius: 99px; background: var(--success-bg); color: var(--success-text); }
.owner-status.missing { background: var(--warning-bg); color: var(--warning-text); }
.owner-data { margin: 12px 0 0; }
.owner-data div { display: grid; gap: 4px; padding: 9px 0; border-bottom: 1px solid var(--border); }
.owner-data dd { margin: 0; font-weight: 600; }
[role="alert"] { color: var(--danger-text); }
</style>
