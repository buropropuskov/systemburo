import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import DateRangeSection from '../DateRangeSection.vue';

afterEach(() => vi.useRealTimers());
describe('history calendar #2676', () => {
  it('preserves an explicit past date on blur without advancing its year', async () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2035-10-11T09:00:00Z'));
    const wrapper = mount(DateRangeSection, { props: { singleMoment: true, isOneDay: true, singleDate: '10.10.2035', startTime: '12:30', fieldConfig: { roof_access: { visible:false }, free_parking: { visible:false } } } });
    expect(wrapper.vm.isPastDate('10.10.2035')).toBe(false);
    expect(wrapper.vm.autoCompleteDate('10.10.2035')).toBe('10.10.2035');
    await wrapper.find('input[placeholder="дд.мм.гггг"]').trigger('blur');
    expect(wrapper.emitted('update:single-date')).toBeUndefined();
    expect(wrapper.findAll('input[placeholder="чч:мм"]')).toHaveLength(1);
    wrapper.unmount();
  });
  it('retains the future-date restriction for ordinary permits', () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2035-10-11T09:00:00Z'));
    const wrapper = mount(DateRangeSection, { props:{ isOneDay:true } });
    expect(wrapper.vm.isPastDate('10.10.2035')).toBe(true);
    expect(wrapper.vm.autoCompleteDate('10.10.2035')).toBe('10.10.2036');
    wrapper.unmount();
  });
});
