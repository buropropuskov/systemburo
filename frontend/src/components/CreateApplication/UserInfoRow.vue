<template>
  <div class="user-info-row">
    <div class="user__input">
      <DirectorySuggestInput
        :model-value="organization"
        label="Организация / Отдел"
        placeholder="Введите организацию"
        :hint="canOverrideDirectory ? 'Заполните организацию или компанию' : 'Организация из вашего профиля'"
        :error="errors.organization"
        :editable="canOverrideDirectory"
        :fetcher="suggestOrganizations"
        testid="create-organization"
        @update:model-value="$emit('update:organization', $event)"
        @select="$emit('select-organization', $event)"
        @validate="$emit('validate-field', 'organization')"
      />
    </div>
    <div class="user__input">
      <DirectorySuggestInput
        :model-value="company"
        label="Компания"
        placeholder="Введите компанию"
        :hint="canOverrideDirectory ? '' : 'Компания из вашего профиля'"
        :error="errors.company"
        :editable="canOverrideDirectory"
        :fetcher="suggestCompanies"
        testid="create-company"
        @update:model-value="$emit('update:company', $event)"
        @select="$emit('select-company', $event)"
        @validate="$emit('validate-field', 'company')"
      />
    </div>
    <div class="user__input">
      <label class="input__label">Инициатор заявки <span class="required">*</span></label>
      <input
        class="input"
        placeholder="Введите ФИО"
        :value="responsiblePerson"
        :class="{ 'input--error': errors.responsiblePerson }"
        @input="$emit('update:responsible-person', $event.target.value)"
        @blur="$emit('validate-field', 'responsiblePerson')"
      >
      <div
        v-if="errors.responsiblePerson"
        class="error-message"
      >
        {{ errors.responsiblePerson }}
      </div>
    </div>
    <div class="user__input">
      <label class="input__label">Телефон <span class="required">*</span></label>
      <input
        class="input"
        placeholder="Номер телефона"
        maxlength="18"
        :value="phoneNumber"
        :class="{ 'input--error': errors.phone }"
        @input="handlePhoneInput($event)"
        @blur="$emit('format-phone')"
        @focus="$emit('clear-phone')"
      >
      <div
        v-if="errors.phone"
        class="error-message"
      >
        {{ errors.phone }}
      </div>
    </div>
  </div>
</template>

<script>
import { formatPhoneNumberImmediately } from '@/composables/usePhoneFormat'
import { suggestCompanies, suggestOrganizations } from '@/api/directory'
import DirectorySuggestInput from './DirectorySuggestInput.vue'

export default {
    name: 'UserInfoRow',
    components: { DirectorySuggestInput },
    props: {
        organization: { type: String, default: null },
        company: { type: String, default: null },
        responsiblePerson: { type: String, default: null },
        phoneNumber: { type: String, default: null },
        errors: { type: Object, default: () => ({}) },
        // Право application.organization.override: без него организация и компания
        // заявки берутся из профиля и правке не подлежат (#1437).
        canOverrideDirectory: { type: Boolean, default: false }
    },
    emits: [
        'update:organization',
        'update:company',
        'update:responsible-person',
        'update:phone-number',
        'select-organization',
        'select-company',
        'validate-field',
        'format-phone',
        'clear-phone'
    ],
    methods: {
        suggestOrganizations,
        suggestCompanies,

        handlePhoneInput(event) {
            // Живое форматирование: не больше 11 цифр, маска +7 (XXX) XXX-XX-XX.
            const digits = event.target.value.replace(/\D/g, '').slice(0, 11);
            const { formatted } = formatPhoneNumberImmediately(digits);
            const value = formatted || digits;
            // Поле на one-way :value, поэтому синхронизируем отображение сразу.
            event.target.value = value;
            this.$emit('update:phone-number', value);
            this.$emit('validate-field', 'phone');
        }
    }
}
</script>

<style scoped>
.user-info-row {
    display: flex;
    gap: 30px;
    padding: 15px;
    border-bottom: 1px solid var(--border);
}

.user__input {
    width: 260px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.input__hint {
    font-size: 11px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.input {
    width: 100%;
    height: 40px;
    border: 1px solid var(--border);
    outline: none;
    background: var(--surface);
    border-radius: 15px;
    padding: 5px 10px;
}

.input--error {
    border-color: var(--danger);
}

.error-message {
    font-size: 11px;
    color: var(--danger-text);
    position: absolute;
    bottom: -15px;
    left: 0;
}

/* 4 фикс-ширных (260px) поля в ряд не влезают на узком - стекаем в колонку. */
@media (max-width: 768px) {
    .user-info-row {
        flex-direction: column;
        gap: 16px;
        padding: 12px;
    }

    .user__input {
        width: 100%;
    }
}
</style>