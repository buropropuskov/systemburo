import { describe, it, expect, beforeEach } from 'vitest';
import { ref } from 'vue';
import { useFormValidation } from '../useFormValidation';

describe('useFormValidation', () => {
  let name;
  let email;

  beforeEach(() => {
    name = ref('');
    email = ref('');
  });

  function createValidation() {
    return useFormValidation(() => [
      { field: 'name', check: !!name.value, message: 'Имя' },
      { field: 'email', check: !!email.value, message: 'Email' },
    ]);
  }

  describe('isValid', () => {
    it('returns false when required fields are empty', () => {
      const { isValid } = createValidation();
      expect(isValid.value).toBe(false);
    });

    it('returns true when all fields are filled', () => {
      name.value = 'Сергей';
      email.value = 'test@mail.ru';
      const { isValid } = createValidation();
      expect(isValid.value).toBe(true);
    });

    it('returns false when only some fields are filled', () => {
      name.value = 'Сергей';
      const { isValid } = createValidation();
      expect(isValid.value).toBe(false);
    });
  });

  describe('missingFields', () => {
    it('lists all missing field messages', () => {
      const { missingFields } = createValidation();
      expect(missingFields.value).toEqual(['Имя', 'Email']);
    });

    it('returns empty array when all fields are valid', () => {
      name.value = 'Сергей';
      email.value = 'test@mail.ru';
      const { missingFields } = createValidation();
      expect(missingFields.value).toEqual([]);
    });
  });

  describe('tooltipMessage', () => {
    // Заголовок нейтральный: в списке причин лежат не только незаполненные поля,
    // но и запреты («Машина в чёрном списке», «уже есть машина По факту»), и с
    // «Заполните поля» они читались как требование что-то ввести (#2320).
    it('shows single field message', () => {
      name.value = 'Сергей';
      const { tooltipMessage } = createValidation();
      expect(tooltipMessage.value).toBe('Не хватает: Email');
    });

    it('shows multiple fields message', () => {
      const { tooltipMessage } = createValidation();
      expect(tooltipMessage.value).toContain('Не хватает:');
      expect(tooltipMessage.value).toContain('• Имя');
      expect(tooltipMessage.value).toContain('• Email');
    });

    it('returns empty string when valid', () => {
      name.value = 'Сергей';
      email.value = 'test@mail.ru';
      const { tooltipMessage } = createValidation();
      expect(tooltipMessage.value).toBe('');
    });
  });

  describe('validateField', () => {
    it('sets error for invalid field', () => {
      const { validateField, getFieldError } = createValidation();
      validateField('name');
      expect(getFieldError('name')).toBe('Имя');
    });

    it('clears error when field becomes valid', () => {
      const { validateField, getFieldError } = createValidation();
      validateField('name');
      expect(getFieldError('name')).toBe('Имя');

      name.value = 'Сергей';
      validateField('name');
      expect(getFieldError('name')).toBe('');
    });

    it('does not return error for untouched field', () => {
      const { getFieldError } = createValidation();
      expect(getFieldError('name')).toBe('');
    });
  });

  describe('validateAll', () => {
    it('returns false and sets errors for all empty fields', () => {
      const { validateAll, getFieldError } = createValidation();
      const result = validateAll();

      expect(result).toBe(false);
      expect(getFieldError('name')).toBe('Имя');
      expect(getFieldError('email')).toBe('Email');
    });

    it('returns true when all fields are valid', () => {
      name.value = 'Сергей';
      email.value = 'test@mail.ru';
      const { validateAll } = createValidation();
      expect(validateAll()).toBe(true);
    });
  });

  describe('resetValidation', () => {
    it('clears all errors and touched state', () => {
      const { validateAll, resetValidation, getFieldError, fieldErrors, touched } = createValidation();
      validateAll();
      expect(getFieldError('name')).toBe('Имя');

      resetValidation();
      expect(fieldErrors.value).toEqual({});
      expect(touched.value).toEqual({});
      expect(getFieldError('name')).toBe('');
    });
  });
});
