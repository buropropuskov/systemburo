import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ManualAttachExistingModal from '../ManualAttachExistingModal.vue';
import BaseDropdown from '../ui/BaseDropdown.vue';
import DateRangeSection from '../CreateApplication/DateRangeSection.vue';
import { getManualAttachContext, getManualAttachAttachments, previewManualAttachSingle, executeManualAttachSingle } from '@/api/manualAttachSingle';
import { getAttachableApplications } from '@/api/applications';
import { useDeletionsStore } from '@/stores/deletions';

vi.mock('@/api/manualAttachSingle', () => ({ getManualAttachContext: vi.fn(), getManualAttachAttachments: vi.fn(), previewManualAttachSingle: vi.fn(), executeManualAttachSingle: vi.fn() }));
vi.mock('@/api/applications', () => ({ getAttachableApplications: vi.fn() }));
const wrappers = [];
const CURRENT_FLAGS = { roof_access: true, free_parking: false, individual_roof_access: false, individual_free_parking: false };
const NEW_FLAGS = { roof_access: true, free_parking: true, individual_roof_access: true, individual_free_parking: false };
const PERIOD = { entry_date_from: '2099-10-01', entry_date_to: '2099-10-03', entry_time_from: '08:00:00', entry_time_to: '22:00:00' };
const APPS = [{ id: 42, application_number: 'SYNTHETIC-42', status: 'В работе', confirmation: 'Согласовано' }];
const ATTACHMENTS = [
  { id: 8, attachment_type: 'cars', attachment_display_name: 'Car attachment', status: 1, is_manual: false, ...PERIOD },
  { id: 9, attachment_type: 'people', attachment_display_name: 'People attachment', status: 1, is_manual: false, ...PERIOD },
  { id: 10, attachment_type: 'cars', status: 0, is_manual: false, ...PERIOD },
  { id: 11, attachment_type: 'cars', status: 1, is_manual: true, ...PERIOD },
];
const context = (bounded = false, id = 17) => ({ entity_id: id, entity_kind: 'car', attachment_id: 5, is_manual: true, application_id: null, flags: CURRENT_FLAGS,
  period_mode: bounded ? 'individual' : 'inherit', effective_period: { ...PERIOD, bounded, source: bounded ? 'individual' : 'manual_unbounded' },
  requires_period_choice: !bounded, can_assign_period: true });
const result = (patch = {}) => ({ entity_id: 17, entity_kind: 'car', old_attachment_id: 5, application_id: 42,
  current_flags: CURRENT_FLAGS, new_flags: NEW_FLAGS,
  destination_attachment_id: null, destination_mode: 'new_attachment', current_effective: { bounded: false },
  new_effective: { ...PERIOD, bounded: true }, current_mode: 'inherit', new_mode: 'inherit', needs_period_choice: true, revision: 'a'.repeat(64), ...patch });
function create(props = {}) {
  const wrapper = mount(ManualAttachExistingModal, { props: { show: true, kind: 'car', entityID: 17, tableID: 4, ...props },
    global: { stubs: { Teleport: true } }, attachTo: document.body });
  wrappers.push(wrapper);
  return wrapper;
}
async function choose(wrapper, { source = true, existing = false } = {}) {
  await flushPromises();
  const appDropdown = wrapper.findAllComponents(BaseDropdown).find(d => d.attributes('data-testid') === 'manual-attach-application');
  appDropdown.vm.$emit('update:modelValue', 42);
  await flushPromises();
  if (existing) await wrapper.find('input[value="existing"]').setValue();
  if (existing) {
    wrapper.findAllComponents(BaseDropdown).find(d => d.attributes('data-testid') === 'manual-attach-target').vm.$emit('update:modelValue', 8);
  }
  if (source) {
    await wrapper.find('input[value="source"]').setValue();
    wrapper.findAllComponents(BaseDropdown).find(d => d.attributes('data-testid') === 'manual-attach-source').vm.$emit('update:modelValue', 9);
  }
  await wrapper.find('[data-testid="manual-attach-reason"]').setValue('  Synthetic reason  ');
  await wrapper.vm.$nextTick();
}
async function preview(wrapper, data = result()) {
  previewManualAttachSingle.mockResolvedValueOnce(data);
  await wrapper.find('[data-testid="manual-attach-preview-button"]').trigger('click');
  await flushPromises();
}

describe('single manual attach real modal #2665', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2099-09-30T12:00:00Z'));
    setActivePinia(createPinia());
    getManualAttachContext.mockReset().mockResolvedValue(context());
    getAttachableApplications.mockReset().mockResolvedValue(APPS);
    getManualAttachAttachments.mockReset().mockResolvedValue(ATTACHMENTS);
    previewManualAttachSingle.mockReset();
    executeManualAttachSingle.mockReset();
  });
  afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers(); });
  it('uses project components, starts without focus/open chooser and requires an explicit period choice', async () => {
    const wrapper = create();
    await flushPromises();
    expect(getManualAttachContext).toHaveBeenCalledWith('car', 17, 4);
    expect(wrapper.findAllComponents(BaseDropdown).every(d => !d.vm.isOpen)).toBe(true);
    expect(wrapper.find('[autofocus]').exists()).toBe(false);
    expect(document.activeElement?.tagName).toBe('BODY');
    await choose(wrapper, { source: false });
    expect(wrapper.vm.periodChoice).toBe('');
    expect(wrapper.find('[data-testid="manual-attach-preview-button"]').attributes('disabled')).toBeDefined();
    expect(previewManualAttachSingle).not.toHaveBeenCalled();
  });
  it('previews then saves only the actual entity with the reviewed revision; emits a refresh event', async () => {
    const wrapper = create();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    await choose(wrapper);
    await preview(wrapper);
    const body = { application_id: 42, reason: 'Synthetic reason', period_choice: 'source', source_attachment_id: 9 };
    expect(previewManualAttachSingle).toHaveBeenCalledWith('car', 17, 4, body);
    expect(executeManualAttachSingle).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain('Признаки до привязки');
    expect(wrapper.text()).toContain('Признаки после привязки');
    expect(wrapper.text()).toContain('Да — собственный признак');
    expect(wrapper.text()).toContain('Да — из вложения');
    executeManualAttachSingle.mockResolvedValue(result({ destination_attachment_id: 25 }));
    await wrapper.find('[data-testid="manual-attach-save"]').trigger('click');
    await flushPromises();
    expect(executeManualAttachSingle).toHaveBeenCalledWith('car', 17, 4, { ...body, expected_revision: 'a'.repeat(64) });
    expect(wrapper.emitted('changed')).toEqual([[result({ destination_attachment_id: 25 })]]);
    expect(wrapper.emitted('close')).toHaveLength(1);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
  });
  it('existing target uses compatible active types, source can be another type, and explains target common places', async () => {
    const wrapper = create();
    await choose(wrapper, { existing: true });
    expect(getManualAttachAttachments).toHaveBeenCalledWith('car', 17, 4, 42);
    expect(wrapper.vm.targetOptions.map(a => a.id)).toEqual([8]);
    expect(wrapper.vm.sourceOptions.map(a => a.id)).toEqual([8, 9]);
    expect(wrapper.text()).toContain('Машина получит общие места разгрузки целевого вложения');
    await preview(wrapper, result({ destination_mode: 'existing_attachment', destination_attachment_id: 8, new_mode: 'individual' }));
    expect(previewManualAttachSingle).toHaveBeenCalledWith('car', 17, 4, {
      target_attachment_id: 8, reason: 'Synthetic reason', period_choice: 'source', source_attachment_id: 9,
    });
  });
  it('cannot preview even a finite record if narrow metadata is denied; never falls back to general attachments', async () => {
    getManualAttachContext.mockResolvedValue(context(true));
    getManualAttachAttachments.mockRejectedValueOnce(Object.assign(new Error('Metadata denied'), { status: 403 }));
    const wrapper = create();
    await choose(wrapper, { source: false });
    expect(wrapper.text()).toContain('Metadata denied');
    expect(wrapper.vm.attachmentsReady).toBe(false);
    expect(wrapper.vm.canPreview).toBe(false);
    expect(previewManualAttachSingle).not.toHaveBeenCalled();
  });
  it('preserves a finite individual without demanding detail permission or sending a hidden replacement', async () => {
    getManualAttachContext.mockResolvedValue({ ...context(true), can_assign_period: false });
    const wrapper = create();
    await choose(wrapper, { source: false });
    expect(wrapper.find('input[value="source"]').exists()).toBe(false);
    expect(wrapper.text()).toContain('Конечный срок открытой записи сохранится');
    await preview(wrapper, result({ current_mode: 'individual', new_mode: 'individual', needs_period_choice: false }));
    expect(previewManualAttachSingle).toHaveBeenCalledWith('car', 17, 4, { application_id: 42, reason: 'Synthetic reason' });
  });
  it('blocks unbounded assignment when server denies detail permission', async () => {
    getManualAttachContext.mockResolvedValue({ ...context(), can_assign_period: false });
    const wrapper = create();
    await choose(wrapper, { source: false });
    expect(wrapper.text()).toContain('нужно право изменения срока записи');
    expect(wrapper.vm.canPreview).toBe(false);
    expect(previewManualAttachSingle).not.toHaveBeenCalled();
  });
  it('reuses DateRangeSection and validates all four manual date fields before preview', async () => {
    const wrapper = create();
    await choose(wrapper, { source: false });
    await wrapper.find('input[value="individual"]').setValue();
    const section = wrapper.findComponent(DateRangeSection);
    section.vm.$emit('update:start-time', '');
    await wrapper.vm.$nextTick();
    await wrapper.find('[data-testid="manual-attach-preview-button"]').trigger('click');
    expect(wrapper.vm.dateErrors.startTime).toBeTruthy();
    expect(previewManualAttachSingle).not.toHaveBeenCalled();
    section.vm.$emit('update:start-time', '09:30');
    await wrapper.vm.$nextTick();
    await preview(wrapper, result({ new_mode: 'individual' }));
    expect(previewManualAttachSingle).toHaveBeenCalledWith('car', 17, 4, {
      application_id: 42, reason: 'Synthetic reason', period_choice: 'individual', period: { ...PERIOD, entry_time_from: '09:30:00' },
    });
  });
  it('invalidates a reviewed preview after input changes', async () => {
    const wrapper = create();
    await choose(wrapper);
    await preview(wrapper);
    await wrapper.find('[data-testid="manual-attach-reason"]').setValue('Different reason');
    expect(wrapper.vm.preview).toBeNull();
    expect(wrapper.find('[data-testid="manual-attach-save"]').exists()).toBe(false);
  });
  it('409 requires explicit refresh and a new preview without automatic retry', async () => {
    const wrapper = create();
    await choose(wrapper);
    await preview(wrapper);
    executeManualAttachSingle.mockRejectedValueOnce(Object.assign(new Error('Target changed'), { status: 409 }));
    await wrapper.find('[data-testid="manual-attach-save"]').trigger('click');
    await flushPromises();
    expect(wrapper.vm.conflict).toBe(true);
    expect(executeManualAttachSingle).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(wrapper.text()).toContain('автоматически не повторяется');
    await wrapper.find('[data-testid="manual-attach-refresh"]').trigger('click');
    await flushPromises();
    expect(getManualAttachContext).toHaveBeenCalledTimes(2);
    expect(wrapper.vm.applicationID).toBeNull();
    expect(wrapper.vm.preview).toBeNull();
    expect(executeManualAttachSingle).toHaveBeenCalledTimes(1);
  });
  it('rejects a mismatched preview and never enables save', async () => {
    const wrapper = create();
    await choose(wrapper);
    await preview(wrapper, result({ entity_id: 18 }));
    expect(wrapper.vm.preview).toBeNull();
    expect(wrapper.text()).toContain('подтверждение привязки');
    expect(executeManualAttachSingle).not.toHaveBeenCalled();
  });
  it('blocks closing and double submit while saving', async () => {
    const wrapper = create();
    await choose(wrapper);
    await preview(wrapper);
    let resolve;
    executeManualAttachSingle.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    await wrapper.find('[data-testid="manual-attach-save"]').trigger('click');
    wrapper.vm.close();
    await wrapper.vm.save();
    expect(executeManualAttachSingle).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('close')).toBeUndefined();
    resolve(result({ destination_attachment_id: 25 }));
    await flushPromises();
    expect(wrapper.emitted('changed')).toHaveLength(1);
  });
  it('ignores stale attachments after application changes', async () => {
    const wrapper = create();
    await flushPromises();
    let resolve;
    getManualAttachAttachments.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    const old = wrapper.vm.selectApplication(42);
    getManualAttachAttachments.mockResolvedValueOnce([{ ...ATTACHMENTS[0], id: 18 }]);
    await wrapper.vm.selectApplication(43);
    resolve(ATTACHMENTS);
    await old;
    expect(wrapper.vm.attachments.map(a => a.id)).toEqual([18]);
  });
  it('clears the explicit period choice and reviewed preview when changing application', async () => {
    const wrapper = create();
    await choose(wrapper, { existing: true });
    await preview(wrapper, result({ destination_mode: 'existing_attachment', destination_attachment_id: 8 }));
    await wrapper.vm.selectApplication(43);
    expect(wrapper.vm.periodChoice).toBe('');
    expect(wrapper.vm.targetID).toBeNull();
    expect(wrapper.vm.sourceID).toBeNull();
    expect(wrapper.vm.preview).toBeNull();
    expect(executeManualAttachSingle).not.toHaveBeenCalled();
  });
  it('ignores late preview after close and late context after identity switch', async () => {
    const wrapper = create();
    await choose(wrapper);
    let resolve;
    previewManualAttachSingle.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    await wrapper.find('[data-testid="manual-attach-preview-button"]').trigger('click');
    wrapper.vm.close();
    resolve(result());
    await flushPromises();
    expect(wrapper.vm.preview).toBeNull();
    expect(wrapper.emitted('changed')).toBeUndefined();
    getManualAttachContext.mockResolvedValueOnce(context(false, 18));
    await wrapper.setProps({ entityID: 18 });
    await flushPromises();
    expect(wrapper.vm.context.entity_id).toBe(18);
  });
  it('does not emit an old saved entity into a newly opened card', async () => {
    const wrapper = create();
    await choose(wrapper);
    await preview(wrapper);
    let resolve;
    executeManualAttachSingle.mockReturnValueOnce(new Promise(done => { resolve = done; }));
    await wrapper.find('[data-testid="manual-attach-save"]').trigger('click');
    getManualAttachContext.mockResolvedValueOnce(context(false, 18));
    await wrapper.setProps({ entityID: 18 });
    await flushPromises();
    resolve(result({ destination_attachment_id: 25 }));
    await flushPromises();
    expect(wrapper.emitted('changed')).toBeUndefined();
    expect(wrapper.vm.context.entity_id).toBe(18);
    expect(wrapper.vm.saving).toBe(false);
  });
});
