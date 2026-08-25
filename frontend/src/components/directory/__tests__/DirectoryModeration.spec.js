import { describe, it, expect, beforeEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import DirectoryModeration from '../DirectoryModeration.vue';
import ApplicationDetail from '@/components/ApplicationDetail/ApplicationDetail.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { useDeletionsStore } from '@/stores/deletions';
import {
  approveDirectoryEntry,
  renameDirectoryEntry,
  mergeDirectoryEntry,
  fetchApprovedDirectory,
} from '@/api/directory';

// Разбор организации/компании «на проверке»: сам компонент и его гейт в детали заявки (#1437, #1875).
vi.mock('@/api/directory', () => ({
  approveDirectoryEntry: vi.fn(),
  renameDirectoryEntry: vi.fn(),
  mergeDirectoryEntry: vi.fn(),
  fetchApprovedDirectory: vi.fn(),
  suggestOrganizations: vi.fn(),
  suggestCompanies: vi.fn(),
}));

// Деталь на mounted грузит вложения и помечает прочтение - к плашке это не относится.
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false }) }));
vi.mock('@/api/applications', () => ({ markAsRead: vi.fn().mockResolvedValue({}) }));

const MODERATE = 'application.organization.moderate';

function mountPanel(props = {}) {
  return mount(DirectoryModeration, {
    props: { kind: 'organization', entryId: 7, entryName: 'ООО Рмашка', ...props },
  });
}

const testid = (w, name) => w.find(`[data-testid="org-moderation-organization-${name}"]`);

describe('DirectoryModeration', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    fetchApprovedDirectory.mockResolvedValue([]);
  });

  it('подтверждает запись и отдаёт результат наверх', async () => {
    approveDirectoryEntry.mockResolvedValue({
      status: 'approved',
      entry: { id: 7, name: 'ООО Рмашка', moderation_status: 'approved' },
    });
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    const w = mountPanel();

    await testid(w, 'approve').trigger('click');
    await flush();

    expect(approveDirectoryEntry).toHaveBeenCalledWith('organization', 7);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success', bold: 'ООО Рмашка' }));
    expect(w.emitted('resolved')[0][0]).toEqual({ kind: 'organization', id: 7, name: 'ООО Рмашка' });
  });

  it('исправляет наименование введённым значением', async () => {
    renameDirectoryEntry.mockResolvedValue({
      status: 'renamed',
      entry: { id: 7, name: 'ООО Ромашка', moderation_status: 'approved' },
    });
    const w = mountPanel();

    await testid(w, 'rename-open').trigger('click');
    await testid(w, 'rename-input').setValue('  ООО Ромашка  ');
    await testid(w, 'rename-save').trigger('click');
    await flush();

    expect(renameDirectoryEntry).toHaveBeenCalledWith('organization', 7, 'ООО Ромашка');
    expect(w.emitted('resolved')[0][0]).toEqual({ kind: 'organization', id: 7, name: 'ООО Ромашка' });
  });

  it('столкновение наименований предлагает привязку к найденной записи', async () => {
    approveDirectoryEntry.mockResolvedValue({
      status: 'conflict',
      existing: { id: 12, name: 'ООО Ромашка', moderation_status: 'approved' },
      message: 'Организация с таким наименованием уже есть в справочнике',
    });
    mergeDirectoryEntry.mockResolvedValue({ target: { id: 12, name: 'ООО Ромашка' }, reassigned: {}, dropped_duplicates: {} });
    const w = mountPanel();

    await testid(w, 'approve').trigger('click');
    await flush();
    expect(testid(w, 'conflict').exists()).toBe(true);
    expect(w.emitted('resolved')).toBeUndefined();

    await testid(w, 'conflict-merge').trigger('click');
    await flush();

    expect(mergeDirectoryEntry).toHaveBeenCalledWith('organization', 7, 12);
    expect(w.emitted('resolved')[0][0]).toEqual({ kind: 'organization', id: 12, name: 'ООО Ромашка' });
  });

  it('в целях привязки не показывает саму разбираемую запись', async () => {
    fetchApprovedDirectory.mockResolvedValue([
      { id: 7, name: 'ООО Рмашка', moderation_status: 'pending' },
      { id: 12, name: 'ООО Ромашка', moderation_status: 'approved' },
      { id: 13, name: 'ЗАО Победа', moderation_status: 'approved' },
    ]);
    const w = mountPanel();

    await testid(w, 'merge-open').trigger('click');
    await flush();

    const names = w.findAll('[data-testid="org-moderation-organization-target"]').map(b => b.text());
    expect(names).toEqual(['ООО Ромашка', 'ЗАО Победа']);

    await testid(w, 'merge-search').setValue('победа');
    await w.vm.$nextTick();
    expect(w.findAll('[data-testid="org-moderation-organization-target"]').map(b => b.text())).toEqual(['ЗАО Победа']);
  });

  it('ошибку бэка показывает уведомлением и оставляет плашку на месте', async () => {
    approveDirectoryEntry.mockRejectedValue(new Error('Организация не найдена или находится в архиве'));
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    const w = mountPanel();

    await testid(w, 'approve').trigger('click');
    await flush();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error',
      bold: 'Организация не найдена или находится в архиве',
    }));
    expect(w.emitted('resolved')).toBeUndefined();
    expect(testid(w, 'approve').exists()).toBe(true);
  });
});

// Гейт плашки внутри детали заявки: право + статус записи.
const APP = {
  id: 1,
  application_number: 'A-1',
  sending_datetime: '2026-01-01T10:00:00Z',
  status: 'В работе',
  confirmation: 'Согласовано',
  organization_id: 7,
  organization_name: 'ООО Рмашка',
  organization_moderation_status: 'pending',
  company_id: null,
  company_moderation_status: null,
};

const detailStubs = {
  teleport: true,
  ForwardModal: true,
  ForwardMessages: true,
  ApplicationActionBar: true,
  ApplicationAttachments: true,
  ApplicationMessageModal: true,
  ApplicationAttachmentDetail: true,
  ApplicationConfirmation: true,
  ApplicationHistory: true,
  ApplicationQuestions: true,
  VehicleDetailsModal: true,
  EmployeeDetailsModal: true,
  BlacklistOverrideModal: true,
  Badge: true,
};

function mountDetail(application, allow = []) {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
  return mount(ApplicationDetail, {
    props: { application, currentUserId: 5, mode: 'center' },
    global: { stubs: detailStubs },
  });
}

const panels = w => w.findAllComponents(DirectoryModeration);

describe('ApplicationDetail: гейт плашки разбора', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    fetchApprovedDirectory.mockResolvedValue([]);
  });

  it('без права разбора плашки нет', () => {
    expect(panels(mountDetail(APP)).length).toBe(0);
  });

  it('с правом показывает плашку для записи на проверке', () => {
    const w = mountDetail(APP, [MODERATE]);
    expect(panels(w).length).toBe(1);
    expect(panels(w)[0].props()).toMatchObject({ kind: 'organization', entryId: 7, entryName: 'ООО Рмашка' });
  });

  it('проверенную организацию не предлагает разбирать', () => {
    const approved = { ...APP, organization_moderation_status: 'approved' };
    expect(panels(mountDetail(approved, [MODERATE])).length).toBe(0);
  });

  it('организацию и компанию на проверке показывает по отдельности', () => {
    const both = {
      ...APP,
      company_id: 3,
      company_name: 'ООО Компания',
      company_moderation_status: 'pending',
    };
    const w = mountDetail(both, [MODERATE]);
    expect(panels(w).map(p => p.props('kind'))).toEqual(['organization', 'company']);
  });

  it('разбор гасит плашку и отдаёт заявку наверх с новым наименованием', async () => {
    const w = mountDetail(APP, [MODERATE]);
    panels(w)[0].vm.$emit('resolved', { kind: 'organization', id: 12, name: 'ООО Ромашка' });
    await w.vm.$nextTick();

    expect(panels(w).length).toBe(0);
    const changed = w.emitted('application-changed').at(-1)[0];
    expect(changed).toMatchObject({
      organization_id: 12,
      organization_name: 'ООО Ромашка',
      organization_moderation_status: 'approved',
    });
  });

  // Заявка без организации показывает в шапке имя компании (COALESCE на бэке), поэтому
  // разбор компании обязан обновить и organization_name - иначе шапка врёт старым именем.
  it('разбор компании у заявки без организации правит и наименование в шапке', async () => {
    const companyOnly = {
      ...APP,
      organization_id: null,
      organization_moderation_status: null,
      organization_name: 'ООО Компашка',
      company_id: 3,
      company_name: 'ООО Компашка',
      company_moderation_status: 'pending',
    };
    const w = mountDetail(companyOnly, [MODERATE]);
    expect(panels(w).map(p => p.props('kind'))).toEqual(['company']);

    panels(w)[0].vm.$emit('resolved', { kind: 'company', id: 9, name: 'ООО Компания' });
    await w.vm.$nextTick();

    expect(panels(w).length).toBe(0);
    expect(w.emitted('application-changed').at(-1)[0]).toMatchObject({
      company_id: 9,
      company_name: 'ООО Компания',
      company_moderation_status: 'approved',
      organization_name: 'ООО Компания',
    });
  });
});

/** Даёт отработать цепочке промисов действия (запрос -> notify -> emit). */
async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}
