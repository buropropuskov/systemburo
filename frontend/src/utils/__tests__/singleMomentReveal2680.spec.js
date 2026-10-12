import { afterEach, describe, expect, it, vi } from 'vitest';
import { prepareSingleMomentCalendar } from '../singleMomentCalendar';
afterEach(() => { document.body.innerHTML = ''; vi.restoreAllMocks(); });
function setup() {
  const popup = document.createElement('div'); popup.className = 'datepicker'; document.body.append(popup);
  popup.getBoundingClientRect = () => ({ width: 260, height: 333 });
  const anchor = { getBoundingClientRect: () => ({ left: 20, top: window.innerHeight - 50, bottom: window.innerHeight - 10 }) };
  let reveal;
  const vm = { showSingleDatepicker: true, datepickerStyle: {}, $nextTick(callback) { reveal = callback; } };
  return { popup, anchor, vm, reveal: () => reveal() };
}
describe('compact calendar first visible frame', () => {
  it('is hidden during measurement and appears within the viewport at its lower edge', () => {
    const { anchor, vm, reveal } = setup();
    prepareSingleMomentCalendar(vm, anchor, 1);
    expect(vm.datepickerStyle.visibility).toBe('hidden');
    reveal();
    expect(vm.datepickerStyle.visibility).toBe('visible');
    expect(parseFloat(vm.datepickerStyle.top)).toBeGreaterThanOrEqual(8);
    expect(parseFloat(vm.datepickerStyle.top) + 333).toBeLessThanOrEqual(window.innerHeight - 8);
  });
  it('cannot reveal a calendar closed before measurement finishes', () => {
    const { anchor, vm, reveal } = setup();
    prepareSingleMomentCalendar(vm, anchor, 1);
    vm.showSingleDatepicker = false; reveal();
    expect(vm.datepickerStyle.visibility).toBe('hidden');
  });
});
