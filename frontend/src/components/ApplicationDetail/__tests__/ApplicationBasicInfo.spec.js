import { afterEach, describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import ApplicationBasicInfo from '../ApplicationBasicInfo.vue';
const wrappers=[];
function render(application){const w=mount(ApplicationBasicInfo,{props:{application},attachTo:document.body});wrappers.push(w);return w;}
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.replaceChildren();});
describe('sender permitted display labels',()=>{
  it('shows the server short label and uses the previously visible full label in a focusable hint',async()=>{
    const w=render({sender_name:'Иванов И.И.',sender_full_name:'Иванов Иван Иванович'});
    expect(w.text()).toContain('Иванов И.И.');expect(w.text()).not.toContain('Иванов Иван Иванович');
    const anchor=w.find('.sender-name');expect(anchor.attributes('aria-label')).toBe('Иванов Иван Иванович');
    await anchor.trigger('focus');expect(document.querySelector('[role=tooltip]')?.textContent).toContain('Иванов Иван Иванович');
  });
  it('retains the server mask in both the short label and tooltip without reconstructing another identity',async()=>{
    const w=render({sender_name:'Оператор бюро',sender_full_name:'Оператор бюро',sender_user_id:99});
    expect(w.find('.sender-name').text()).toBe('Оператор бюро');
    await w.find('.sender-name').trigger('focus');
    expect(document.querySelector('[role=tooltip]')?.textContent.trim()).toBe('Оператор бюро');
  });
  it('does not invent a full name when the server only permits a login',async()=>{
    const w=render({sender_name:'@participant',sender_full_name:null});
    await w.find('.sender-name').trigger('focus');
    expect(document.querySelector('[role=tooltip]')?.textContent.trim()).toBe('@participant');
  });
});
