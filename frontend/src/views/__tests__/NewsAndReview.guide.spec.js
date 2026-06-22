import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

const hasPermission = vi.fn();
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission }),
}));

import NewsAndReview from '../NewsAndReview.vue';
import { USER_GUIDE_SECTIONS } from '../../components/news/userGuideSections.js';
import { ADMIN_GUIDE_SECTIONS } from '../../components/news/adminGuideSections.js';
import { SECURITY_GUIDE_SECTIONS } from '../../components/news/securityGuideSections.js';

function jsonResponse(body) {
  return { ok: true, json: async () => body };
}

function mockApiByUserType(userType) {
  apiRequest.mockImplementation((url) => {
    if (url === '/users/me') return Promise.resolve(jsonResponse({ user_type: userType }));
    if (url === '/news') return Promise.resolve(jsonResponse([]));
    if (url === '/announcements/active') return Promise.resolve(jsonResponse(null));
    return Promise.resolve(jsonResponse({}));
  });
}

async function mountView() {
  const wrapper = shallowMount(NewsAndReview);
  await flushPromises();
  return wrapper;
}

describe('NewsAndReview - выбор руководства по роли', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    hasPermission.mockReset();
  });

  it('охранник видит руководство охранника', async () => {
    hasPermission.mockReturnValue(false);
    mockApiByUserType('security');
    const wrapper = await mountView();
    expect(wrapper.vm.guideTitle).toBe('Руководство охранника');
    expect(wrapper.vm.guideSections).toBe(SECURITY_GUIDE_SECTIONS);
  });

  it('обычный пользователь видит пользовательское руководство', async () => {
    hasPermission.mockReturnValue(false);
    mockApiByUserType('user');
    const wrapper = await mountView();
    expect(wrapper.vm.guideTitle).toBe('Руководство пользователя');
    expect(wrapper.vm.guideSections).toBe(USER_GUIDE_SECTIONS);
  });

  it('админ видит руководство администратора независимо от типа', async () => {
    hasPermission.mockReturnValue(true);
    mockApiByUserType('security');
    const wrapper = await mountView();
    expect(wrapper.vm.guideTitle).toBe('Руководство администратора');
    expect(wrapper.vm.guideSections).toBe(ADMIN_GUIDE_SECTIONS);
  });
});
