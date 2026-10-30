import { beforeEach, describe, expect, it, vi } from 'vitest';

const getFieldConfig = vi.hoisted(() => vi.fn());
vi.mock('@/api/attachment-templates', () => ({ getFieldConfig }));
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), apiRequestRaw: vi.fn() }));
import SupplementModal from '../SupplementModal.vue';

const load = (vm, id) => SupplementModal.methods.loadFieldConfig.call(vm, id);
const state = () => ({ fieldConfigByAttachment: {}, fieldConfigSeq: 0, loadingFieldConfig: false });
const configured = { base: [{ key: 'position', visible: false, required: false, locked: true, requirable: false }] };
beforeEach(() => { getFieldConfig.mockReset(); });

describe('supplement field configuration retry and concurrent reads', () => {
  it('retries a failed read instead of caching the fallback as success', async () => {
    const vm = state();
    getFieldConfig.mockRejectedValueOnce(new Error('Temporary read failure')).mockResolvedValueOnce(configured);
    await load(vm, 17);
    expect(vm.loadingFieldConfig).toBe(false);
    expect(vm.fieldConfigByAttachment).not.toHaveProperty('17');
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(2);
    expect(vm.fieldConfigByAttachment[17]).toEqual({ position: { visible: false, required: false, locked: true, requirable: false } });
    expect(vm.loadingFieldConfig).toBe(false);
  });

  it.each([configured, { base: [] }])('retains a successful concurrent response after a late failure ($base)', async (configuration) => {
    const vm = state();
    let resolveFirst;
    let rejectSecond;
    getFieldConfig
      .mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((_, reject) => { rejectSecond = reject; }));
    const first = load(vm, 17);
    await vi.waitFor(() => expect(getFieldConfig).toHaveBeenCalledTimes(1));
    const second = load(vm, 17);
    await vi.waitFor(() => expect(getFieldConfig).toHaveBeenCalledTimes(2));
    resolveFirst(configuration);
    await first;
    const successful = vm.fieldConfigByAttachment[17];
    expect(vm.loadingFieldConfig).toBe(true);
    rejectSecond(new Error('Late read failure'));
    await second;
    expect(vm.fieldConfigByAttachment[17]).toBe(successful);
    expect(vm.loadingFieldConfig).toBe(false);
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(2);
  });

  it('caches a successfully read empty configuration', async () => {
    const vm = state();
    getFieldConfig.mockResolvedValue({ base: [] });
    await load(vm, 17);
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(1);
    expect(vm.fieldConfigByAttachment[17]).toEqual({});
  });

  it('releases the active form when returning to a cached template while another read is pending', async () => {
    const vm = state();
    const cached = { position: { visible: true } };
    vm.fieldConfigByAttachment[17] = cached;
    let resolvePending;
    getFieldConfig.mockImplementation(() => new Promise(resolve => { resolvePending = resolve; }));
    const pending = load(vm, 18);
    await vi.waitFor(() => expect(getFieldConfig).toHaveBeenCalledTimes(1));
    expect(vm.loadingFieldConfig).toBe(true);
    try {
      await load(vm, 17);
      expect(vm.loadingFieldConfig).toBe(false);
      expect(vm.fieldConfigByAttachment[17]).toBe(cached);
    } finally {
      resolvePending(configured);
      await pending;
    }
    expect(vm.fieldConfigByAttachment[17]).toBe(cached);
    expect(vm.loadingFieldConfig).toBe(false);
  });
});
