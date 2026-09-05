import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }),
}));

import EmployeeForm from '../EmployeeForm.vue';
import { toEmployeePayload } from '@/utils/applicationEntityPayload';

const REQUIRED = { visible: true, required: true };
const CONFIG = {
  last_name: REQUIRED, first_name: REQUIRED, position: REQUIRED, citizenship: REQUIRED,
  passport: REQUIRED, patent: { visible: true, required: false }, target_tables: { visible: false, required: false },
  pd_consent: REQUIRED,
};

async function mountForm(config = CONFIG) {
  const w = mount(EmployeeForm, { props: { fieldConfig: config } });
  await flushPromises();
  await w.setData({
    selectedCitizenship: { id: 1, name: 'Россия', patent_required: false },
    lastName: 'Пешков',
    firstName: 'Иван',
    position: 'Монтажник',
    passportSeriesNumber: '4510 111222',
  });
  return w;
}

// Согласие субъекта на обработку персональных данных (152-ФЗ): у сотрудника вводят
// паспорт и патент, то есть данные третьего лица. Отметка ставится на каждого человека
// отдельно и уходит в подачу флагом - дату и автора пишет сервер.
describe('EmployeeForm - согласие субъекта на обработку персональных данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('без отметки кнопка добавления заблокирована и объясняет причину', async () => {
    const w = await mountForm();

    expect(w.vm.canAddEmployee).toBe(false);
    expect(w.vm.getTooltipMessage).toContain('уведомлён об обработке персональных данных');

    await w.find('[data-testid="employee-pd-consent"]').setValue(true);
    expect(w.vm.canAddEmployee).toBe(true);
  });

  it('добавленный сотрудник несёт отметку, а форма её сбрасывает под следующего', async () => {
    const w = await mountForm();
    await w.find('[data-testid="employee-pd-consent"]').setValue(true);

    w.vm.addEmployee();
    await flushPromises();

    const added = w.emitted('employee-added')[0][0];
    expect(added.pdConsent).toBe(true);
    expect(toEmployeePayload([added])[0].pd_consent).toBe(true);
    expect(w.vm.pdConsent).toBe(false, 'следующего работника подтверждают заново');
  });

  it('поле, выключенное в шаблоне, не показывается и не блокирует добавление', async () => {
    const w = await mountForm({ ...CONFIG, pd_consent: { visible: false, required: false } });

    expect(w.find('[data-testid="employee-pd-consent"]').exists()).toBe(false);
    expect(w.vm.canAddEmployee).toBe(true);
  });

  it('у сотрудника из реестра отметку не спрашивают - согласие получено при заведении записи', async () => {
    const w = await mountForm();
    await w.setData({ selectedExistingEmployees: [{ id: 5, last_name: 'Тихонов', first_name: 'Лев', pd_consent_at: '2026-08-15T10:00:00Z' }] });

    expect(w.find('[data-testid="employee-pd-consent"]').exists()).toBe(false);
  });
});
