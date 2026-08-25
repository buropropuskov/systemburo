import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const getPushSummary = vi.fn()
vi.mock('@/api/webPush', () => ({
  getPushSummary: (...args) => getPushSummary(...args),
}))

import PushAdoptionSummary from '../PushAdoptionSummary.vue'
import AnalyticsDonutChart from '../AnalyticsDonutChart.vue'

const mountBlock = () => mount(PushAdoptionSummary, {
  global: { stubs: { AnalyticsDonutChart: true } },
})

const tileVal = (wrapper, label) => {
  const tile = wrapper.findAll('.push-summary__tile').find((t) => t.text().includes(label));
  return tile?.find('.push-summary__tile-val').text();
};

// Форма ответа - models.PushSummary (internal/models/push_subscription.go).
function fixture(overrides = {}) {
  return {
    active_users_total: 40,
    users_with_push: 12,
    users_without_push: 28,
    subscriptions_by_platform: { ios: 8, android: 4, desktop: 0, unknown: 0 },
    users_by_last_login_platform: { ios: 25, android: 10, desktop: 5, unknown: 0 },
    ...overrides,
  };
}

beforeEach(() => {
  getPushSummary.mockReset()
})

describe('PushAdoptionSummary', () => {
  it('рисует переданные числа по плиткам', async () => {
    getPushSummary.mockResolvedValue(fixture())
    const wrapper = mountBlock()
    await flushPromises()

    expect(tileVal(wrapper, 'Активные пользователи')).toBe('40')
    expect(tileVal(wrapper, 'С push-подпиской')).toBe('12')
    expect(tileVal(wrapper, 'Заходят с iOS')).toBe('25')
  })

  it('доля считается от активных пользователей, а не от подписок', async () => {
    // 12 из 40 активных -> 30%. Если бы делили от подписок (12/12), было бы 100%.
    getPushSummary.mockResolvedValue(fixture({ active_users_total: 40, users_with_push: 12 }))
    const wrapper = mountBlock()
    await flushPromises()

    const rate = wrapper.find('[data-testid="push-adoption-rate"]')
    expect(rate.text()).toContain('30%')
  })

  it('пустой ответ не роняет компонент - прочерки, а не 0 или ошибка', async () => {
    getPushSummary.mockResolvedValue({})
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="push-adoption-summary"]').exists()).toBe(true)
    expect(tileVal(wrapper, 'Активные пользователи')).toBe('—')
    expect(tileVal(wrapper, 'С push-подпиской')).toBe('—')
    expect(tileVal(wrapper, 'Заходят с iOS')).toBe('—')
    expect(wrapper.find('[data-testid="push-adoption-rate"]').text()).toContain('—')
  })

  it('сбой запроса не роняет компонент - тот же прочерк, без исключения наружу', async () => {
    getPushSummary.mockRejectedValue(new Error('net'))
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="push-adoption-summary"]').exists()).toBe(true)
    expect(tileVal(wrapper, 'Активные пользователи')).toBe('—')
  })

  it('ноль активных пользователей не даёт NaN/Infinity в доле', async () => {
    getPushSummary.mockResolvedValue(fixture({
      active_users_total: 0,
      users_with_push: 0,
      subscriptions_by_platform: { ios: 0, android: 0, desktop: 0, unknown: 0 },
    }))
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="push-adoption-rate"]').text()).toContain('—')
  })

  it('разрез подписок по платформам передаётся в донат-чарт с русской подписью iOS (iPhone, iPad)', async () => {
    getPushSummary.mockResolvedValue(fixture({
      subscriptions_by_platform: { ios: 3, android: 2, desktop: 0, unknown: 0 },
    }))
    const wrapper = mountBlock()
    await flushPromises()

    const chart = wrapper.findComponent(AnalyticsDonutChart)
    expect(chart.props('data')).toEqual([
      { label: 'iOS (iPhone, iPad)', value: 3 },
      { label: 'Android', value: 2 },
    ])
  })

  it('нулевые сегменты (desktop/unknown = 0) не попадают в донат-чарт', async () => {
    getPushSummary.mockResolvedValue(fixture({
      subscriptions_by_platform: { ios: 3, android: 0, desktop: 0, unknown: 0 },
    }))
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.findComponent(AnalyticsDonutChart).props('data')).toEqual([
      { label: 'iOS (iPhone, iPad)', value: 3 },
    ])
  })

  it('без единой подписки донат-чарт не рисуется, показана пустая подпись', async () => {
    getPushSummary.mockResolvedValue(fixture({
      users_with_push: 0,
      subscriptions_by_platform: { ios: 0, android: 0, desktop: 0, unknown: 0 },
    }))
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.findComponent(AnalyticsDonutChart).exists()).toBe(false)
    expect(wrapper.find('.push-summary__chart-empty').exists()).toBe(true)
  })

  it('defineExpose(refresh) перезагружает данные повторным запросом', async () => {
    // Отображаемое число проверяют другие тесты (плитки/прочерк) - здесь важен
    // сам факт повторной загрузки: значение внутри AnimatedNumber доезжает до
    // цели через requestAnimationFrame, а не синхронно на следующем тике.
    getPushSummary.mockResolvedValue(fixture({ active_users_total: 1, users_with_push: 1 }))
    const wrapper = mountBlock()
    await flushPromises()
    expect(getPushSummary).toHaveBeenCalledTimes(1)

    getPushSummary.mockResolvedValue(fixture({ active_users_total: 2, users_with_push: 2 }))
    await wrapper.vm.refresh()
    await flushPromises()

    expect(getPushSummary).toHaveBeenCalledTimes(2)
  })
})
