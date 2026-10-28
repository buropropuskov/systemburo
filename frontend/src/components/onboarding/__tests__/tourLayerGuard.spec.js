import { afterEach, describe, expect, it, vi } from 'vitest';
import { createTourLayerGuard } from '../tourLayerGuard';
import { setModalOpen, releaseModal, isTopModal, resetModalStack } from '@/utils/modalStack';

let guard;
afterEach(() => { guard?.dispose(); resetModalStack(); document.body.replaceChildren(); });

describe('tour and global dialog ownership', () => {
  it('gives the upper dialog keyboard ownership and resumes after it closes', () => {
    const changed = vi.fn(), lower = {}, upper = {};
    setModalOpen(lower, true, 11000);
    guard = createTourLayerGuard(changed); guard.activate();
    expect(isTopModal(lower)).toBe(false);
    setModalOpen(upper, true, 22000);
    expect(guard.isBlocked()).toBe(true); expect(isTopModal(upper)).toBe(true);
    releaseModal(upper);
    expect(guard.isBlocked()).toBe(false);
    expect(changed.mock.calls).toEqual([[true], [false]]);
    guard.deactivate(); expect(isTopModal(lower)).toBe(true);
  });

  it('also pauses for consent outside the shared stack and ignores a hidden overlay', async () => {
    guard = createTourLayerGuard(vi.fn()); guard.activate();
    const overlay = document.createElement('div'); overlay.className = 'pdc-overlay';
    overlay.getBoundingClientRect = () => ({ top: 0, left: 0, width: 390, height: 844 });
    document.body.append(overlay); await Promise.resolve();
    expect(guard.isBlocked()).toBe(true);
    overlay.hidden = true; await Promise.resolve();
    expect(guard.isBlocked()).toBe(false);
  });

  it('disposes both stack and DOM subscriptions without a late resume', async () => {
    const changed = vi.fn(); guard = createTourLayerGuard(changed); guard.activate();
    setModalOpen({}, true, 22000); expect(changed).toHaveBeenCalledOnce();
    guard.dispose(); resetModalStack();
    document.body.append(document.createElement('div')); await Promise.resolve();
    expect(changed).toHaveBeenCalledOnce(); expect(guard.isBlocked()).toBe(false);
  });

  it('preserves focus in the upper dialog across Driver public destroy focus restoration', () => {
    guard = createTourLayerGuard(vi.fn());
    const previous = document.createElement('button'), dialog = document.createElement('div');
    dialog.style.zIndex = '22000'; const field = document.createElement('input'); dialog.append(field);
    document.body.append(previous, dialog); field.focus();
    guard.preserveGlobalFocus(() => previous.focus());
    expect(document.activeElement).toBe(field);
  });
});
