import { beforeEach, describe, expect, it, vi } from 'vitest';
import { installAssetLoadRecovery, isAssetLoadError } from '../assetLoadRecovery';

describe('asset loading recovery', () => {
  let target, router, confirm, notify, reload, dirty, confirmOpen, ready, handle;
  const failure = () => new TypeError('Failed to fetch dynamically imported module: /assets/old.js');
  beforeEach(() => {
    target = new EventTarget();
    target.navigator = { onLine: true };
    router = { onError: vi.fn((callback) => { handle = callback; return vi.fn(); }) };
    confirm = vi.fn().mockResolvedValue(true);
    notify = vi.fn(); reload = vi.fn(); dirty = false; confirmOpen = false;
    ready = Promise.resolve();
  });
  function install() {
    return installAssetLoadRecovery({ target, router, ready, confirm, notify, reload,
      isDirty: () => dirty, isConfirmOpen: () => confirmOpen });
  }
  const settle = async () => { for (let n = 0; n < 6; n++) await Promise.resolve(); };
  it('recognizes module and CSS errors, ignores API and application errors', () => {
    expect(isAssetLoadError(failure())).toBe(true);
    expect(isAssetLoadError(new Error('Unable to preload CSS for /assets/old.css'))).toBe(true);
    expect(isAssetLoadError(new Error('error loading dynamically imported module'))).toBe(true);
    for (const error of [null, new Error('HTTP 403'), new TypeError('Failed to fetch'), new Error('render failed')]) {
      expect(isAssetLoadError(error)).toBe(false);
    }
  });
  it('waits for mounted shell, merges Vite/router reports and reloads only after consent', async () => {
    let mount; ready = new Promise((resolve) => { mount = resolve; });
    install(); const error = failure();
    const event = new Event('vite:preloadError', { cancelable: true }); event.payload = error;
    target.dispatchEvent(event); handle(error); await settle();
    expect(confirm).not.toHaveBeenCalled(); expect(reload).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
    mount(); await settle(); expect(confirm).toHaveBeenCalledTimes(1); expect(reload).toHaveBeenCalledTimes(1);
    handle(failure()); await settle(); expect(reload).toHaveBeenCalledTimes(1);
  });
  it('does not reload or ask to discard a dirty form', async () => {
    dirty = true; install(); handle(failure()); await settle();
    expect(notify).toHaveBeenCalled(); expect(confirm).not.toHaveBeenCalled(); expect(reload).not.toHaveBeenCalled();
  });
  it('rechecks dirty state after the confirmation', async () => {
    let accept; confirm.mockImplementation(() => new Promise((resolve) => { accept = resolve; }));
    install(); handle(failure()); await settle(); dirty = true; accept(true); await settle();
    expect(reload).not.toHaveBeenCalled(); expect(notify).toHaveBeenCalled();
  });
  it('does not reload when cancelled', async () => {
    confirm.mockResolvedValue(false); install(); handle(failure()); await settle();
    expect(reload).not.toHaveBeenCalled();
  });
  it('does not replace another confirmation', async () => {
    confirmOpen = true; install(); handle(failure()); await settle();
    expect(confirm).not.toHaveBeenCalled(); expect(notify).toHaveBeenCalled();
  });
  it('offline reports once without retries or reload', async () => {
    target.navigator.onLine = false; install(); const error = failure(); handle(error); handle(error); await settle();
    expect(notify).toHaveBeenCalledTimes(1); expect(confirm).not.toHaveBeenCalled(); expect(reload).not.toHaveBeenCalled();
  });
  it('ignores unrelated router failures', async () => {
    install(); handle(new Error('HTTP 500')); await settle(); expect(confirm).not.toHaveBeenCalled(); expect(notify).not.toHaveBeenCalled();
  });
  it('cleanup cancels a pending recovery and unsubscribes', async () => {
    let mount; ready = new Promise((resolve) => { mount = resolve; });
    const stop = install(); handle(failure()); stop(); mount(); await settle();
    expect(confirm).not.toHaveBeenCalled(); expect(router.onError.mock.results[0].value).toHaveBeenCalled();
    const event = new Event('vite:preloadError'); event.payload = failure(); target.dispatchEvent(event); await settle();
    expect(notify).not.toHaveBeenCalled();
  });
});