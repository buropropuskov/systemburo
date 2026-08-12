<template>
  <BaseModal
    :show="show"
    :title="mandatory ? 'Задайте свой пароль' : 'Смена пароля'"
    width="480px"
    radius="30px"
    :closable="!mandatory"
    :close-on-overlay="!mandatory"
    :z-index="mandatory ? 25400 : 1000"
    content-testid="change-password-modal"
    @close="$emit('close')"
  >
    <form
      class="cp-form"
      @submit.prevent="submit"
    >
      <p
        v-if="mandatory"
        class="cp-reason"
        data-testid="cp-reason"
      >
        Присланный пароль лежит в почтовом ящике открытым текстом, поэтому действует
        только до первой смены. Пока свой пароль не задан, разделы системы закрыты.
      </p>

      <label class="cp-field">
        <span class="cp-label">Текущий пароль</span>
        <PasswordInput
          v-model="currentPassword"
          autocomplete="current-password"
          placeholder="Введите текущий пароль"
          data-testid="cp-current"
        />
      </label>

      <label class="cp-field">
        <span class="cp-label">Новый пароль</span>
        <PasswordInput
          v-model="newPassword"
          placeholder="Придумайте новый пароль"
          data-testid="cp-new"
        />
      </label>

      <label class="cp-field">
        <span class="cp-label">Новый пароль ещё раз</span>
        <PasswordInput
          v-model="repeatPassword"
          placeholder="Повторите новый пароль"
          data-testid="cp-repeat"
        />
      </label>

      <div class="cp-checklist">
        <span class="cp-checklist__title">Требования к паролю</span>
        <ul
          class="cp-rules"
          data-testid="cp-rules"
        >
          <li
            v-for="rule in rules"
            :key="rule.key"
            class="cp-rule"
            :class="{ 'cp-rule--ok': rule.ok }"
          >
            <span
              class="cp-rule__mark"
              aria-hidden="true"
            >{{ rule.ok ? '✓' : '' }}</span>
            {{ rule.label }}
          </li>
          <li
            class="cp-rule"
            :class="{ 'cp-rule--ok': repeatMatches }"
          >
            <span
              class="cp-rule__mark"
              aria-hidden="true"
            >{{ repeatMatches ? '✓' : '' }}</span>
            Пароли совпадают
          </li>
        </ul>
      </div>

      <p class="cp-note">
        После смены пароля вход на всех устройствах потребуется выполнить заново.
      </p>
    </form>

    <template #actions>
      <button
        v-if="!mandatory"
        type="button"
        class="lk-button lk-button--ghost"
        :disabled="saving"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <!-- Обязательное окно закрыть нечем, поэтому выход обязан быть внутри него:
           иначе человек, не нашедший письмо, заперт в форме без единого действия. -->
      <button
        v-else
        type="button"
        class="lk-button lk-button--ghost"
        :disabled="saving"
        data-testid="cp-logout"
        @click="$emit('logout')"
      >
        Выйти
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        :disabled="!canSubmit"
        data-testid="cp-submit"
        @click="submit"
      >
        {{ saving ? 'Сохраняем...' : 'Сменить пароль' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import { BaseModal } from '@/components/ui';
import PasswordInput from './ui/PasswordInput.vue';
import { changeOwnPassword } from '@/api/users';
import { getPasswordPolicy } from '@/api/settings';
import { DEFAULT_PASSWORD_POLICY, evaluatePassword, passwordMeetsPolicy } from '@/utils/passwordPolicy';
import { useDeletionsStore } from '@/stores/deletions';
import { useAuthStore } from '@/stores/auth';

export default {
  name: 'ChangePasswordModal',
  components: { BaseModal, PasswordInput },
  props: {
    show: { type: Boolean, required: true },
    // Обязательная смена (#1911): окно нельзя закрыть ни крестиком, ни по затемнению,
    // ни Escape, и отмены у него нет - до смены пароля система всё равно отвечает
    // отказом, и закрытое окно оставило бы человека перед пустым экраном.
    mandatory: { type: Boolean, default: false },
  },
  emits: ['close', 'changed', 'logout'],
  data() {
    return {
      currentPassword: '',
      newPassword: '',
      repeatPassword: '',
      policy: { ...DEFAULT_PASSWORD_POLICY },
      saving: false,
    };
  },
  computed: {
    rules() {
      return evaluatePassword(this.policy, this.newPassword);
    },
    repeatMatches() {
      return this.newPassword.length > 0 && this.newPassword === this.repeatPassword;
    },
    canSubmit() {
      return !this.saving
        && this.currentPassword.length > 0
        && this.repeatMatches
        && passwordMeetsPolicy(this.policy, this.newPassword);
    },
  },
  watch: {
    show(opened) {
      if (opened) {
        this.reset();
        this.loadPolicy();
      }
    },
  },
  methods: {
    reset() {
      this.currentPassword = '';
      this.newPassword = '';
      this.repeatPassword = '';
      this.saving = false;
    },
    async loadPolicy() {
      try {
        this.policy = await getPasswordPolicy();
      } catch (error) {
        // Политика - подсказка, а не гейт: настоящую проверку делает сервер.
        // Упавшая загрузка оставляет дефолтный чеклист, форму не блокирует.
        console.error('Не удалось загрузить политику паролей:', error);
      }
    },
    async submit() {
      if (!this.canSubmit) return;
      this.saving = true;
      try {
        const response = await changeOwnPassword(this.currentPassword, this.newPassword);
        if (response.ok) {
          useDeletionsStore().notify({ bold: 'Пароль изменён', suffix: ', войдите заново' });
          this.$emit('changed');
          // Сервер отозвал все маркеры продления, включая маркер этой вкладки:
          // держать интерфейс залогиненным нечестно - он умрёт на первом же
          // продлении. Чистим сессию сами и уводим на вход, как это делает
          // client.js при истёкшей сессии.
          useAuthStore().clearTokens();
          if (this.$router.currentRoute.value.path !== '/') {
            this.$router.push('/');
          }
          return;
        }
        const errorData = await response.json().catch(() => ({}));
        useDeletionsStore().notify({
          prefix: 'Не удалось сменить пароль: ',
          bold: errorData.message || 'ошибка',
          type: 'error',
        });
      } catch (error) {
        console.error('Ошибка сети при смене пароля:', error);
        useDeletionsStore().notify({
          prefix: 'Не удалось сменить пароль: ',
          bold: 'нет связи с сервером',
          type: 'error',
        });
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
/* base-modal__body идёт без padding - отступы несёт содержимое (как у соседних
   окон). Без них поля и чеклист упирались в края окна и начинались левее
   заголовка, у которого свой отступ есть. */
.cp-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 14px 20px 18px;
}

.cp-field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.cp-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

/* Требования - отдельной карточкой, а не хвостом формы: человек читает их, пока
   набирает пароль, и они не должны сливаться с полями. */
.cp-checklist {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.cp-checklist__title {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}

.cp-rules {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 6px 14px;
  font-size: 13px;
}

.cp-rule {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  transition: color 0.15s ease;
}

/* Кружок-заглушка и галочка занимают одно место, поэтому строка не дёргается,
   когда требование выполняется. */
.cp-rule__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 1px solid var(--border);
  font-size: 11px;
  line-height: 1;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.cp-rule--ok {
  color: var(--text);
}

.cp-rule--ok .cp-rule__mark {
  background: var(--success);
  border-color: var(--success);
  color: #fff;
}

.cp-note {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.cp-reason {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-2);
  font-size: 13px;
  line-height: 1.4;
  color: var(--text);
}

/* На узком экране окно выезжает листом снизу и шапка ужимается до 16px по бокам -
   содержимое идёт тем же полем, иначе оно шире собственного заголовка. */
@media (max-width: 768px) {
  .cp-form {
    padding: 12px 16px 16px;
    gap: 14px;
  }

  /* Требования в одну колонку: две по 180px в лист шириной 390px не встают, и
     вторая колонка обрезалась по правому краю. */
  .cp-rules {
    grid-template-columns: 1fr;
  }
}
</style>
