import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ApplicationDetail from '../ApplicationDetail/ApplicationDetail.vue';
import { apiRequest } from '@/api/client';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
const load = ApplicationDetail.methods.loadAttachmentDetails;
function context(type = 'people') {
  return { loadAttachmentSeq: 0, application: { id: 5 }, applicationData: { id: 5 },
    selectedAttachment: { id: 7 }, attachments: [{ id: 7, attachment_type: type }, { id: 8, attachment_type: type }],
    attachmentCars: [], attachmentEmployees: [], attachmentItems: [], loadingAttachmentDetails: false };
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
const response = data => ({ ok: true, json: async () => data });
const types = [['cars', 'attachmentCars'], ['people', 'attachmentEmployees'], ['items', 'attachmentItems']];

describe('Attachment reader sequence/context for period refresh #2665', () => {
  beforeEach(() => apiRequest.mockReset());
  afterEach(() => vi.restoreAllMocks());
  it.each(types)('%s does not let an older GET overwrite the period refresh', async (type, rows) => {
    const ctx = context(type);
    const old = deferred();
    apiRequest.mockReturnValueOnce(old.promise).mockResolvedValueOnce(response([{ id: 17, entry_date_to: '2035-01-05' }]));
    const pending = load.call(ctx, 7);
    await load.call(ctx, 7);
    expect(ctx[rows][0].entry_date_to).toBe('2035-01-05');
    old.resolve(response([{ id: 17, entry_date_to: '2035-01-02' }]));
    await pending;
    expect(ctx[rows][0].entry_date_to).toBe('2035-01-05');
    expect(ctx.loadingAttachmentDetails).toBe(false);
  });
  it.each(['application prop', 'application data', 'selection'])('checks %s again after delayed JSON parsing', async change => {
    const ctx = context();
    const json = deferred();
    apiRequest.mockResolvedValue({ ok: true, json: () => json.promise });
    const pending = load.call(ctx, 7);
    await Promise.resolve();
    if (change === 'application prop') ctx.application = { id: 6 };
    else if (change === 'application data') ctx.applicationData = { id: 6 };
    else ctx.selectedAttachment = { id: 8 };
    json.resolve([{ id: 17, entry_date_to: '2035-01-02' }]);
    await pending;
    expect(ctx.attachmentEmployees).toEqual([]);
    expect(ctx.loadingAttachmentDetails).toBe(false);
  });
  it('an old request cannot clear loading while the new selection is still loading', async () => {
    const ctx = context();
    const first = deferred(), second = deferred();
    apiRequest.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const old = load.call(ctx, 7);
    ctx.selectedAttachment = { id: 8 };
    const current = load.call(ctx, 8);
    first.resolve(response([{ id: 17 }]));
    await old;
    expect(ctx.loadingAttachmentDetails).toBe(true);
    expect(ctx.attachmentEmployees).toEqual([]);
    second.resolve(response([{ id: 18 }]));
    await current;
    expect(ctx.attachmentEmployees).toEqual([{ id: 18 }]);
    expect(ctx.loadingAttachmentDetails).toBe(false);
  });
  it('preserves active error cleanup and suppresses stale request error reporting', async () => {
    const ctx = context();
    const report = vi.spyOn(console, 'error').mockImplementation(() => {});
    const first = deferred();
    apiRequest.mockReturnValueOnce(first.promise).mockResolvedValueOnce(response([{ id: 18 }]));
    const old = load.call(ctx, 7);
    await load.call(ctx, 7);
    first.reject(new Error('old request'));
    await old;
    expect(report).not.toHaveBeenCalled();
    apiRequest.mockRejectedValueOnce(new Error('current request'));
    await load.call(ctx, 7);
    expect(report).toHaveBeenCalledTimes(1);
    expect(ctx.attachmentEmployees).toEqual([]);
    expect(ctx.loadingAttachmentDetails).toBe(false);
  });
});
