import { beforeEach, describe, expect, it, vi } from 'vitest';

const getFieldConfig = vi.hoisted(() => vi.fn());
vi.mock('@/api/attachment-templates', () => ({ getFieldConfig }));
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), apiRequestRaw: vi.fn() }));
vi.mock('exceljs', () => ({ default: {} }));
import CreateApplication from '../CreateApplication.vue';

const load = (vm, id) => CreateApplication.methods.loadFieldConfig.call(vm, id);
const state = () => ({ fieldConfigByAttachment: {}, customFieldDefinitions: {} });
beforeEach(() => { getFieldConfig.mockReset(); });

describe('field configuration caches successful reads and retries failed reads', () => {
  it('does not request a configuration without a template ID', async () => {
    await load(state(), undefined);
    expect(getFieldConfig).not.toHaveBeenCalled();
  });

  it('retains the configured base fields and additional fields', async () => {
    const vm = state();
    const custom = [{ key: 'project_code', label: 'Код работ', required: true }];
    getFieldConfig.mockResolvedValue({ base: [{ key: 'position', visible: true, required: false, locked: true, requirable: false }], custom });
    await load(vm, 17);
    expect(vm.fieldConfigByAttachment[17]).toEqual({ position: { visible: true, required: false, locked: true, requirable: false } });
    expect(vm.customFieldDefinitions[17]).toEqual(custom);
  });

  it('retries the same template after a temporary error and restores its configured fields', async () => {
    const vm = state();
    const custom = [{ key: 'project_code', label: 'Код работ', required: true }];
    getFieldConfig.mockRejectedValueOnce(new Error('Temporary read failure')).mockResolvedValueOnce({ base: [], custom });
    await load(vm, 17);
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(2);
    expect(vm.customFieldDefinitions[17]).toEqual(custom);
  });

  it('caches a successfully read empty configuration', async () => {
    const vm = state();
    getFieldConfig.mockResolvedValue({ base: [], custom: [] });
    await load(vm, 17);
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(1);
    expect(vm.fieldConfigByAttachment[17]).toEqual({});
    expect(vm.customFieldDefinitions[17]).toEqual([]);
  });

  it.each([
    { base: [{ key: 'position', visible: false, required: false, locked: true, requirable: false }] },
    { base: [] },
  ])('preserves a successful concurrent read when the other request fails ($base)', async ({ base }) => {
    const vm = state();
    const custom = [{ key: 'project_code', required: true }];
    let resolveFirst;
    let rejectSecond;
    getFieldConfig
      .mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((_, reject) => { rejectSecond = reject; }));
    const first = load(vm, 17);
    await vi.waitFor(() => expect(getFieldConfig).toHaveBeenCalledTimes(1));
    const second = load(vm, 17);
    await vi.waitFor(() => expect(getFieldConfig).toHaveBeenCalledTimes(2));
    resolveFirst({ base, custom });
    await first;
    const successful = vm.fieldConfigByAttachment[17];
    rejectSecond(new Error('Late temporary read failure'));
    await second;
    expect(vm.fieldConfigByAttachment[17]).toBe(successful);
    expect(vm.customFieldDefinitions[17]).toEqual(custom);
    await load(vm, 17);
    expect(getFieldConfig).toHaveBeenCalledTimes(2);
  });

  it('does not replace a different template configuration when a read fails', async () => {
    const vm = state();
    vm.fieldConfigByAttachment[11] = { position: { visible: true } };
    vm.customFieldDefinitions[11] = [{ key: 'project_code' }];
    getFieldConfig.mockRejectedValue(new Error('Temporary read failure'));
    await load(vm, 17);
    expect(vm.fieldConfigByAttachment[11]).toEqual({ position: { visible: true } });
    expect(vm.customFieldDefinitions[11]).toEqual([{ key: 'project_code' }]);
    expect(vm.fieldConfigByAttachment).not.toHaveProperty('17');
    expect(vm.customFieldDefinitions[17]).toEqual([]);
  });
});
