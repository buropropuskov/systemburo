import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createStepPreparation, waitSelectorOf } from '../stepPreparation';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
import { countShownSteps } from '@/composables/useOnboarding';
import { groupStepsBySection, chapterOf } from '../stepsFlow';
import { applyReveal } from '../reveal';
import { buildSecurityFactSteps } from '../securityFactSteps';
import { findTourTarget } from '../targetResolver';
vi.mock('../reveal',()=>({applyReveal:vi.fn(async()=>false)}));
vi.mock('@/stores/onboarding',()=>({useOnboardingStore:()=>({})}));
const owner=(key,state)=>{const node=document.createElement('div');node.dataset.obOwner=key;node.dataset.obState=state;document.body.append(node);return node;};
function prepare(step, targetReader=async()=>null){const controller=new AbortController();const waitForElement=vi.fn(targetReader);const execute=createStepPreparation({store:{steps:[step]},watchers:{stopAll:vi.fn()},beforePrepare:()=>{},applyDemoAttachment:()=>false,waitSelectorOf,waitForElement,ensureInView:vi.fn(),demoFallback:'demo'});return{run:execute(0,{isCurrent:()=>!controller.signal.aborted,signal:controller.signal}),waitForElement,controller};}
beforeEach(()=>{vi.useFakeTimers();applyReveal.mockReset().mockResolvedValue(false);});
afterEach(()=>{document.body.replaceChildren();vi.clearAllTimers();vi.useRealTimers();});
describe('known empty accessible owner',()=>{
  it.each(['sec-aa-detail','sec-aa-elements','sec-aa-preview','sec-aa-blank'])('%s declares both list prerequisite and selected detail dependency',id=>{
    const step=securityOnboardingSteps.find(s=>s.id===id);expect(step.dataPrerequisite).toBe('accessible-list');expect(step.dataOwner).toBe('accessible-detail');
    if(['sec-aa-preview','sec-aa-blank'].includes(id))expect(step.dataAvailability).toBe('blank');
  });
  it.each(['sec-aa-preview','sec-aa-blank'])('known empty list skips %s without revealing or starting a target wait',async id=>{
    owner('accessible-list','empty');owner('accessible-detail','empty');const probe=prepare(securityOnboardingSteps.find(s=>s.id===id));
    await expect(probe.run).resolves.toBe(false);expect(applyReveal).not.toHaveBeenCalled();expect(probe.waitForElement).not.toHaveBeenCalled();
  });
  it.each(['error','forbidden'])('does not turn %s list plus default empty detail into a skip',async state=>{
    owner('accessible-list',state);owner('accessible-detail','empty');const probe=prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-blank'));
    await expect(probe.run).rejects.toMatchObject({reason:state});expect(probe.waitForElement).not.toHaveBeenCalled();
  });
  it('waits for a loading list instead of consuming the default empty detail',async()=>{
    const list=owner('accessible-list','loading');owner('accessible-detail','empty');const probe=prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-preview'));let settled=false;probe.run.then(()=>{settled=true;});
    await vi.advanceTimersByTimeAsync(200);expect(settled).toBe(false);expect(applyReveal).not.toHaveBeenCalled();list.dataset.obState='empty';await vi.advanceTimersByTimeAsync(50);await expect(probe.run).resolves.toBe(false);
  });
  it('unknown owner is a bounded readiness error, not empty',async()=>{
    owner('accessible-detail','empty');const probe=prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-preview'));const outcome=expect(probe.run).rejects.toMatchObject({reason:'timeout'});await vi.advanceTimersByTimeAsync(5000);await outcome;
  });
  it('cancellation while the prerequisite loads neither reveals nor skips a step',async()=>{
    owner('accessible-list','loading');owner('accessible-detail','empty');const probe=prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-preview'));
    await vi.advanceTimersByTimeAsync(100);probe.controller.abort();await expect(probe.run).resolves.toBeUndefined();expect(applyReveal).not.toHaveBeenCalled();expect(probe.waitForElement).not.toHaveBeenCalled();
  });
  it.each(['sec-aa-preview','sec-aa-blank'])('known missing blank skips %s before file reveal or target timeout',async id=>{
    owner('accessible-list','ready');const detail=owner('accessible-detail','ready');detail.dataset.obBlank='empty';
    const probe=prepare(securityOnboardingSteps.find(s=>s.id===id));await expect(probe.run).resolves.toBe(false);
    expect(probe.waitForElement).not.toHaveBeenCalled();expect(applyReveal.mock.calls.some(args=>args[0][args[1]].reveal.open==='attachment-blank')).toBe(false);
  });
  it.each(['error','forbidden'])('blank availability %s is not an absent file',async state=>{
    owner('accessible-list','ready');const detail=owner('accessible-detail','ready');detail.dataset.obBlank=state;
    await expect(prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-blank')).run).rejects.toMatchObject({reason:state});
  });
  it('missing blank availability remains unknown and retryable',async()=>{
    owner('accessible-list','ready');owner('accessible-detail','ready');const probe=prepare(securityOnboardingSteps.find(s=>s.id==='sec-aa-blank'));
    const outcome=expect(probe.run).rejects.toMatchObject({reason:'timeout'});await vi.advanceTimersByTimeAsync(5000);await outcome;
  });
  it.each(['sec-aa-preview','sec-aa-blank'])('loading detail and a real blank preserve the live %s step',async id=>{
    owner('accessible-list','ready');const detail=owner('accessible-detail','empty');detail.dataset.obBlank='empty';
    let selected=false;
    applyReveal.mockImplementation(async(steps,index)=>{const open=steps[index].reveal.open;if(open==='first-attachment'&&!selected){selected=true;detail.dataset.obState='loading';detail.dataset.obBlank='loading';setTimeout(()=>{detail.dataset.obState='ready';detail.dataset.obBlank='ready';const button=document.createElement('button');button.dataset.testid='aa-preview-blank';button.getBoundingClientRect=()=>({top:20,left:20,width:100,height:40});document.body.append(button);},100);}if(open==='attachment-blank'){const node=document.createElement('div');node.dataset.testid='ob-blank-preview';node.getBoundingClientRect=()=>({top:20,left:20,width:100,height:40});document.body.append(node);}return true;});
    const probe=prepare(securityOnboardingSteps.find(s=>s.id===id),async selector=>document.querySelector(selector));await vi.advanceTimersByTimeAsync(200);await expect(probe.run).resolves.toBe(true);
    expect(selected).toBe(true);expect(probe.waitForElement).toHaveBeenCalled();
    if(id==='sec-aa-blank')expect(applyReveal.mock.calls.map(args=>args[0][args[1]].reveal.open)).toEqual(['first-attachment','attachment-blank']);
  });
  it('keeps mandatory Fact material with an empty list and no live post',async()=>{
    owner('accessible-list','empty');owner('accessible-detail','empty');const fact=buildSecurityFactSteps(null).find(step=>step.id==='sec-fact-intro');expect(fact.optional).not.toBe(true);await expect(prepare(fact).run).resolves.toBe(true);
  });
});
describe('accessible intro live target',()=>{
  function intro(state, testid, { initial = true, hidden = false } = {}) {
    const list = owner('accessible-list', state);
    list.hidden = hidden;
    const target = document.createElement('div');
    target.dataset.testid = testid;
    target.getBoundingClientRect = () => ({ top:20,left:20,width:300,height:100 });
    list.append(target);
    const step = securityOnboardingSteps.find(s => s.id === 'sec-aa-intro');
    const ensureInView = vi.fn();
    const waitForElement = vi.fn(async selector => findTourTarget(selector));
    const execute = createStepPreparation({
      store:{ steps:[step] }, watchers:{ stopAll:vi.fn() }, beforePrepare:()=>{},
      applyDemoAttachment:()=>false, waitSelectorOf, waitForElement, ensureInView, demoFallback:'demo',
    });
    const run = execute(0, { isCurrent:()=>true, signal:new AbortController().signal }, { initial });
    return { list, target, run, ensureInView, waitForElement };
  }
  it.each([true,false])('prepares the visible empty state with initial=%s without centering or skipping',async initial=>{
    const probe = intro('empty','aa-empty',{ initial });
    await expect(probe.run).resolves.toBe(true);
    expect(probe.ensureInView).toHaveBeenCalledWith(probe.target,'start',expect.any(AbortSignal));
    expect(probe.waitForElement).toHaveBeenCalledTimes(1);
  });
  it('prepares the populated list as the same intro',async()=>{
    const probe = intro('ready','aa-list');
    await expect(probe.run).resolves.toBe(true);
    expect(probe.ensureInView).toHaveBeenCalledWith(probe.target,'start',expect.any(AbortSignal));
  });
  it('does not consume a stale empty target until its owner finishes loading',async()=>{
    const probe = intro('loading','aa-empty');
    let settled = false;
    probe.run.then(()=>{ settled = true; });
    await vi.advanceTimersByTimeAsync(200);
    expect(settled).toBe(false);
    expect(probe.waitForElement).not.toHaveBeenCalled();
    expect(probe.ensureInView).not.toHaveBeenCalled();
    probe.list.dataset.obState = 'empty';
    await vi.advanceTimersByTimeAsync(50);
    await expect(probe.run).resolves.toBe(true);
    expect(probe.ensureInView).toHaveBeenCalledWith(probe.target,'start',expect.any(AbortSignal));
  });
  it.each(['error','forbidden'])('keeps %s recovery despite a stale empty target',async state=>{
    const probe = intro(state,'aa-empty');
    await expect(probe.run).rejects.toMatchObject({ stepId:'sec-aa-intro',reason:state });
    expect(probe.waitForElement).not.toHaveBeenCalled();
    expect(probe.ensureInView).not.toHaveBeenCalled();
  });
  it.each([[true,'demo'],[false,false]])('preserves hidden-list fallback with initial=%s',async(initial,result)=>{
    const probe = intro('empty','aa-empty',{ initial,hidden:true });
    const populated = probe.target.cloneNode();
    populated.dataset.testid = 'aa-list';
    populated.getBoundingClientRect = probe.target.getBoundingClientRect;
    probe.list.append(populated);
    await expect(probe.run).resolves.toBe(result);
    expect(probe.list.hidden).toBe(true);
    expect(probe.ensureInView).not.toHaveBeenCalled();
    expect(probe.waitForElement).not.toHaveBeenCalled();
  });
});
describe('one visible step projection',()=>{
  const steps=[{id:'one',route:'/news'},{id:'empty1',route:'/empty'},{id:'empty2',route:'/empty'},{id:'fact',route:'/news'},{id:'finish',route:'/news'}];
  it('counts only unique valid known skips, exactly like the dropdown',()=>{
    const skipped=[1,2,2,-1,8,1.5,'3'];const groups=groupStepsBySection(steps,skipped);const menu=groups.flatMap(g=>g.items);
    expect(countShownSteps(steps,3,skipped)).toEqual({index:2,total:menu.length});expect(countShownSteps(steps,4,skipped)).toEqual({index:3,total:3});
    expect(chapterOf(steps,3,skipped)).toMatchObject({number:1,total:1});
  });
  it('does not remove future optional targets before their owner outcome is known',()=>{
    const future=steps.map(step=>({...step,optional:true}));expect(countShownSteps(future,0,[])).toEqual({index:1,total:5});expect(groupStepsBySection(future).flatMap(group=>group.items)).toHaveLength(5);
  });
  it('supports Back/Jump and a wholly empty projection without false completion',()=>{
    expect(countShownSteps(steps,0,[1,2])).toEqual({index:1,total:3});expect(countShownSteps(steps,3,[1,2])).toEqual({index:2,total:3});
    expect(countShownSteps([],0,[])).toEqual({index:0,total:0});expect(countShownSteps(steps,4,[0,1,2,3,4])).toEqual({index:0,total:0});
  });
});
