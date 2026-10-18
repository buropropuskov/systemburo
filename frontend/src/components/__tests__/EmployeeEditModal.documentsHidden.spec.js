import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }));
import EmployeeEditModal from '../EmployeeEditModal.vue';

const employee = {
    id: 7, last_name: 'Fixture', first_name: 'Employee', middle_name: '', position: 'Original role',
    citizenship_id: 1, organization_id: 55, company_id: null, user_id: 42,
    documents_hidden: true, passport_series_number: null, patent_number: null, other_permission: null,
    pd_consent_at: '2026-01-12T09:00:00Z',
};
const ownership = { user_id: 1, has_organization: true, has_company: false, organization_id: 10, company_id: null };
const citizenships = [{ id: 1, name: 'Fixture', patent_required: true }];

function open(fields = {}) {
    return mount(EmployeeEditModal, {
        props: { visible: true, editingEmployee: { ...employee, ...fields }, foreignRecord: true, ownershipInfo: ownership, citizenships },
        global: { stubs: { teleport: true } },
    });
}

describe('EmployeeEditModal hidden document updates', () => {
    beforeEach(() => { setActivePinia(createPinia()); apiRequest.mockReset(); });

    it('does not treat unread nulls as changes or missing mandatory documents', () => {
        const w = open();
        expect(w.vm.hasChanges()).toBe(false);
        expect(w.vm.canSaveEmployee).toBe(true);
        expect(w.vm.saveEmployeeHint).toBe('');
        expect(w.find('.completion__passport').exists()).toBe(false);
        expect(w.find('.completion__permission').exists()).toBe(false);
        expect(w.get('[data-testid="employee-documents-hidden"]').text()).toContain('Документы скрыты');
        w.unmount();
    });

    it('saves a neighbouring field without document keys and emits the authoritative response', async () => {
        const saved = { ...employee, position: 'Updated role' };
        apiRequest.mockResolvedValue({ ok: true, json: async () => saved });
        const w = open();
        await w.setData({ position: 'Updated role' });
        expect(w.vm.hasChanges()).toBe(true);
        await w.vm.saveEmployee();
        expect(apiRequest).toHaveBeenCalledTimes(1);
        const [url, options] = apiRequest.mock.calls[0];
        expect(url).toBe('/unique-employees/7');
        expect(options.method).toBe('PUT');
        const body = JSON.parse(options.body);
        expect(body.position).toBe('Updated role');
        for (const key of ['passport_series_number', 'patent_number', 'other_permission', 'documents_hidden']) expect(body).not.toHaveProperty(key);
        expect(w.emitted('saved')).toEqual([[saved]]);
        expect(w.vm.hasChanges()).toBe(false);
        w.unmount();
    });

    it('keeps visible document editing and its existing validation', async () => {
        const w = open({ documents_hidden: false });
        expect(w.find('.completion__passport').exists()).toBe(true);
        expect(w.vm.canSaveEmployee).toBe(false);
        await w.setData({ passportSeriesNumber: 'new passport', patentNumber: 'new patent', position: 'Updated role' });
        expect(w.vm.canSaveEmployee).toBe(true);
        apiRequest.mockResolvedValue({ ok: true, json: async () => ({ ...employee, documents_hidden: false }) });
        await w.vm.saveEmployee();
        const body = JSON.parse(apiRequest.mock.calls[0][1].body);
        expect(body.passport_series_number).toBe('new passport');
        expect(body.patent_number).toBe('new patent');
        expect(body).not.toHaveProperty('documents_hidden');
        w.unmount();
    });
});
