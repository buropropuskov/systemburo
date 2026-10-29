import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useOnboardingStore } from '../onboarding';
import { useAuthStore } from '../auth';
import { usePermissionsStore } from '../permissions';
import { interceptRead } from '@/api/readInterceptor';
import { markOnboardingComplete } from '@/api/onboarding';
vi.mock('@/api/onboarding', () => ({ getOnboardingStatus: vi.fn(async()=>({completed:{}})), markOnboardingComplete: vi.fn(), getSecurityPostContext: vi.fn(async()=>null) }));
vi.mock('@/api/approvers', () => ({ getMyApprovalRole: vi.fn(async()=>({is_approver:true,is_reviewer:true})) }));
vi.mock('@/api/applications', () => ({ getUserApplicationsPaginated: vi.fn(async()=>({items:[],meta:{total:0}})) }));
vi.mock('@/api/auth', async original=>({...await original(),getMe:vi.fn(async()=>({user_type_code:'user'}))}));
const token=id=>`${btoa('{}')}.${btoa(JSON.stringify({username:'fixture',user_id:id,exp:9999999999}))}.signature`;
let store,auth;
beforeEach(()=>{localStorage.clear();vi.clearAllMocks();setActivePinia(createPinia());auth=useAuthStore();auth.setTokens(token(42));auth.userTypeCode='user';const p=usePermissionsStore();p.loaded=true;p.mode='super';store=useOnboardingStore();});
describe('real store connects role-specific data lifetime',()=>{
  it('does not start a center demo for an unknown actor',async()=>{
    store.reset();auth.setTokens(`${btoa('{}')}.${btoa(JSON.stringify({username:'fixture',exp:9999999999}))}.signature`);auth.userTypeCode='user';
    await store.prepareTour('accept');expect(store.start({tour:'accept',manual:true})).toBe(false);
    expect(interceptRead('/applications')).toBe(null);expect(store.isActive).toBe(false);
  });
  it('direct supplement section, Back and Jump keep semantic IDs and total',async()=>{
    expect((await store.prepareTour('accept')).status).toBe('ready');
    expect(store.start({tour:'accept',manual:true,fromStepId:'acc-detail-supplement'})).toBe(true);
    const total=store.totalSteps,id=store.currentStepId;
    expect(store.currentStep.demoScene).toBe('accept-supplement-approved');
    await store.prepareCenterDemo(store.currentStep.demoScene,{isCurrent:()=>true});
    expect((await interceptRead('/applications/999000002/details').json()).data.status).toBe('В работе');
    const prior=store.steps.findIndex(step=>step.id==='acc-detail-take');store.setIndex(prior);
    await store.prepareCenterDemo(store.currentStep.demoScene,{isCurrent:()=>true});
    expect((await interceptRead('/applications/999000002/details').json()).data.status).toBe('Непрочитано');
    store.setIndex(store.steps.findIndex(step=>step.id===id));await store.prepareCenterDemo(store.currentStep.demoScene,{isCurrent:()=>true});
    expect(store.totalSteps).toBe(total);expect(store.currentStepId).toBe(id);expect(markOnboardingComplete).not.toHaveBeenCalled();store.stop();
  });
  it('reviewer scene assigns only its own fictional vote, actor switch removes all interception',async()=>{
    await store.prepareTour('approve');expect(store.start({tour:'approve',manual:true})).toBe(true);
    const response=interceptRead('/applications/999000002/details'),data=(await response.json()).data;
    expect(data.responsible_users.find(user=>user.id===42).approval_status).toBe('pending');
    const held=interceptRead('/applications/999000002/details');auth.setTokens(token(43));
    expect(store.isActive).toBe(false);expect(interceptRead('/applications')).toBe(null);
    await expect(held.json()).rejects.toMatchObject({name:'AbortError'});expect(markOnboardingComplete).not.toHaveBeenCalled();
  });
});
