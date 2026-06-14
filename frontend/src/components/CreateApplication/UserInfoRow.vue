<template>
  <div class="user-info-row">
    <div class="user__input">
      <label class="input__label">Организация / Отдел</label>
      <input
        class="input"
        placeholder="Введите организацию"
        :value="organization"
        :class="{ 'input--error': errors.organization }"
        @input="$emit('update:organization', $event.target.value)"
        @blur="$emit('validate-field', 'organization')"
      >
      <span class="input__hint">Заполните организацию или компанию</span>
      <div
        v-if="errors.organization"
        class="error-message"
      >
        {{ errors.organization }}
      </div>
    </div>
    <div class="user__input">
      <label class="input__label">Компания</label>
      <input 
        class="input" 
        placeholder="Введите компанию" 
        :value="company"
        :class="{ 'input--error': errors.company }"
        @input="$emit('update:company', $event.target.value)"
        @blur="$emit('validate-field', 'company')"
      >
      <div
        v-if="errors.company"
        class="error-message"
      >
        {{ errors.company }}
      </div>
    </div>
    <div class="user__input responsible">
      <label class="input__label responsible">Ответственное лицо <span class="required">*</span></label>
      <div
        class="input contacts"
        :class="{ 'input--error': errors.responsiblePerson || errors.phone }"
      >
        <input 
          class="contact-input" 
          placeholder="Введите ФИО" 
          :value="responsiblePerson"
          @input="$emit('update:responsible-person', $event.target.value)"
          @blur="$emit('validate-field', 'responsiblePerson')"
        >
        <input 
          class="contact-input phone" 
          placeholder="Номер телефона"
          :value="phoneNumber"
          @input="handlePhoneInput($event)"
          @blur="$emit('format-phone')"
          @focus="$emit('clear-phone')"
        >
      </div>
      <div
        v-if="errors.responsiblePerson"
        class="error-message"
      >
        {{ errors.responsiblePerson }}
      </div>
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
export default {
    name: 'UserInfoRow',
    props: {
        organization: { type: String, default: null },
        company: { type: String, default: null },
        responsiblePerson: { type: String, default: null },
        phoneNumber: { type: String, default: null },
        errors: { type: Object, default: () => ({}) }
    },
    emits: [
        'update:organization',
        'update:company',
        'update:responsible-person',
        'update:phone-number',
        'validate-field',
        'format-phone',
        'clear-phone'
    ],
    methods: {
        handlePhoneInput(event) {
            this.$emit('update:phone-number', event.target.value);
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
    border-bottom: 1px solid #e6e6e6;
}

.user__input {
    width: 260px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
}

.responsible {
    width: 430px !important;
}

.contacts {
    display: flex;
    justify-content: space-between;
}

.contact-input {
    border: none;
    outline: none;
    background: transparent;
    width: 60%;
}

.phone {
    width: 35%;
    text-align: end;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.input__hint {
    font-size: 11px;
    color: #a2a2a2;
}

.required {
    color: #ff4444;
}

.input {
    width: 100%;
    height: 40px;
    border: 1px solid #e6e6e6;
    outline: none;
    background: #FFF;
    border-radius: 15px;
    padding: 5px 10px;
}

.input--error {
    border-color: #ff4444;
}

.error-message {
    font-size: 11px;
    color: #ff4444;
    position: absolute;
    bottom: -15px;
    left: 0;
}
</style>