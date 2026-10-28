import { afterEach, describe, expect, it } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import OnboardingStartDialog from '../OnboardingStartDialog.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
let wrapper;
afterEach(()=>{wrapper?.unmount();document.body.replaceChildren();});
function render(){wrapper=mount(OnboardingStartDialog,{props:{show:true,chapters:[{id:'cars-table',name:'Автомобили'},{id:'employees-table',name:'Сотрудники'}]}});return wrapper;}
describe('sequential learning entry',()=>{
 it('uses semantic IDs and keeps the section action disabled until an available chapter is selected',async()=>{
  render();expect(document.querySelector('[data-testid="ob-start-section"]').disabled).toBe(true);
  wrapper.findComponent(BaseDropdown).vm.$emit('update:modelValue','employees-table');await flushPromises();
  document.querySelector('[data-testid="ob-start-section"]').click();
  expect(wrapper.emitted('start')).toEqual([[{fromStepId:'employees-table'}]]);
 });
 it('does not use a removed chapter or invent an index fallback',async()=>{
  render();wrapper.findComponent(BaseDropdown).vm.$emit('update:modelValue','cars-table');await flushPromises();
  await wrapper.setProps({chapters:[{id:'employees-table',name:'Сотрудники'}]});
  expect(document.querySelector('[data-testid="ob-start-section"]').disabled).toBe(true);
  expect(wrapper.emitted('start')).toBeUndefined();
 });
 it('offers full restart and an explicit independent resume without writing completion',()=>{
  render();document.querySelector('[data-testid="ob-start-full"]').click();
  expect(wrapper.emitted('start')).toEqual([[{}]]);expect(wrapper.emitted('resume')).toBeUndefined();
 });
 it('shows resume only when the current store progress permits it',async()=>{
  render();expect(document.querySelector('[data-testid="ob-start-resume"]')).toBe(null);
  await wrapper.setProps({canResume:true});document.querySelector('[data-testid="ob-start-resume"]').click();
  expect(wrapper.emitted('resume')).toHaveLength(1);
 });
});
