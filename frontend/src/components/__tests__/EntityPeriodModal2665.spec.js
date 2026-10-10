import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import EntityPeriodModal from '../EntityPeriodModal.vue';
import DateRangeSection from '../CreateApplication/DateRangeSection.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const revision = 'a'.repeat(64);
const window = {
  entry_date_from: '2035-10-08', entry_date_to: '2035-10-09',
  entry_time_from: '08:00:00', entry_time_to: '22:00:00',
};
const snapshot = (overrides = {}) => ({
  entity_id: 17, attachment_id: 31, application_id: 9,
  period_mode: 'inherit', individual_period: null, source_period: window,
  effective_period: { ...window, bounded: true, source: 'attachment' },
  period_revision: revision, approvals_reset: false, ...overrides,
});
// api/client returns a Response whose json() has already unwrapped envelope.data.
const response = (data, status = 200) => ({ ok: status >= 200 && status < 300, status, json: async () => data });
const wrappers = [];
function modal(props = {}) {
  const wrapper = mount(EntityPeriodModal, {
    props: { show: true, kind: 'employee', entityID: 17, title: 'Тестовая запись', ...props },
    global: { stubs: { Teleport: true } },
  });
  wrappers.push(wrapper);
  return wrapper;
}
async function selectIndividual(wrapper) {
  await wrapper.find('input[value="individual"]').setValue();
  await wrapper.find('[data-testid="entity-period-reason"]').setValue('  Продление по обращению  ');
}

describe('EntityPeriodModal #2665', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2035-10-07T21:30:00Z'));
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue(response(snapshot()));
  });
  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount());
    vi.useRealTimers();
  });

  it('loads current effective dates and requires a reason', async () => {
    const wrapper = modal();
    await flushPromises();
    expect(apiRequest).toHaveBeenCalledExactlyOnceWith('/employees/17/period');
    expect(wrapper.find('[data-testid="entity-period-current"]').text()).toContain('08.10.2035 08:00 - 09.10.2035 22:00');
    expect(wrapper.find('[data-testid="entity-period-save"]').attributes('disabled')).toBeDefined();
    await wrapper.find('input[value="individual"]').setValue();
    const dates = wrapper.findComponent(DateRangeSection);
    expect(dates.props()).toMatchObject({ startDate: '08.10.2035', endDate: '09.10.2035', startTime: '08:00', endTime: '22:00' });
  });

  it('sends the selected row, table context, full Moscow window and current revision', async () => {
    const wrapper = modal({ kind: 'car', tableID: 5 });
    await flushPromises();
    expect(apiRequest).toHaveBeenNthCalledWith(1, '/cars/17/period?table_id=5');
    await selectIndividual(wrapper);
    wrapper.findComponent(DateRangeSection).vm.$emit('update:end-date', '10.10.2035');
    await wrapper.vm.$nextTick();
    const result = snapshot({ period_mode: 'individual', period_revision: 'b'.repeat(64) });
    apiRequest.mockResolvedValueOnce(response(result));
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    await flushPromises();
    const [path, options] = apiRequest.mock.calls[1];
    expect(path).toBe('/cars/17/period');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body)).toEqual({
      period_mode: 'individual', reason: 'Продление по обращению', expected_revision: revision, table_id: 5,
      period: { ...window, entry_date_to: '2035-10-10' },
    });
    expect(wrapper.emitted('changed')).toEqual([[result]]);
    expect(wrapper.emitted('close')).toHaveLength(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
  });

  it('explicit inheritance sends no own window and no absent table context', async () => {
    apiRequest.mockResolvedValueOnce(response(snapshot({ period_mode: 'individual', individual_period: window })));
    const wrapper = modal();
    await flushPromises();
    await wrapper.find('input[value="inherit"]').setValue();
    await wrapper.find('[data-testid="entity-period-reason"]').setValue('Вернуть срок вложения');
    apiRequest.mockResolvedValueOnce(response(snapshot()));
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    await flushPromises();
    expect(JSON.parse(apiRequest.mock.calls[1][1].body)).toEqual({
      period_mode: 'inherit', reason: 'Вернуть срок вложения', expected_revision: revision,
    });
  });

  it('shows a manual unbounded permit without inventing dates', async () => {
    apiRequest.mockResolvedValueOnce(response(snapshot({
      application_id: null,
      effective_period: { bounded: false, source: 'manual_unbounded', entry_date_from: null, entry_date_to: null, entry_time_from: null, entry_time_to: null },
    })));
    const wrapper = modal();
    await flushPromises();
    expect(wrapper.find('[data-testid="entity-period-current"]').text()).toContain('Бессрочно');
    expect(wrapper.text()).toContain('Срок ручного основания');
    await selectIndividual(wrapper);
    expect(wrapper.findComponent(DateRangeSection).props('startDate')).toBe('');
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    expect(apiRequest).toHaveBeenCalledTimes(1);
    expect(wrapper.findComponent(DateRangeSection).props('errors')).toMatchObject({ startDate: 'Укажите дату начала' });
  });

  it('keeps single-day quick-selection events and validates the entire interval', async () => {
    const wrapper = modal();
    await flushPromises();
    await selectIndividual(wrapper);
    const dates = wrapper.findComponent(DateRangeSection);
    dates.vm.$emit('update:is-one-day', true);
    dates.vm.$emit('update:single-date', '08.10.2035');
    dates.vm.$emit('update:start-time', '22:00');
    dates.vm.$emit('update:end-time', '08:00');
    await wrapper.vm.$nextTick();
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    expect(apiRequest).toHaveBeenCalledTimes(1);
    expect(dates.props('errors').endTime).toContain('позже');
  });

  it('uses Moscow now when refusing an expired window', async () => {
    const wrapper = modal();
    await flushPromises();
    await selectIndividual(wrapper);
    Object.assign(wrapper.vm.form, { isOneDay: true, singleDate: '08.10.2035', startTime: '00:00', endTime: '00:15' });
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    expect(apiRequest).toHaveBeenCalledTimes(1);
    expect(wrapper.findComponent(DateRangeSection).props('errors').endDate).toContain('истёк');
  });

  it('409 requires an explicit refresh and never silently retries the write', async () => {
    const wrapper = modal();
    await flushPromises();
    await selectIndividual(wrapper);
    apiRequest.mockResolvedValueOnce(response({ message: 'stale' }, 409));
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    await flushPromises();
    expect(wrapper.find('[role="alert"]').text()).toContain('проверьте новые значения');
    expect(wrapper.find('[data-testid="entity-period-save"]').attributes('disabled')).toBeDefined();
    expect(wrapper.emitted('changed')).toBeUndefined();
    await wrapper.vm.save();
    expect(apiRequest).toHaveBeenCalledTimes(2);
    const fresh = snapshot({ period_revision: 'c'.repeat(64), effective_period: { ...window, entry_date_to: '2035-10-12', bounded: true, source: 'attachment' } });
    apiRequest.mockResolvedValueOnce(response(fresh));
    await wrapper.find('[data-testid="entity-period-refresh"]').trigger('click');
    await flushPromises();
    expect(apiRequest).toHaveBeenCalledTimes(3);
    expect(wrapper.vm.snapshot.period_revision).toBe(fresh.period_revision);
    expect(wrapper.vm.form.endDate).toBe('12.10.2035');
    expect(wrapper.vm.conflict).toBe(false);
    expect(wrapper.emitted('changed')).toBeUndefined();
  });

  it.each([403, 500])('keeps the dialog open after HTTP %s with no success event', async status => {
    const wrapper = modal();
    await flushPromises();
    await selectIndividual(wrapper);
    apiRequest.mockResolvedValueOnce(response({ message: 'Сохранение недоступно' }, status));
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain(status >= 500 ? 'Не удалось изменить срок' : 'Сохранение недоступно');
    if (status >= 500) expect(wrapper.text()).not.toContain('Сохранение недоступно');
    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('does not accept a stale inspection after changing the selected entity', async () => {
    let resolveOld;
    apiRequest.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve; }));
    const wrapper = modal();
    apiRequest.mockResolvedValueOnce(response(snapshot({ entity_id: 18 })));
    await wrapper.setProps({ entityID: 18 });
    await flushPromises();
    resolveOld(response(snapshot()));
    await flushPromises();
    expect(wrapper.vm.snapshot.entity_id).toBe(18);
  });

  it('prevents duplicate writes and closing while a save is pending', async () => {
    const wrapper = modal();
    await flushPromises();
    await selectIndividual(wrapper);
    let resolveSave;
    apiRequest.mockReturnValueOnce(new Promise(resolve => { resolveSave = resolve; }));
    await wrapper.find('[data-testid="entity-period-save"]').trigger('click');
    await wrapper.vm.save();
    wrapper.vm.close();
    expect(apiRequest).toHaveBeenCalledTimes(2);
    expect(wrapper.emitted('close')).toBeUndefined();
    resolveSave(response(snapshot({ period_mode: 'individual' })));
    await flushPromises();
    expect(wrapper.emitted('changed')).toHaveLength(1);
  });

  it('a denied inspection cannot enable saving', async () => {
    apiRequest.mockResolvedValueOnce(response({ message: 'Недостаточно прав' }, 403));
    const wrapper = modal();
    await flushPromises();
    expect(wrapper.text()).toContain('Недостаточно прав');
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.find('[data-testid="entity-period-save"]').attributes('disabled')).toBeDefined();
    expect(wrapper.emitted('changed')).toBeUndefined();
  });

  it('blocks incomplete inspection data and invalid entity identifiers', async () => {
    apiRequest.mockResolvedValueOnce(response(snapshot({ period_revision: '' })));
    const wrapper = modal();
    await flushPromises();
    expect(wrapper.vm.snapshot).toBeNull();
    expect(wrapper.text()).toContain('текущую версию');
    await wrapper.setProps({ entityID: 0 });
    await flushPromises();
    expect(apiRequest).toHaveBeenCalledTimes(1);
    expect(wrapper.find('[data-testid="entity-period-save"]').attributes('disabled')).toBeDefined();
  });
});
