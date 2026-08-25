import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import UserInfoRow from '../UserInfoRow.vue';

// Ввод организации и компании в заявке (#1437): без права поля показывают запись из
// профиля, с правом открыт ручной ввод; подача уходит id выбранной записи либо
// наименованием, введённым руками.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
  createExtendedTimeoutSignal: vi.fn(() => 'FAKE_SIGNAL'),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/api/directory', () => ({
  suggestOrganizations: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
  suggestCompanies: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
}));

const hasPermission = vi.fn().mockReturnValue(false);
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission }),
}));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
  hasPermission.mockReturnValue(false);
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

describe('UserInfoRow - гейт ручного ввода справочников (#1437)', () => {
  const mountRow = (canOverrideDirectory) => mount(UserInfoRow, {
    props: { organization: 'ООО "Победа"', company: '', errors: {}, canOverrideDirectory },
  });

  it('без права организация и компания только для чтения', () => {
    const w = mountRow(false);
    expect(w.get('[data-testid="create-organization"]').attributes('readonly')).toBeDefined();
    expect(w.get('[data-testid="create-company"]').attributes('readonly')).toBeDefined();
    expect(w.text()).toContain('Организация из вашего профиля');
  });

  it('с правом поля редактируются', () => {
    const w = mountRow(true);
    expect(w.get('[data-testid="create-organization"]').attributes('readonly')).toBeUndefined();
    expect(w.get('[data-testid="create-company"]').attributes('readonly')).toBeUndefined();
  });

  // Замок на цепочку событий целиком: подсказка -> DirectorySuggestInput -> UserInfoRow ->
  // CreateApplication. Прямой вызов applyOrganizationChoice в тесте ниже опечатку в имени
  // события в шаблоне не поймает, а без неё выбор подсказки не доедет до id заявки.
  it('выбор подсказки доходит наверх реальным событием', async () => {
    const { suggestOrganizations } = await import('@/api/directory');
    const item = { id: 42, name: 'ООО "Максима Групп"' };
    suggestOrganizations.mockResolvedValueOnce({ items: [item], canonical: item.name, matched: true });

    const w = mountRow(true);
    const input = w.get('[data-testid="create-organization"]');
    input.element.value = 'максима';
    await input.trigger('input');
    await new Promise((resolve) => setTimeout(resolve, 300));
    await flushPromises();

    await w.get('[data-testid="create-organization-option"]').trigger('mousedown');

    expect(w.emitted('select-organization').at(-1)).toEqual([item]);
    expect(w.emitted('update:organization').at(-1)).toEqual([item.name]);
  });
});

describe('CreateApplication - организация и компания в подаче (#1437)', () => {
  it('право открывает ручной ввод', async () => {
    hasPermission.mockImplementation((key) => key === 'application.organization.override');
    const w = await mountApp();
    expect(w.vm.canOverrideDirectory).toBe(true);
  });

  it('выбор подсказки связывает поле с записью, ручная правка связь рвёт', async () => {
    const w = await mountApp();

    w.vm.applyOrganizationChoice({ id: 42, name: 'ООО "Максима Групп"' });
    expect(w.vm.organizationId).toBe(42);
    expect(w.vm.hasOrganization).toBe(true);

    w.vm.applyOrganizationChoice(null);
    expect(w.vm.organizationId).toBeNull();
    // Привязка машин к организации возможна только по записи справочника: у введённой
    // руками организации id появится лишь после подачи.
    expect(w.vm.hasOrganization).toBe(false);

    w.vm.applyCompanyChoice({ id: 7, name: 'ООО "Парк развлечений"' });
    expect(w.vm.companyId).toBe(7);
    w.vm.applyCompanyChoice(null);
    expect(w.vm.companyId).toBeNull();
    expect(w.vm.hasCompany).toBe(false);
  });

  // Ключевое поведение среза: связанное поле уходит id, введённое руками - наименованием.
  // Отправить и то и другое нельзя: при заданном id сервер наименование не смотрит, и
  // чужой текст молча привязался бы к записи профиля.
  it.each([
    {
      title: 'связанное поле уходит id',
      setup: (vm) => { vm.organization = 'ООО "Максима Групп"'; vm.organizationId = 42; },
      expected: { organization_id: 42, organization_name: null },
    },
    {
      title: 'введённое руками наименование уходит текстом',
      setup: (vm) => { vm.organization = 'ООО "Новая Ромашка"'; vm.organizationId = null; },
      expected: { organization_id: null, organization_name: 'ООО "Новая Ромашка"' },
    },
  ])('подача: $title', async ({ setup, expected }) => {
    const { apiRequest } = await import('@/api/client');
    const w = await mountApp();
    w.vm.attachments = [{ local_id: 'a1', attachment_type: 'items', name: 'items', display_name: 'Вещи', id: 1 }];
    w.vm.responsiblePerson = 'Иванов Иван';
    w.vm.phoneNumber = '+7 (900) 123-45-67';
    setup(w.vm);

    apiRequest.mockClear();
    await w.vm.sendCompleteApplication();

    const submit = apiRequest.mock.calls.find(([path]) => path === '/applications/submit-complete-application');
    expect(submit, 'подача должна уйти на бэк').toBeTruthy();
    const body = JSON.parse(submit[1].body);
    expect(body.organization_id).toBe(expected.organization_id);
    expect(body.organization_name).toBe(expected.organization_name);
  });

  it('черновик хранит id рядом с наименованием', async () => {
    const w = await mountApp();
    w.vm.organization = 'ООО "Максима Групп"';
    w.vm.organizationId = 42;
    w.vm.company = 'ООО "Победа"';
    w.vm.companyId = 7;

    w.vm.saveToLocalStorage();
    const draft = JSON.parse(localStorage.getItem('draftApplicationState'));
    expect(draft.organizationId).toBe(42);
    expect(draft.companyId).toBe(7);

    w.vm.organizationId = null;
    w.vm.companyId = null;
    w.vm.restoreFromLocalStorage();
    expect(w.vm.organizationId).toBe(42);
    expect(w.vm.companyId).toBe(7);
  });
});
