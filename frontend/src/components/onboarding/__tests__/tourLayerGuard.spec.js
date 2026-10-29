import { afterEach, describe, expect, it, vi } from 'vitest';
import { createTourLayerGuard } from '../tourLayerGuard';
import { setModalOpen, releaseModal, isTopModal, resetModalStack } from '@/utils/modalStack';

let guard;
afterEach(() => { guard?.dispose(); resetModalStack(); document.body.replaceChildren(); });

describe('tour and global dialog ownership', () => {
  it('pauses for a manually opened action menu but keeps a tour-owned menu teachable', async () => {
    const changed = vi.fn(); let reveal = 'first-application';
    guard = createTourLayerGuard(changed, { getRevealOpen: () => reveal }); guard.activate();
    const popup = document.createElement('div'); popup.className = 'base-dropdown__menu';
    const trigger = document.createElement('div'); trigger.className = 'detail-actions-menu';
    trigger.dataset.dropdownOpen = 'false'; document.body.append(trigger);
    const actions = document.createElement('div'); actions.className = 'detail-header-actions--compact';
    actions.getBoundingClientRect = () => ({ top: 100, left: 100, width: 300, height: 200 });
    popup.append(actions); document.body.append(popup); await Promise.resolve();
    expect(guard.isBlocked()).toBe(false);
    trigger.dataset.dropdownOpen = 'true'; await Promise.resolve();
    expect(guard.isBlocked()).toBe(true);
    popup.remove(); await Promise.resolve(); expect(guard.isBlocked()).toBe(false);
    reveal = 'application-actions'; document.body.append(popup); await Promise.resolve();
    expect(guard.isBlocked()).toBe(false); expect(changed.mock.calls).toEqual([[true], [false]]);
  });

  it('pauses for the download dialog and returns keyboard ownership after it closes', async () => {
    const changed = vi.fn(), background = {}, dialog = {};
    setModalOpen(background, true, 9000);
    const overlay = document.createElement('div'); overlay.dataset.testid = 'download-blanks-modal'; overlay.hidden = true;
    overlay.getBoundingClientRect = () => ({ top: 0, left: 0, width: 390, height: 844 });
    document.body.append(overlay);
    guard = createTourLayerGuard(changed);
    guard.activate(); expect(guard.isBlocked()).toBe(false);
    overlay.hidden = false; setModalOpen(dialog, true, 10000); await Promise.resolve();
    expect(guard.isBlocked()).toBe(true); expect(isTopModal(dialog)).toBe(true);
    overlay.hidden = true; releaseModal(dialog); await Promise.resolve();
    expect(guard.isBlocked()).toBe(false); expect(isTopModal(background)).toBe(false);
    expect(changed.mock.calls).toEqual([[true], [false]]);
  });

  it('preserves focus in the download dialog even when its ordinary layer is below the tour', () => {
    const previous = document.createElement('button'), overlay = document.createElement('div');
    overlay.dataset.testid = 'download-blanks-modal'; const field = document.createElement('input'); overlay.append(field);
    document.body.append(previous, overlay); field.focus();
    guard = createTourLayerGuard(vi.fn());
    guard.preserveGlobalFocus(() => previous.focus());
    expect(document.activeElement).toBe(field);
  });

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
