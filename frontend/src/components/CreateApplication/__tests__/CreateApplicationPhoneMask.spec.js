import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import UserInfoRow from '../UserInfoRow.vue';

// Телефон в подаче заявки (userbugs-0728): маска накладывается по мере ввода, а
// «Введите корректный номер» не выскакивает на недобранном номере и не остаётся
// висеть на корректном.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/api/directory', () => ({
  suggestOrganizations: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
  suggestCompanies: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
}));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: vi.fn().mockReturnValue(false) }),
}));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

function mountRow() {
  return mount(UserInfoRow, { props: { errors: {} } });
}

/** Печатает в поле телефона по одному символу, как пользователь. */
async function typePhone(wrapper, text) {
  const input = wrapper.get('input[placeholder="Номер телефона"]');
  for (const char of text) {
    input.element.value = input.element.value + char;
    await input.trigger('input');
  }
  return input;
}

describe('UserInfoRow - маска телефона при вводе', () => {
  it('накладывает маску с первой цифры, а не по готовому номеру', async () => {
    const w = mountRow();
    const input = await typePhone(w, '9');
    expect(input.element.value).toBe('+7 (9');

    await typePhone(w, '16');
    expect(input.element.value).toBe('+7 (916)');
  });

  it('доводит номер до полной маски и отдаёт её наверх', async () => {
    const w = mountRow();
    const input = await typePhone(w, '9161234567');

    expect(input.element.value).toBe('+7 (916) 123 45-67');
    const emitted = w.emitted('update:phone-number');
    expect(emitted[emitted.length - 1][0]).toBe('+7 (916) 123 45-67');
  });

  it('одно нажатие Backspace стирает цифру, даже если под кареткой был разделитель', async () => {
    const w = mount(UserInfoRow, { props: { errors: {}, phoneNumber: '+7 (916) 123 45-67' } });
    const input = w.get('input[placeholder="Номер телефона"]');

    // Браузер уже съел ")" - каретка стоит на его месте, набор цифр не изменился
    input.element.value = '+7 (916 123 45-67';
    input.element.setSelectionRange(7, 7);
    await input.trigger('input', { inputType: 'deleteContentBackward' });

    expect(input.element.value).toBe('+7 (911) 234 56-7');
  });

  it('ввод просит live-валидацию, blur - обычную', async () => {
    const w = mountRow();
    await typePhone(w, '9');
    expect(w.emitted('validate-field')[0]).toEqual(['phone', { live: true }]);

    await w.get('input[placeholder="Номер телефона"]').trigger('blur');
    expect(w.emitted('format-phone')).toHaveLength(1);
  });
});

describe('CreateApplication - валидация телефона', () => {
  const mountApp = async () => {
    const w = shallowMount(CreateApplication);
    await flushPromises();
    return w;
  };

  it('не ругается на недобранный номер по ходу ввода', async () => {
    const w = await mountApp();
    w.vm.phoneNumber = '+7 (916) 12';
    w.vm.validateField('phone', { live: true });
    expect(w.vm.errors.phone).toBe('');
  });

  it('показывает ошибку на битом полном номере уже по ходу ввода', async () => {
    const w = await mountApp();
    w.vm.phoneNumber = '+7 (016) 123 45-67';
    w.vm.validateField('phone', { live: true });
    expect(w.vm.errors.phone).toBe('Введите корректный номер');
  });

  it('принимает корректный номер после blur (регресс: ложная ошибка на +7)', async () => {
    const w = await mountApp();
    w.vm.phoneNumber = '+79161234567';
    w.vm.handleFormatPhoneNumber();

    expect(w.vm.phoneNumber).toBe('+7 (916) 123 45-67');
    expect(w.vm.errors.phone).toBe('');
  });

  it('на blur пустое поле остаётся обязательным, по ходу ввода - нет', async () => {
    const w = await mountApp();
    w.vm.phoneNumber = '';

    w.vm.validateField('phone', { live: true });
    expect(w.vm.errors.phone).toBe('');

    w.vm.validateField('phone');
    expect(w.vm.errors.phone).toBe('Обязательное поле');
  });
});
