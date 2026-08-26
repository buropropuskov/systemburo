import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({ id: 3 }) });
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [{ id: 1, name: 'Россия', patent_required: false }];
const OWNERSHIP = {
  user_id: 1, has_organization: true, has_company: false,
  organization_id: 10, organization_name: 'ООО Ромашка', company_id: null, company_name: null,
};

function mountModal(props = {}) {
  return mount(EmployeeEditModal, {
    props: { visible: true, citizenships: CITIZENSHIPS, ownershipInfo: OWNERSHIP, ...props },
    global: { stubs: { teleport: true } },
  });
}

async function fill(wrapper) {
  await wrapper.setData({
    lastName: 'Пешков', firstName: 'Иван', position: 'Монтажник',
    passportSeriesNumber: '4510 111222', selectedCitizenship: CITIZENSHIPS[0],
  });
}

// Карточка реестра - вторая точка ввода персональных данных работника, и там согласие
// требуется всегда: шаблонов полей у реестра нет, а сервер без отметки запись не создаёт
// (uniqueEmployeeService.Create). У записи с уже полученным согласием показываем дату:
// подтверждать второй раз нечего, а снять отметку правкой нельзя.
describe('EmployeeEditModal - согласие субъекта на обработку персональных данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockClear();
  });

  it('новая запись не сохраняется без отметки, с отметкой уходит флаг', async () => {
    const wrapper = mountModal();
    await fill(wrapper);

    expect(wrapper.vm.canSaveEmployee).toBe(false);
    expect(wrapper.vm.saveEmployeeHint).toContain('согласие');

    await wrapper.find('[data-testid="employee-registry-pd-consent"]').setValue(true);
    expect(wrapper.vm.canSaveEmployee).toBe(true);

    await wrapper.vm.saveEmployee();

    const [url, options] = apiRequest.mock.calls[0];
    expect(url).toBe('/unique-employees');
    expect(JSON.parse(options.body).pd_consent).toBe(true);
  });

  // Форма после добавления остаётся открытой - следующего работника вводят сразу. Отметка
  // обязана сниматься вместе с данными: подтверждают конкретного человека, а не всех, кого
  // заведут дальше. Оставшись стоять, она превращала осознанное подтверждение в состояние
  // формы, и руководство пользователя обещало обратное («после нажатия "Добавить" снимается»).
  it('после успешного добавления отметка снимается вместе с полями', async () => {
    const wrapper = mountModal();
    await fill(wrapper);
    await wrapper.find('[data-testid="employee-registry-pd-consent"]').setValue(true);

    await wrapper.vm.saveEmployee();
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.pdConsent).toBe(false);
    expect(wrapper.find('[data-testid="employee-registry-pd-consent"]').element.checked).toBe(false);
    expect(wrapper.vm.lastName).toBe('');
    expect(wrapper.vm.passportSeriesNumber).toBe('');
    // Следующая запись без новой отметки не уходит - гейт снова закрыт.
    expect(wrapper.vm.canSaveEmployee).toBe(false);
  });

  it('у записи с полученным согласием показывает дату вместо галочки и не мешает сохранению', async () => {
    const wrapper = mountModal({
      editingEmployee: {
        id: 3, last_name: 'Пешков', first_name: 'Иван', position: 'Монтажник',
        citizenship_id: 1, passport_series_number: '4510 111222',
        organization_id: 10, company_id: null, user_id: 1,
        pd_consent_at: '2026-08-15T10:00:00Z',
      },
    });

    const granted = wrapper.find('[data-testid="employee-consent-granted"]');
    expect(granted.exists()).toBe(true);
    expect(granted.text()).toContain('15.08.2026');
    expect(wrapper.find('[data-testid="employee-registry-pd-consent"]').exists()).toBe(false);
    expect(wrapper.vm.canSaveEmployee).toBe(true);

    await wrapper.setData({ position: 'Старший монтажник' });
    await wrapper.vm.saveEmployee();

    const [, options] = apiRequest.mock.calls[0];
    expect(JSON.parse(options.body).pd_consent).toBe(true, 'отметка остаётся у записи после правки');
  });

  it('у записи без отметки согласие подтверждают заново', async () => {
    const wrapper = mountModal({
      editingEmployee: {
        id: 4, last_name: 'Кротов', first_name: 'Семён', position: 'Слесарь',
        citizenship_id: 1, passport_series_number: '4510 333444',
        organization_id: 10, company_id: null, user_id: 1, pd_consent_at: null,
      },
    });

    expect(wrapper.find('[data-testid="employee-registry-pd-consent"]').exists()).toBe(true);
    expect(wrapper.vm.canSaveEmployee).toBe(false);
  });
});
