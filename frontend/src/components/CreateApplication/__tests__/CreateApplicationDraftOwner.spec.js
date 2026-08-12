import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

/**
 * Черновик заявки принадлежит учётной записи, а не браузеру.
 *
 * Хранилище одно на устройство, а в бюро и на проходной за одним компьютером
 * работают посменно: арендатор открывал форму и видел в шапке организацию и
 * компанию предыдущего работника, а сервер отбивал такую заявку отказом «подать от
 * другой организации». Найдено руками на стенде.
 */

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
vi.mock('@/api/directory', () => ({
  suggestOrganizations: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
  suggestCompanies: vi.fn().mockResolvedValue({ items: [], canonical: '', matched: false }),
}));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true }),
}));

// Текущий работник задаётся маркером доступа - так его берёт и сама форма.
let currentUserId = 7;
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', userPayload: { user_id: currentUserId } }),
}));

const CHUZHOY_DRAFT = {
  ownerId: 42,
  organization: 'Отдел контроля доступа',
  organizationId: 5,
  company: 'Бюро пропусков',
  companyId: 6,
  message: 'черновик соседа по смене',
};

const draftInStorage = () => JSON.parse(localStorage.getItem('draftApplicationState') || 'null');

async function mountForm() {
  const wrapper = shallowMount(CreateApplication, { global: { stubs: { teleport: true } } });
  await flushPromises();
  return wrapper;
}

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
  currentUserId = 7;
});

describe('CreateApplication - черновик принадлежит учётной записи', () => {
  it('черновик другого работника не подставляется и стирается', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify(CHUZHOY_DRAFT));

    const wrapper = await mountForm();

    expect(wrapper.vm.organization, 'в шапке должна остаться своя организация').toBe(PROFILE.organization);
    expect(wrapper.vm.company).toBe(PROFILE.company);
    expect(wrapper.vm.message).toBe('');
    expect(draftInStorage(), 'чужой черновик не остаётся лежать в браузере').toBeNull();
  });

  it('свой черновик восстанавливается как раньше', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify({
      ...CHUZHOY_DRAFT,
      ownerId: currentUserId,
      organization: 'Организация из своего черновика',
      company: 'Компания из своего черновика',
    }));

    const wrapper = await mountForm();

    // Сообщение здесь не проверяем: черновик без вложений его сбрасывает сам,
    // независимо от владельца.
    expect(wrapper.vm.organization).toBe('Организация из своего черновика');
    expect(wrapper.vm.company).toBe('Компания из своего черновика');
    expect(draftInStorage(), 'свой черновик остаётся в браузере').not.toBeNull();
  });

  it('черновик, сохранённый до этой правки, владельца не имеет и не подставляется', async () => {
    const { ownerId, ...bezVladeltsa } = CHUZHOY_DRAFT;
    void ownerId;
    localStorage.setItem('draftApplicationState', JSON.stringify(bezVladeltsa));

    const wrapper = await mountForm();

    expect(wrapper.vm.organization, 'чей это черновик - неизвестно, берём профиль').toBe(PROFILE.organization);
    expect(draftInStorage()).toBeNull();
  });

  it('сохранение проставляет владельца', async () => {
    const wrapper = await mountForm();

    wrapper.vm.message = 'моя заявка';
    wrapper.vm.saveToLocalStorage();

    expect(draftInStorage().ownerId).toBe(currentUserId);
  });

  it('дубль заявки, отложенный другим работником, не подхватывается', async () => {
    localStorage.setItem('pendingDuplicateState', JSON.stringify({
      ownerId: 42,
      message: 'дубль соседа',
      attachments: [],
    }));

    const wrapper = await mountForm();

    expect(wrapper.vm.message).toBe('');
    expect(localStorage.getItem('pendingDuplicateState')).toBeNull();
  });
});
