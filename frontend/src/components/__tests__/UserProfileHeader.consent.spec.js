import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Бейдж согласия в шапке кабинета - единственное место, где работник может
// отозвать своё согласие сам (#1567).

const listMyConsents = vi.fn();
const revokeMyConsent = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  listMyConsents: (...a) => listMyConsents(...a),
  revokeMyConsent: (...a) => revokeMyConsent(...a),
}));

const refresh = vi.fn();
vi.mock('@/stores/pdConsent', () => ({
  usePDConsentStore: () => ({ refresh }),
}));

import UserProfileHeader from '../UserProfileHeader.vue';
import { useUiStore } from '@/stores/ui';
import { useDeletionsStore } from '@/stores/deletions';

const consent = (over = {}) => ({
  id: 1,
  consent_type: 'pd_processing',
  granted: true,
  granted_at: '2026-07-12T08:30:00Z',
  revoked_at: null,
  document_version: 17,
  ...over,
});

function mountHeader() {
  return mount(UserProfileHeader, {
    props: { lastName: 'Иванов', firstName: 'Иван', email: 'i@example.com' },
  });
}

const badge = (w) => w.find('[data-testid="cabinet-consent-badge"]');

describe('UserProfileHeader — согласие на обработку данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    listMyConsents.mockResolvedValue([consent()]);
    revokeMyConsent.mockResolvedValue({});
  });

  it('показывает бейдж, когда согласие дано', async () => {
    const wrapper = mountHeader();
    await flushPromises();

    expect(badge(wrapper).exists()).toBe(true);
    expect(badge(wrapper).text()).toContain('Согласие на обработку данных');
    expect(badge(wrapper).attributes('title')).toContain('12.07.2026');

    wrapper.unmount();
  });

  it('бейдж стоит над именем, а не в ряду контактов', async () => {
    const wrapper = mountHeader();
    await flushPromises();

    expect(badge(wrapper).element.closest('.user-details-row')).toBe(null);
    const mainInfo = wrapper.find('.main-info').element;
    const consentRow = wrapper.find('.consent-row').element;
    const nameRow = wrapper.find('.name-and-type').element;
    expect(consentRow.parentElement).toBe(mainInfo);
    expect(consentRow.compareDocumentPosition(nameRow) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    wrapper.unmount();
  });

  it('без действующего согласия бейджа нет - отзывать нечего', async () => {
    listMyConsents.mockResolvedValue([consent({ revoked_at: '2026-07-20T10:00:00Z', granted: false })]);
    const wrapper = mountHeader();
    await flushPromises();

    expect(badge(wrapper).exists()).toBe(false);

    wrapper.unmount();
  });

  it('согласие другого вида бейдж не показывает', async () => {
    listMyConsents.mockResolvedValue([consent({ consent_type: 'pd_transfer' })]);
    const wrapper = mountHeader();
    await flushPromises();

    expect(badge(wrapper).exists()).toBe(false);

    wrapper.unmount();
  });

  it('клик спрашивает подтверждение, отзывает и поднимает окно согласия', async () => {
    const wrapper = mountHeader();
    await flushPromises();
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await badge(wrapper).trigger('click');
    await flushPromises();

    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ confirmText: 'Отозвать' }));
    expect(revokeMyConsent).toHaveBeenCalledWith('pd_processing');
    expect(notify).toHaveBeenCalled();
    // Без принудительного перечитывания окно согласия появилось бы только после F5.
    expect(refresh).toHaveBeenCalledWith(true);
    expect(badge(wrapper).exists()).toBe(false);

    wrapper.unmount();
  });

  it('отказ в подтверждении ничего не отзывает', async () => {
    const wrapper = mountHeader();
    await flushPromises();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);

    await badge(wrapper).trigger('click');
    await flushPromises();

    expect(revokeMyConsent).not.toHaveBeenCalled();
    expect(badge(wrapper).exists()).toBe(true);

    wrapper.unmount();
  });

  it('ошибка отзыва сообщается, бейдж остаётся', async () => {
    revokeMyConsent.mockRejectedValue(new Error('нет связи'));
    const wrapper = mountHeader();
    await flushPromises();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await badge(wrapper).trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(badge(wrapper).exists()).toBe(true);

    wrapper.unmount();
  });

  it('недоступные сведения о согласии страницу не ломают', async () => {
    listMyConsents.mockRejectedValue(new Error('403'));
    const wrapper = mountHeader();
    await flushPromises();

    expect(badge(wrapper).exists()).toBe(false);
    expect(wrapper.text()).toContain('Иванов');

    wrapper.unmount();
  });
});
