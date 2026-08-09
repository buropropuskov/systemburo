import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// Черновик против профиля (#1457). Форму заполняют два источника: сохранённый в
// браузере черновик и профиль пользователя (/user-data). Профиль перезаписывает
// организацию, компанию, ФИО и телефон безусловно, поэтому важно, кто приходит
// последним. Раньше это был профиль: заявитель, набравший чужую организацию по праву
// application.organization.override (#1437), после перезагрузки молча получал свою.

const PROFILE = {
  organization: 'Своя Орг из профиля',
  organization_id: 11,
  company: 'Своя Компания',
  company_id: 22,
  last_name: 'Профилев',
  first_name: 'Профиль',
  middle_name: 'Профильевич',
  phone: '9990000000',
};

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn((url) => {
    if (url === '/user-data') {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(PROFILE) });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve([]) });
  }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/api/directory', () => ({
  suggestOrganizations: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
  suggestCompanies: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
}));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true }),
}));

/** Кладёт черновик в браузер так, как его сохраняет сама форма. */
function seedDraft(fields) {
  localStorage.setItem('draftApplicationState', JSON.stringify(fields));
}

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

describe('CreateApplication - черновик побеждает профиль (#1457)', () => {
  it('введённая вручную чужая организация не затирается значением из профиля', async () => {
    seedDraft({
      organization: 'Чужая Орг руками',
      organizationId: null,
      company: 'Чужая Компания руками',
      companyId: null,
    });

    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(w.vm.organization).toBe('Чужая Орг руками');
    expect(w.vm.company).toBe('Чужая Компания руками');
    // Идентификатор ручного ввода пуст - наименование ещё не сопоставлено справочнику,
    // и пара «id + название» обязана остаться согласованной, а не смешаться с профилем.
    expect(w.vm.organizationId).toBeNull();
    expect(w.vm.companyId).toBeNull();
  });

  it('ФИО и телефон из черновика тоже переживают загрузку профиля', async () => {
    seedDraft({ responsiblePerson: 'Черновиков Черновик', phoneNumber: '+7 (911) 111-11-11' });

    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(w.vm.responsiblePerson).toBe('Черновиков Черновик');
    expect(w.vm.phoneNumber).toContain('911');
  });

  it('без черновика поля заполняются профилем, как и раньше', async () => {
    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(w.vm.organization).toBe(PROFILE.organization);
    expect(w.vm.organizationId).toBe(PROFILE.organization_id);
    expect(w.vm.company).toBe(PROFILE.company);
    expect(w.vm.responsiblePerson).toBe('Профилев Профиль Профильевич');
  });

  it('сбой профиля не стоит введённого: черновик восстанавливается всё равно', async () => {
    const { apiRequest } = await import('@/api/client');
    apiRequest.mockImplementationOnce(() => Promise.reject(new Error('сеть отвалилась')));
    vi.spyOn(console, 'error').mockImplementation(() => {});
    // Текст сообщения тут не проверяем: черновик без вложений намеренно сбрасывает
    // message и согласие (restoreFromLocalStorage), это своя логика и к профилю
    // отношения не имеет.
    seedDraft({ organization: 'Чужая Орг руками', responsiblePerson: 'Черновиков Ч.' });

    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(w.vm.organization).toBe('Чужая Орг руками');
    expect(w.vm.responsiblePerson).toBe('Черновиков Ч.');
  });
});
