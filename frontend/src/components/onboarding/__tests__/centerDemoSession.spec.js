import { describe, it, expect, vi } from 'vitest';
import { createCenterDemoSession } from '../centerDemoSession';
const input = { actorId: 42, tourKey: 'accept', now: Date.parse('2026-12-31T23:59:00Z') };
const operation = () => ({ isCurrent: () => true });
describe('center demo data lifetime', () => {
  it('requires a known actor, does not invent an account or membership', () => {
    const session = createCenterDemoSession({ install: vi.fn() });
    expect(() => session.begin({ ...input, actorId: null })).toThrow();
    expect(session.active()).toBe(false);
  });
  it('restores the same clock/snapshot with Back or a direct scene jump', async () => {
    const installed = [], session = createCenterDemoSession({ install: value => installed.push(value) });
    session.begin(input);
    const base = installed.at(-1);
    await session.prepare('accept-supplement-approved', operation());
    expect(installed.at(-1).application.status).toBe('В работе');
    expect(installed.at(-1).cars).toHaveLength(3);
    await session.prepare('accept-ready', operation());
    expect(installed.at(-1)).toBe(base);
    expect(base.now).toBe(input.now);
  });
  it('does not replace a manually opened real card', async () => {
    const install = vi.fn(), session = createCenterDemoSession({ install }); session.begin(input);
    session.registerOwner({ canReplace: () => false, replace: vi.fn() });
    await expect(session.prepare('accept-ready', operation())).rejects.toThrow('Закрой');
    expect(install.mock.calls.at(-1)[0].application.status).toBe('Непрочитано');
  });
  it.each(['same', 'different'])('old held owner cannot commit into a %s actor restart', async kind => {
    const install = vi.fn(), session = createCenterDemoSession({ install }); session.begin(input);
    let release, oldOperation;
    const replace = vi.fn(args => { oldOperation = args; return new Promise(resolve => { release = resolve; }); });
    session.registerOwner({ canReplace: () => true, replace });
    const pending = session.prepare('accept-supplement-approved', operation());
    session.stop(); session.begin({ ...input, actorId: kind === 'same' ? 42 : 43 });
    expect(oldOperation.isCurrent()).toBe(false); release(); await pending;
    expect(install.mock.calls.at(-1)[0].actorId).toBe(kind === 'same' ? 42 : 43);
    expect(install.mock.calls.at(-1)[0].scene).toBe('accept-ready');
  });
  it('cancelled preparation is not accepted and must prepare its owner again', async () => {
    const session = createCenterDemoSession({ install: vi.fn() }); session.begin(input);
    const replace = vi.fn().mockResolvedValue();session.registerOwner({ canReplace: () => true, replace });
    await session.prepare('accept-ready', { isCurrent: () => false });
    expect(replace).not.toHaveBeenCalled();
    await session.prepare('accept-ready', operation()); expect(replace).toHaveBeenCalledOnce();
  });
  it('retries a cancelled scene replacement even when that owner prepared the prior scene', async () => {
    const session=createCenterDemoSession({install:vi.fn()});session.begin(input);
    const replace=vi.fn().mockResolvedValue();session.registerOwner({canReplace:()=>true,replace});
    await session.prepare('accept-ready',operation());
    let release,current=true;replace.mockReturnValueOnce(new Promise(resolve=>{release=resolve;}));
    const pending=session.prepare('accept-supplement-approved',{isCurrent:()=>current});
    current=false;release();await pending;
    await session.prepare('accept-supplement-approved',operation());expect(replace).toHaveBeenCalledTimes(3);
  });
});
