import { afterEach, describe, expect, it, vi } from 'vitest';
import { effectScope, nextTick, reactive, toRaw } from 'vue';
import { useCenterTourReveal } from '../useCenterTourReveal';
import { createCenterDemoSession } from '@/components/onboarding/centerDemoSession';
const { state } = vi.hoisted(() => ({ state: { store: null } }));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => state.store }));
const ID = 999000002;
let scope;
afterEach(() => scope?.stop());
function setup() {
  let owner;
  state.store = reactive({ isActive: true, revealOpen: null, centerDemoRevision: 1,
    contextIdentity: () => 'actor1', registerCenterDemoOwner(value) { owner = value; return vi.fn(); } });
  const view = reactive({ selectedApplication: null, sortedApplications: [{ id: ID, status: 'Непрочитано' }],
    fetchApplications: vi.fn(async () => {}),
    openApplication: vi.fn(async (application, operation) => { if (!operation || operation.isCurrent()) view.selectedApplication = application; }),
    closeDetail: vi.fn(() => { view.selectedApplication = null; reveal.release(); }),
  });
  scope = effectScope(); let reveal; scope.run(() => { reveal = useCenterTourReveal(view); });
  return { view, reveal, owner, store: state.store };
}
describe('owned center scene remount', () => {
  it('closes only its fictional card, reloads and reopens the new snapshot', async () => {
    const {view,owner,store}=setup();store.revealOpen='first-application';await nextTick();
    expect(view.selectedApplication.status).toBe('Непрочитано');
    view.fetchApplications.mockImplementationOnce(async () => { view.sortedApplications=[{id:ID,status:'В работе'}]; });
    await owner.replace({isCurrent:()=>true});await nextTick();
    expect(view.closeDetail).toHaveBeenCalledOnce();expect(view.selectedApplication.status).toBe('В работе');
    expect(view.openApplication).toHaveBeenCalledTimes(2);
  });
  it.each([14, ID])('refuses manually opened card %s instead of capturing ownership', async id => {
    const {view,owner}=setup();view.selectedApplication={id,status:'В работе'};await nextTick();
    expect(owner.canReplace()).toBe(false);
    await expect(owner.replace({isCurrent:()=>true})).rejects.toThrow('Закрой');
    expect(view.closeDetail).not.toHaveBeenCalled();expect(view.fetchApplications).not.toHaveBeenCalled();
  });
  it('does not reopen after a cancelled held reload', async () => {
    const {view,owner,store}=setup();store.revealOpen='first-application';await nextTick();
    let release,current=true;view.fetchApplications.mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    const pending=owner.replace({isCurrent:()=>current});await nextTick();current=false;store.isActive=false;
    release();await pending;await nextTick();expect(view.selectedApplication).toBe(null);
  });
  it('guards late initial open when actor/run changes', async () => {
    const {view,store}=setup();let release;
    view.openApplication.mockImplementationOnce(async (application,operation)=>{await new Promise(resolve=>{release=resolve;});if(operation.isCurrent())view.selectedApplication=application;});
    store.revealOpen='first-application';await nextTick();store.centerDemoRevision++;store.isActive=false;
    release();await nextTick();await nextTick();expect(view.selectedApplication).toBe(null);
  });
  it('finds its exact fake card even when an existing status filter hides the next scene',async()=>{
    const {view,owner,store}=setup();store.activeTourKey='accept';store.revealOpen='first-application';await nextTick();
    view.fetchApplications.mockImplementationOnce(async()=>{view.applications=[{id:ID,status:'В работе'}];view.sortedApplications=[];});
    await owner.replace({isCurrent:()=>true});expect(view.selectedApplication.status).toBe('В работе');
  });
  it.each(['real', 'fake', 'close'])('does not capture a %s user intent during held reload',async intent=>{
    const {view,owner,store}=setup();store.revealOpen='first-application';await nextTick();
    let release;view.fetchApplications.mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    const pending=owner.replace({isCurrent:()=>true});const rejected=expect(pending).rejects.toThrow();await nextTick();
    const manual=intent==='close'?null:{id:intent==='real'?14:ID,status:'В работе'};
    if(intent==='close')view.closeDetail();else view.selectedApplication=manual;
    await nextTick();release();await rejected;await nextTick();
    expect(toRaw(view.selectedApplication)).toBe(manual);expect(view.openApplication).toHaveBeenCalledTimes(1);
  });
  it('does not mark a manually interrupted scene as prepared',async()=>{
    const {view,owner,store}=setup();store.revealOpen='first-application';await nextTick();
    const session=createCenterDemoSession({install:vi.fn()});session.begin({actorId:42,tourKey:'accept',now:Date.parse('2026-10-28T12:00:00Z')});session.registerOwner(owner);
    let release;view.fetchApplications.mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    const pending=session.prepare('accept-supplement-approved',{isCurrent:()=>true});const rejected=expect(pending).rejects.toThrow();await nextTick();
    view.selectedApplication={id:14,status:'В работе'};await nextTick();release();await rejected;
    view.closeDetail();store.revealOpen=null;await nextTick();store.revealOpen='first-application';await nextTick();
    await session.prepare('accept-supplement-approved',{isCurrent:()=>true});
    expect(view.fetchApplications).toHaveBeenCalledTimes(2);
  });
});
