import { describe, expect, it, vi } from 'vitest';
import { createStepPreparation } from '../stepPreparation';
import { applyReveal } from '../reveal';
vi.mock('../reveal',()=>({applyReveal:vi.fn(async()=>false)}));
function setup(prepareCenterDemo){const store={steps:[{id:'scene',demoScene:'accept-supplement-approved'}],prepareCenterDemo};const execute=createStepPreparation({store,watchers:{stopAll:vi.fn()},applyDemoAttachment:()=>false,beforePrepare:()=>{},waitSelectorOf:step=>step.element,waitForElement:vi.fn(),ensureInView:vi.fn(),demoFallback:'demo'});return execute;}
describe('scene preparation belongs to the current host intent',()=>{
  it('awaits the semantic scene before reveal, including a direct chapter jump',async()=>{
    applyReveal.mockClear();let release;const prepare=vi.fn(()=>new Promise(resolve=>{release=resolve;}));
    const run=setup(prepare)(0,{isCurrent:()=>true});expect(applyReveal).not.toHaveBeenCalled();
    release();await expect(run).resolves.toBe(true);expect(applyReveal).toHaveBeenCalledOnce();
  });
  it('does not reveal after cancelling a held scene replacement',async()=>{
    applyReveal.mockClear();let release,current=true;const run=setup(()=>new Promise(resolve=>{release=resolve;}))(0,{isCurrent:()=>current});
    current=false;release();await run;expect(applyReveal).not.toHaveBeenCalled();
  });
  it('keeps owner failure as recovery rather than skipping or false completion',async()=>{
    applyReveal.mockClear();await expect(setup(async()=>{throw new Error('owned scene failed');})(0,{isCurrent:()=>true})).rejects.toThrow('owned scene failed');expect(applyReveal).not.toHaveBeenCalled();
  });
});
