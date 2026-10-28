import { describe, expect, it, vi } from 'vitest';
import { createTourRecoveryState } from '../tourRecoveryState';

function fixture() {
  const calls = [], restore = vi.fn(async id => calls.push(['restore', id])), resume = vi.fn(), stop = vi.fn();
  let controls;
  const state = createTourRecoveryState({
    getSteps: () => [{ id: 'shown' }, { id: 'target' }], restore, resume, stop,
    present: value => { calls.push(['present']); controls = value; },
  });
  return { state, calls, restore, resume, stop, controls: () => controls };
}

describe('recovery preserves intended and committed step identities', () => {
  it.each(['initial', 'next', 'previous', 'jump', 'route'])('retries the exact %s target after rollback', async intent => {
    const f = fixture(); if (intent !== 'initial') f.state.commit('shown');
    f.state.prepare('target');
    await f.state.fail({ reason: 'load-error', stepId: 'target' });
    expect(f.calls).toEqual([['restore', intent === 'initial' ? null : 'shown'], ['present']]);
    f.controls().retry(); expect(f.resume).toHaveBeenCalledWith('target');
    expect(Boolean(f.controls().back)).toBe(intent !== 'initial');
  });

  it('returns to the shown ID and closes without a completion callback', async () => {
    const f = fixture(); f.state.commit('shown'); f.state.prepare('target'); await f.state.fail({});
    f.controls().back(); expect(f.resume).toHaveBeenCalledWith('shown');
    f.controls().close(); expect(f.stop).toHaveBeenCalledWith('shown');
  });
  it('rolls back a pending target before a global technical interruption', async () => {
    const f = fixture(); f.state.commit('shown'); f.state.prepare('target');
    await f.state.interrupt();
    expect(f.calls).toEqual([['restore', 'shown']]);
    expect(f.stop).toHaveBeenCalledWith('shown');
  });
  it('does not let a late failure supersede a terminal interruption', async () => {
    const f = fixture(); f.state.commit('shown'); f.state.prepare('target');
    const pending = f.state.interrupt();
    const late = f.state.fail({ stepId: 'target' }, { isCurrent: () => true });
    await Promise.all([pending, late]);
    expect(f.calls).toEqual([['restore', 'shown']]);
    expect(f.stop).toHaveBeenCalledWith('shown');
    expect(f.state.isShowing()).toBe(false);
  });

  it('keeps failure on pause and retry until an actual step commits', async () => {
    const f = fixture(); f.state.prepare('target'); await f.state.fail({});
    expect(f.state.retryAt('target')).toBe(1); expect(f.state.isShowing()).toBe(false);
    expect(f.state.hasFailure()).toBe(true); f.state.show(); expect(f.state.isShowing()).toBe(true);
    f.state.commit('target'); expect(f.state.hasFailure()).toBe(false);
  });

  it('never retries a different step occupying a removed target index', async () => {
    const f = fixture(); f.state.commit('shown'); f.state.prepare('target'); await f.state.fail({});
    expect(f.state.retryAt('removed')).toBeUndefined(); expect(f.stop).toHaveBeenCalledWith('shown');
  });

  it('does not resurrect recovery after stop or cancelled restoration', async () => {
    const f = fixture(); f.state.prepare('target');
    const pending = f.state.fail({}, { isCurrent: () => false });
    await pending; expect(f.calls).toEqual([['restore', null]]);
    const another = f.state.fail({}); f.state.reset(); await another;
    expect(f.calls).toEqual([['restore', null], ['restore', null]]);
  });

  it('invalidates delayed failure presentation only when a new target is prepared', async () => {
    const f = fixture();
    let release;
    f.restore.mockImplementationOnce(() => new Promise(resolve => { release = resolve; }));
    f.state.commit('shown'); f.state.prepare('target');
    const pending = f.state.fail({ stepId: 'target' });
    f.state.prepare('shown');
    release(); await pending;
    expect(f.controls()).toBeUndefined();
    expect(f.state.hasFailure()).toBe(true);
    f.state.show(); f.controls().retry(); f.controls().back();
    expect(f.resume.mock.calls).toEqual([['shown'], ['shown']]);
  });

  it('keeps the last rendered identity separate from a newly prepared target', async () => {
    const f = fixture(); f.state.commit('shown'); f.state.prepare('target');
    await f.state.fail({ stepId: 'target' });
    f.state.prepare('replacement'); f.state.show();
    f.controls().retry(); f.controls().back();
    expect(f.resume.mock.calls).toEqual([['replacement'], ['shown']]);
  });

  it('offers no unreachable Back after the committed step loses access', async () => {
    let steps = [{ id: 'cars' }, { id: 'employees' }], controls;
    const resume = vi.fn(), stop = vi.fn();
    const state = createTourRecoveryState({ getSteps: () => steps, restore: async () => {},
      present: value => { controls = value; }, resume, stop });
    state.commit('cars'); steps = [{ id: 'employees' }]; state.prepare('employees');
    await state.fail({ stepId: 'employees' });
    expect(controls.back).toBeUndefined(); controls.retry();
    expect(resume).toHaveBeenCalledWith('employees'); expect(stop).not.toHaveBeenCalled();
  });
});
