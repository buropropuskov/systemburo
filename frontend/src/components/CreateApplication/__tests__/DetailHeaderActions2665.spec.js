import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import DetailHeaderActions, { detailHeaderTitle } from '../DetailHeaderActions.vue';
import EntityPeriodEditor from '@/components/EntityPeriodEditor.vue';
import ManualAttachCardAction from '@/components/ManualAttachCardAction.vue';
import { usePermissionsStore } from '@/stores/permissions';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const wrappers = [];
function actions(props = {}) {
  const wrapper = mount(DetailHeaderActions, { props: { show: true, kind: 'employee', entity: { id: 17 }, source: 'application', ...props } });
  wrappers.push(wrapper);
  return wrapper;
}

describe('DetailHeaderActions extraction #2665', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    usePermissionsStore().$patch({ mode: 'normal', effective: { 'detail.period.change': { value: 'allow' } } });
  });
  afterEach(() => wrappers.splice(0).forEach(wrapper => wrapper.unmount()));

  it('keeps existing action classes, labels and click events', async () => {
    const wrapper = actions({ historyVisible: true, applicationVisible: true, blacklistVisible: true });
    for (const [css, event, label] of [['history-btn', 'history', 'Полная история'], ['application-btn', 'application', 'Открыть заявку'], ['blacklist-add-btn', 'blacklist', 'В ЧС']]) {
      expect(wrapper.find(`.${css}`).text()).toBe(label);
      await wrapper.find(`.${css}`).trigger('click');
      expect(wrapper.emitted(event)).toEqual([[]]);
    }
  });
  it('does not manufacture contextual history/application/blacklist permissions', () => {
    const wrapper = actions();
    expect(wrapper.find('.history-btn').exists()).toBe(false);
    expect(wrapper.find('.application-btn').exists()).toBe(false);
    expect(wrapper.find('.blacklist-add-btn').exists()).toBe(false);
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(true);
  });
  it('forwards exact period identity, table context and the unchanged result event', async () => {
    const entity = { id: 999, activeCarId: 18 };
    const wrapper = actions({ kind: 'car', entity, source: 'carsview', periodTableId: 5 });
    const period = wrapper.findComponent(EntityPeriodEditor);
    expect(period.props()).toMatchObject({ show: true, kind: 'car', entity, source: 'carsview', readonly: false, tableId: 5 });
    const result = { entity_id: 18, period_revision: 'a'.repeat(64) };
    period.vm.$emit('changed', result);
    expect(wrapper.emitted('period-changed')).toEqual([[result]]);
    await wrapper.setProps({ readonly: true });
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
  });
  it('retains registry identity safety and live managed permission revocation', async () => {
    const wrapper = actions({ source: 'employeesview', entity: { id: 999 } });
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
    await wrapper.setProps({ entity: { id: 999, activeEmployeeId: 17 } });
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(true);
    usePermissionsStore().effective = {};
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="entity-period-open"]').exists()).toBe(false);
  });
  it('keeps the vehicle empty-container gate and narrow titles', () => {
    expect(actions({ visible: false }).find('.header-actions').exists()).toBe(false);
    for (const kind of ['employee', 'car']) {
      for (const count of [0, 1, 2]) expect(detailHeaderTitle(kind, true, count)).toBe('Информация');
      expect(detailHeaderTitle(kind, false, 1)).toBe('Детальная информация');
      expect(detailHeaderTitle(kind, false, 2)).toBe('Информация');
    }
    expect(detailHeaderTitle('employee', false, 0)).toBe('Детальная информация о сотруднике');
    expect(detailHeaderTitle('car', false, 0)).toBe('Детальная информация о Т/С');
  });
  it('passes only actual table identity to manual attach and forwards its result separately', async () => {
    const wrapper = actions({ source: 'peopletable', periodTableId: 8 });
    const attach = wrapper.findComponent(ManualAttachCardAction);
    expect(attach.props()).toMatchObject({ kind: 'employee', entityID: 17, tableID: 8 });
    const result = { entity_id: 17, entity_kind: 'employee', application_id: 5, destination_attachment_id: 23 };
    attach.vm.$emit('changed', result);
    expect(wrapper.emitted('manual-attached')).toEqual([[result]]);
    expect(wrapper.emitted('period-changed')).toBeUndefined();
    await wrapper.setProps({ source: 'employeesview', entity: { id: 999, activeEmployeeId: 17 } });
    expect(attach.props('entityID')).toBeNull();
    await wrapper.setProps({ source: 'peopletable', entity: { id: 17, isDraft: true } });
    expect(attach.props('entityID')).toBeNull();
  });
});
