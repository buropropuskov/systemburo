import { describe, it, expect, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import PushAdoptionSummary from '../PushAdoptionSummary.vue';
import AnalyticsDonutChart from '../AnalyticsDonutChart.vue';

const getPushSummary = vi.hoisted(() => vi.fn());
vi.mock('@/api/webPush', () => ({ getPushSummary }));

async function render(data) {
  getPushSummary.mockResolvedValue(data);
  const wrapper = mount(PushAdoptionSummary, { global: { stubs: { AnalyticsDonutChart: true } } });
  await flushPromises();
  return wrapper;
}

describe('PushAdoptionSummary: разные основания расчёта', () => {
  it('подписка iOS остаётся при последнем входе с компьютера: 1 подписка, 1 пользователь iOS, 7% пользователей', async () => {
    const wrapper = await render({ active_users_total: 15, users_with_push: 1,
      subscriptions_by_platform: { ios: 1, android: 0, desktop: 0, unknown: 0 },
      users_with_push_by_platform: { ios: 1, android: 0, desktop: 0, unknown: 0 },
      users_by_last_login_platform: { ios: 0, android: 0, desktop: 15, unknown: 0 } });
    expect(wrapper.findComponent(AnalyticsDonutChart).props('data')).toEqual([{ label: 'iOS (iPhone, iPad)', value: 1 }]);
    expect(wrapper.find('[data-testid="push-adoption-rate"]').text()).toContain('7%');
    const ios = wrapper.findAll('.push-summary__tile').find(t => t.text().includes('iPhone/iPad'));
    expect(ios.find('.push-summary__tile-val').text()).toBe('1');
    wrapper.unmount();
  });

  it('явный null не превращается в ноль', async () => {
    const wrapper = await render({ active_users_total: null, users_with_push: null, users_by_last_login_platform: { ios: null } });
    const values = wrapper.findAll('.push-summary__tile-val').map(v => v.text());
    expect(values).toEqual(['—', '—', '—', '—']);
    wrapper.unmount();
  });

  it('нет данных не означает, что подписок достоверно нет', async () => {
    const wrapper = await render({});
    expect(wrapper.find('.push-summary__chart-empty').text()).toBe('Нет данных о push-подписках');
    wrapper.unmount();
  });
});
