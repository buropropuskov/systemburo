import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { shallowMount } from '@vue/test-utils';
import ActionBar from '@/components/ApplicationDetail/ApplicationActionBar.vue';
import { apiRequest } from '@/api/client';
import { createCenterDemoResponder, DEMO_ACTION_MESSAGE } from '../centerDemoResponder';
import { buildDemoCenterScenario } from '../demoCenterScenarios';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const build=scene=>buildDemoCenterScenario({scene,actorId:42,now:Date.parse('2026-10-28T12:00:00Z')});
beforeEach(()=>{setActivePinia(createPinia());vi.clearAllMocks();});
function mount(scene,{acceptor=false,allow=true}={}){const data=build(scene);return shallowMount(ActionBar,{props:{application:data.application,responsibleUsers:data.responsibleUsers,currentUserId:42,isApprover:acceptor,approvalAllowed:allow,supplements:data.supplements,hasUnoverriddenBlacklistFlags:data.application.has_unoverridden_blacklist_flags},global:{stubs:{BaseModal:true,Badge:true}}});}
describe('actual action bar with role-scoped scenes',()=>{
  it('reviewer sees own vote; explicit denial removes it, membership remains real',async()=>{
    const wrapper=mount('reviewer-pending');expect(wrapper.vm.isResponsibleUser).toBe(true);expect(wrapper.vm.showsApproveButton).toBe(true);
    expect(wrapper.vm.isApproverUser).toBe(false);await wrapper.setProps({approvalAllowed:false});expect(wrapper.vm.showsApproveButton).toBe(false);wrapper.unmount();
  });
  it('acceptor and combined roles retain distinct scene actions',()=>{
    const accept=mount('accept-ready',{acceptor:true});expect(accept.vm.isResponsibleUser).toBe(false);expect(accept.vm.canTakeToWork).toBe(true);expect(accept.text()).toContain('Принять');accept.unmount();
    const both=mount('reviewer-pending',{acceptor:true});expect(both.vm.showsApproveButton).toBe(true);expect(both.vm.approvingCompletesConfirmation).toBe(false);expect(both.vm.canTakeToWork).toBe(false);both.unmount();
    const supplement=mount('accept-supplement-approved',{acceptor:true});expect(supplement.vm.canDecideSupplement).toBe(true);expect(supplement.text()).toContain('Принять доп. №1');supplement.unmount();
  });
  it.each(['acceptApplication','rejectApplication'])('%s reports controlled local rejection without raw envelope or false success',async method=>{
    const data=build('accept-ready'),respond=createCenterDemoResponder(data);
    apiRequest.mockImplementation((path,options)=>Promise.resolve(respond(path,options.method)));
    const wrapper=mount('accept-ready',{acceptor:true});await wrapper.vm[method]();
    const event=wrapper.emitted('action-completed').at(-1)[0];expect(event.success).toBe(false);expect(event.message).toContain(DEMO_ACTION_MESSAGE);expect(event.message).not.toContain('success');wrapper.unmount();
  });
  it('reviewer refusal stays a human error and never records a successful vote',async()=>{
    apiRequest.mockResolvedValue(new Response(JSON.stringify({success:false,error:DEMO_ACTION_MESSAGE}),{status:409}));
    const wrapper=mount('reviewer-pending');await wrapper.vm.updateConfirmation('Согласовано');
    expect(wrapper.emitted('action-completed').at(-1)[0]).toMatchObject({success:false,message:expect.stringContaining(DEMO_ACTION_MESSAGE)});wrapper.unmount();
  });
  it('combined-role rejection of its vote never proceeds to a take-to-work write',async()=>{
    const data=build('reviewer-pending'),respond=createCenterDemoResponder(data);
    apiRequest.mockImplementation((path,options)=>Promise.resolve(respond(path,options.method)));
    const wrapper=mount('reviewer-pending',{acceptor:true});await wrapper.vm.handleCombinedAction('accept');
    expect(apiRequest).toHaveBeenCalledOnce();expect(apiRequest.mock.calls[0][0]).toMatch(/\/approve$/);
    const event=wrapper.emitted('action-completed').at(-1)[0];expect(event.success).toBe(false);expect(event.message).toContain(DEMO_ACTION_MESSAGE);expect(event.message).not.toContain('success');wrapper.unmount();
  });
  it('preserves a real human error through the same parser',async()=>{
    apiRequest.mockResolvedValue(new Response('Заявка уже принята другим сотрудником',{status:409}));
    const wrapper=mount('accept-ready',{acceptor:true});await wrapper.vm.acceptApplication();expect(wrapper.emitted('action-completed').at(-1)[0].message).toContain('Заявка уже принята другим сотрудником');wrapper.unmount();
  });
});
