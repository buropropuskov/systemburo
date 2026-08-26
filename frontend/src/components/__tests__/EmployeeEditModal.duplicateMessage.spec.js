import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Сообщение о дубле (#2021).
 *
 * Сервер различает три случая и пишет по-разному: запись уже привязана к вам,
 * уже есть в этой организации, уже есть в этой компании (`unique_employee_service.go`).
 * Форма перехватывала два последних по подстроке «уже существует» и подменяла их
 * на «уже привязан к вашему аккаунту» - человек шёл искать сотрудника в «Мои
 * сотрудники», не находил и заводил заново либо шёл в бюро с вопросом.
 *
 * Тексты в фикстурах - дословно те, что отдаёт сервер; снято со стенда.
 */

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));

import EmployeeEditModal from '../EmployeeEditModal.vue';

const CITIZENSHIPS = [{ id: 1, name: 'Россия', patent_required: false }];
const OWNERSHIP = {
  user_id: 1, has_organization: true, has_company: true,
  organization_id: 10, organization_name: 'Бюро пропусков',
  company_id: 20, company_name: 'Компания бюро',
};

/** Ответ клиента на ошибку: `wrapJsonUnwrap` кладёт текст сервера в `message`. */
function serverError(text) {
  return { ok: false, json: vi.fn().mockResolvedValue({ message: text }) };
}

function mountModal() {
  return mount(EmployeeEditModal, {
    props: { visible: true, citizenships: CITIZENSHIPS, ownershipInfo: OWNERSHIP },
    global: { stubs: { teleport: true } },
  });
}

async function trySave(wrapper) {
  await wrapper.setData({
    lastName: 'Васьков', firstName: 'Денис', middleName: 'Александрович',
    position: 'Начальник', passportSeriesNumber: '5213 51235',
    selectedCitizenship: CITIZENSHIPS[0], pdConsent: true,
  });
  await wrapper.vm.saveEmployee();
}

describe('EmployeeEditModal - сообщение о дубле', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('запись в организации: показываем текст сервера, а не «привязан к вашему аккаунту»', async () => {
    apiRequest.mockResolvedValue(serverError('Сотрудник с такими паспортными данными уже существует в этой организации'));
    const wrapper = mountModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await trySave(wrapper);

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Сотрудник с такими паспортными данными уже существует в этой организации',
      type: 'error',
    }));
  });

  it('запись в компании: тот же порядок - текст сервера как есть', async () => {
    apiRequest.mockResolvedValue(serverError('Сотрудник с такими паспортными данными уже существует в этой компании'));
    const wrapper = mountModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await trySave(wrapper);

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Сотрудник с такими паспортными данными уже существует в этой компании',
    }));
  });

  it('запись действительно у вас: сервер так и пишет, подменять нечего', async () => {
    apiRequest.mockResolvedValue(serverError('Сотрудник с такими паспортными данными уже привязан к вашему аккаунту'));
    const wrapper = mountModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await trySave(wrapper);

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Сотрудник с такими паспортными данными уже привязан к вашему аккаунту',
    }));
  });

  it('сервер молчит - остаётся общая формулировка формы', async () => {
    apiRequest.mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue({}) });
    const wrapper = mountModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await trySave(wrapper);

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'Ошибка при сохранении сотрудника',
    }));
  });
});
