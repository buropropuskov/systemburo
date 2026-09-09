<template>
  <BaseModal
    :show="show"
    title="Выгрузка справки"
    width="520px"
    @close="$emit('close')"
  >
    <div class="pdse__form">
      <p class="pdse__hint">
        Передача сведений третьему лицу - это раскрытие персональных данных. Выдача
        попадёт в журнал: при проверке им подтверждают, что раскрытие было законным.
      </p>

      <label class="pdse__field">
        <span class="pdse__label">Кому выдаются сведения</span>
        <input
          v-model="form.recipient"
          class="lk-input"
          type="text"
          placeholder="УМВД по г. Москве, либо «субъекту лично»"
          data-testid="pdse-recipient"
        >
      </label>

      <label class="pdse__field">
        <span class="pdse__label">Реквизиты запроса</span>
        <input
          v-model="form.request_ref"
          class="lk-input"
          type="text"
          placeholder="исх. 12/345 от 08.09.2026"
          data-testid="pdse-request"
        >
      </label>

      <label class="pdse__field">
        <span class="pdse__label">Основание <span class="pdse__optional">необязательно</span></span>
        <textarea
          v-model="form.basis"
          class="lk-textarea"
          rows="2"
          placeholder="Запрос о нахождении на объекте"
        />
      </label>

      <div class="pdse__formats">
        <button
          v-for="f in formats"
          :key="f.value"
          type="button"
          class="lk-button"
          :class="form.format === f.value ? 'lk-button--primary' : 'lk-button--ghost'"
          @click="form.format = f.value"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <template #actions>
      <button
        class="lk-button lk-button--ghost"
        type="button"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="lk-button lk-button--primary"
        type="button"
        :disabled="!canSubmit || loading"
        data-testid="pdse-submit"
        @click="submit"
      >
        {{ loading ? 'Готовим...' : 'Выгрузить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script setup>
import { computed, reactive, watch } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';

const props = defineProps({
  show: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
});
const emit = defineEmits(['close', 'submit']);

const formats = [
  { value: 'xlsx', label: 'Excel' },
  { value: 'pdf', label: 'PDF' },
];

const form = reactive({ recipient: '', request_ref: '', basis: '', format: 'xlsx' });

// Кнопка гаснет ровно по тем полям, которые требует сервер: получатель и реквизиты
// запроса. Иначе человек заполнит форму, нажмёт и получит отказ - на нём и узнает,
// что без них выдача невозможна.
const canSubmit = computed(() => form.recipient.trim() !== '' && form.request_ref.trim() !== '');

watch(() => props.show, (open) => {
  if (!open) return;
  form.recipient = '';
  form.request_ref = '';
  form.basis = '';
  form.format = 'xlsx';
});

function submit() {
  if (!canSubmit.value) return;
  emit('submit', { ...form });
}
</script>

<style scoped>
/* base-modal__body идёт без padding - отступы несёт содержимое, как у соседних окон
   (см. ChangePasswordModal). Без них поля упирались в края окна и начинались левее
   заголовка, у которого свой отступ есть. */
.pdse__form {
  padding: 14px 20px 18px;
}

.pdse__hint {
  margin: 0 0 16px;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.5;
}

.pdse__field {
  display: block;
  margin-bottom: 14px;
}

.pdse__label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 500;
}

.pdse__optional {
  color: var(--text-muted);
  font-weight: 400;
}

.pdse__formats {
  display: flex;
  gap: 8px;
}
</style>
