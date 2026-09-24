<template>
  <BaseModal
    :show="show"
    title="Сменить пароли всем работникам?"
    width="440px"
    content-testid="rotation-confirm"
    @close="$emit('close')"
  >
    <div class="rotation-confirm">
      <p>
        Пароли сменятся у <b>{{ eligible }}</b> работников с указанным
        адресом почты. Каждому уйдёт письмо с новым паролем.
      </p>
      <p>
        Все текущие сессии будут завершены - людям придётся войти заново.
        <span v-if="withoutEmail > 0">
          Работников без почты ({{ withoutEmail }}) действие не затронет.
        </span>
      </p>
    </div>

    <template #actions>
      <button
        class="btn btn--secondary"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        class="btn btn--danger"
        data-testid="rotation-confirm-button"
        @click="$emit('confirm')"
      >
        Сменить пароли
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';

/**
 * Подтверждение массовой смены паролей: кого затронет и что все сессии оборвутся.
 */
export default {
  name: 'RotationConfirmModal',
  components: { BaseModal },
  props: {
    show: { type: Boolean, required: true },
    eligible: { type: Number, default: 0 },
    withoutEmail: { type: Number, default: 0 },
  },
  emits: ['close', 'confirm'],
};
</script>

<style scoped>
.rotation-confirm {
  padding: 16px 20px;
}

.rotation-confirm p {
  margin: 0 0 10px;
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
}

.rotation-confirm p:last-child {
  margin-bottom: 0;
}
</style>
