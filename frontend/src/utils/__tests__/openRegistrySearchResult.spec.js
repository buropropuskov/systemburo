import { describe, expect, it, vi } from 'vitest';
import { openRegistrySearchResult, openRegistrySearchLink } from '../openRegistrySearchResult';

function setup(openID = '4') {
  const state = { route: { query: { open: openID, q: 'Тест' } } };
  return { state, args: { getRoute: () => state.route, router: { replace: vi.fn().mockResolvedValue(undefined) },
    items: [], open: vi.fn(), fetchPage: vi.fn().mockResolvedValue({ items: [{ id: 4 }], meta: { total: 1 } }) } };
}
describe('registry result opens across permitted sections without replacing tab rows', () => {
  it('uses the loaded row without a second request', async () => {
    const { args } = setup(); args.items = [{ id: 4 }];
    expect(await openRegistrySearchResult(args)).toBe(true);
    expect(args.fetchPage).not.toHaveBeenCalled(); expect(args.open).toHaveBeenCalledWith(args.items[0]);
  });
  it('opens company-only or unbound own results from the permitted union', async () => {
    const { args } = setup();
    expect(await openRegistrySearchResult(args)).toBe(true);
    expect(args.fetchPage).toHaveBeenCalledWith({ filter_type: 'all', search_query: 'Тест', page: 1, per_page: 100 });
    expect(args.items).toEqual([]); expect(args.open).toHaveBeenCalledWith({ id: 4 });
    expect(args.router.replace).toHaveBeenCalledWith({ query: { q: 'Тест' } });
  });
  it('retains system scope only when explicitly supplied', async () => {
    const { args } = setup();
    await openRegistrySearchResult({ ...args, systemRead: true });
    expect(args.fetchPage).toHaveBeenCalledWith(expect.objectContaining({ filter_type: 'all_system' }));
  });
  it('continues through bounded pages until the requested row is found', async () => {
    const { args } = setup();
    args.fetchPage.mockResolvedValueOnce({ items: [{ id: 3 }], meta: { total: 101 } });
    expect(await openRegistrySearchResult(args)).toBe(true);
    expect(args.fetchPage).toHaveBeenCalledTimes(2);
    expect(args.fetchPage.mock.calls[1][0].page).toBe(2);
  });
  it('does not open a stale response after another route is selected', async () => {
    const { args, state } = setup();
    args.fetchPage.mockImplementation(async () => { state.route = { query: {} }; return { items: [{ id: 4 }], meta: { total: 1 } }; });
    expect(await openRegistrySearchResult(args)).toBe(false); expect(args.open).not.toHaveBeenCalled();
    expect(args.router.replace).not.toHaveBeenCalled();
  });
  it('keeps missing or unavailable records closed', async () => {
    const { args } = setup(); args.fetchPage.mockResolvedValue({ items: [], meta: { total: 0 } });
    expect(await openRegistrySearchResult(args)).toBe(false); expect(args.open).not.toHaveBeenCalled();
    args.fetchPage.mockRejectedValue(new Error('forbidden'));
    await expect(openRegistrySearchResult(args)).rejects.toThrow('forbidden'); expect(args.open).not.toHaveBeenCalled();
  });
  it('ignores a missing or invalid open parameter', async () => {
    const { args } = setup('invalid');
    expect(await openRegistrySearchResult(args)).toBe(false); expect(args.fetchPage).not.toHaveBeenCalled();
  });
  it('does not clear another registry route with the same record ID', async () => {
    const { args, state } = setup(); state.route.path = '/carsview';
    args.fetchPage.mockImplementation(async () => {
      state.route = { path: '/employeesview', query: { open: '4', q: 'Тест' } };
      return { items: [{ id: 4 }], meta: { total: 1 } };
    });
    expect(await openRegistrySearchResult(args)).toBe(false);
    expect(args.open).not.toHaveBeenCalled(); expect(args.router.replace).not.toHaveBeenCalled();
  });
  it('discards a response after the view was unmounted', async () => {
    const { args, state } = setup();
    const view = { $route: state.route, $router: args.router, $: { isUnmounted: false } };
    args.fetchPage.mockImplementation(async () => { view.$.isUnmounted = true; return { items: [{ id: 4 }], meta: { total: 1 } }; });
    expect(await openRegistrySearchLink(view, [], args.open, args.fetchPage, false)).toBe(false);
    expect(args.open).not.toHaveBeenCalled(); expect(args.router.replace).not.toHaveBeenCalled();
  });
  it('opens a mounted view outside the document', async () => {
    const { args, state } = setup();
    const view = { $route: state.route, $router: args.router, $el: { isConnected: false }, $: { isUnmounted: false } };
    expect(await openRegistrySearchLink(view, [], args.open, args.fetchPage, false)).toBe(true);
    expect(args.open).toHaveBeenCalledWith({ id: 4 });
  });
  it('coalesces repeated watchers for the same card', async () => {
    const { args, state } = setup(); let resolve;
    args.fetchPage.mockImplementation(() => new Promise(done => { resolve = done; }));
    const view = { $route: state.route, $router: args.router };
    const first = openRegistrySearchLink(view, [], args.open, args.fetchPage, false);
    expect(await openRegistrySearchLink(view, [], args.open, args.fetchPage, false)).toBe(false);
    resolve({ items: [{ id: 4 }], meta: { total: 1 } });
    expect(await first).toBe(true); expect(args.fetchPage).toHaveBeenCalledTimes(1);
    expect(args.open).toHaveBeenCalledTimes(1);
  });
  it('permits a new target while discarding a previous in-flight response', async () => {
    const { args, state } = setup(); let resolveFirst;
    args.fetchPage.mockImplementationOnce(() => new Promise(done => { resolveFirst = done; }))
      .mockResolvedValueOnce({ items: [{ id: 5 }], meta: { total: 1 } });
    const view = { $route: state.route, $router: args.router };
    const first = openRegistrySearchLink(view, [], args.open, args.fetchPage, false);
    view.$route = { query: { open: '5', q: 'Тест' } };
    expect(await openRegistrySearchLink(view, [], args.open, args.fetchPage, false)).toBe(true);
    resolveFirst({ items: [{ id: 4 }], meta: { total: 1 } });
    expect(await first).toBe(false); expect(args.open).toHaveBeenCalledTimes(1);
    expect(args.open).toHaveBeenCalledWith({ id: 5 });
  });
});
