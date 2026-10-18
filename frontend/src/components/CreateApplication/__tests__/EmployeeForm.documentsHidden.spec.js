import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }) }));
vi.mock('@/api/blacklist', () => ({ checkPersonBlacklist: vi.fn().mockResolvedValue({ is_blacklisted: false }) }));
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';
import { toEmployeePayload } from '@/utils/applicationEntityPayload';
import { employeeDocumentErrors } from '@/utils/employeeDocuments';

const REQUIRED = { visible: true, required: true };
const CONFIG = { last_name: REQUIRED, first_name: REQUIRED, position: REQUIRED, citizenship: REQUIRED,
    passport: REQUIRED, patent: { visible: true, required: false }, target_tables: REQUIRED,
    pd_consent: { visible: false, required: false } };
const raw = { id: 7, last_name: 'Fixture', first_name: 'Employee', middle_name: '', position: 'Role',
    citizenship_id: 1, citizenship_name: 'Fixture', documents_hidden: true,
    passport_series_number: null, patent_number: null, other_permission: null, pd_consent_at: '2026-01-12T09:00:00Z' };

async function open(fieldConfig = CONFIG) {
    const w = mount(EmployeeForm, { props: { fieldConfig }, global: { stubs: { ExistingEmployeesModal: true } } });
    await flushPromises();
    await w.setData({ availableCitizenships: [{ id: 1, name: 'Fixture', patent_required: false }],
        selectedPassageTables: [11], allPassageTables: [{ table: { id: 11, display_name: 'Fixture table', table_type: 'people' } }] });
    return w;
}

describe('EmployeeForm document input for a new application', () => {
    beforeEach(() => { setActivePinia(createPinia()); apiRequest.mockClear(); });

    it('keeps hidden metadata when a catalogue row is selected and added', async () => {
        const w = await open();
        await w.setData({ selectedExistingEmployees: [raw] });
        expect(w.vm.canAddEmployee).toBe(true, 'catalogue selection itself only requires passage tables');
        w.vm.addEmployee();
        const row = w.emitted('employees-added')[0][0][0];
        expect(row).toMatchObject({ isExisting: true, existingEmployeeId: 7, documents_hidden: true,
            sourceDocumentsHidden: true, citizenshipPatentRequired: false, passportSeriesNumber: null, patentNumber: null, otherPermission: null });
        expect(toEmployeePayload([row])[0]).not.toHaveProperty('passport_series_number');
        expect(employeeDocumentErrors([row], CONFIG)[0]).toContain('для новой заявки');
        w.unmount();
    });

    it('allows new documents without editing the registry and preserves them on repeat editing', async () => {
        const w = await open();
        await w.setData({ selectedExistingEmployees: [raw] });
        w.vm.addEmployee();
        const selected = { ...w.emitted('employees-added')[0][0][0], id: 1 };
        w.vm.editEmployee(selected);
        await w.vm.$nextTick();
        expect(w.get('.completion__passport input').attributes('disabled')).toBeUndefined();
        expect(w.get('.completion__position input').attributes('disabled')).toBeDefined();
        expect(w.get('[data-testid="employee-draft-documents-info"]').text()).toContain('справочник не изменится');
        expect(w.vm.canAddEmployee).toBe(false, 'required passport must be entered for this application');
        await w.get('.completion__passport input').setValue('new application passport');
        expect(w.vm.canAddEmployee).toBe(true);
        w.vm.addEmployee();
        const updated = w.emitted('employee-updated')[0][0];
        expect(updated).toMatchObject({ isExisting: true, existingEmployeeId: 7, sourceDocumentsHidden: true,
            documents_hidden: false, passportSeriesNumber: 'new application passport' });
        expect(toEmployeePayload([updated])[0].passport_series_number).toBe('new application passport');
        expect(employeeDocumentErrors([updated], CONFIG)).toEqual([]);
        expect(raw.passport_series_number).toBeNull();
        w.vm.editEmployee(updated);
        await w.vm.$nextTick();
        expect(w.get('.completion__passport input').attributes('disabled')).toBeUndefined();
        expect(w.get('.completion__passport input').element.value).toBe('new application passport');
        await w.get('.completion__passport input').setValue('');
        expect(w.vm.canAddEmployee).toBe(false);
        expect(w.vm.preserveHiddenDocuments).toBe(true);
        await w.get('.completion__passport input').setValue('changed application passport');
        w.vm.addEmployee();
        expect(w.emitted('employee-updated')[1][0].passportSeriesNumber).toBe('changed application passport');
        expect(apiRequest.mock.calls.every(([, options]) => !['PUT', 'PATCH', 'POST'].includes(options?.method))).toBe(true);
        w.unmount();
    });
    it.each([false, true])('accepts required permission with hidden patent (citizenship patent required=%s)', async (patentRequired) => {
        const config = { ...CONFIG, patent: { visible: false, required: false }, work_permission: REQUIRED };
        const w = await open(config);
        const citizenship = { id: 1, name: 'Fixture', patent_required: patentRequired };
        await w.setData({ availableCitizenships: [citizenship], selectedExistingEmployees: [raw] });
        w.vm.addEmployee();
        w.vm.editEmployee({ ...w.emitted('employees-added')[0][0][0], id: 1 });
        await w.vm.$nextTick();
        await w.get('.completion__passport input').setValue('new passport');
        expect(w.find('.completion__patent').exists()).toBe(false);
        expect(w.vm.effectivePatentRequired).toBe(patentRequired);
        const permissionButton = w.get('.permission__dropdown-button');
        expect(permissionButton.attributes('disabled')).toBeUndefined();
        expect(w.vm.canAddEmployee).toBe(false);
        expect(w.vm.getTooltipMessage).toContain('иное разрешение');
        const permission = w.vm.availablePermissions[0];
        w.vm.selectPermission(permission);
        w.vm.selectCitizenship(citizenship);
        expect(w.vm.selectedPermission).toBe(permission);
        expect(w.vm.canAddEmployee).toBe(true);
        w.vm.addEmployee();
        const updated = w.emitted('employee-updated')[0][0];
        expect(updated).toMatchObject({ isExisting: true, existingEmployeeId: 7, sourceDocumentsHidden: true,
            documents_hidden: false, otherPermission: permission });
        expect(toEmployeePayload([updated])[0].other_permission).toBe(permission);
        expect(employeeDocumentErrors([updated], config)).toEqual([]);
        expect(raw.other_permission).toBeNull();
        w.vm.editEmployee(updated);
        await w.vm.$nextTick();
        expect(w.vm.selectedPermission).toBe(permission);
        expect(w.get('.permission__dropdown-button').attributes('disabled')).toBeUndefined();
        w.vm.selectPermission('');
        expect(w.vm.canAddEmployee).toBe(false);
        expect(apiRequest.mock.calls.every(([, options]) => !['PUT', 'PATCH', 'POST'].includes(options?.method))).toBe(true);
        w.unmount();
    });

    it('requires explicit permission even when a conditional patent was entered', async () => {
        const config = { ...CONFIG, work_permission: REQUIRED };
        const w = await open(config);
        await w.setData({ availableCitizenships: [{ id: 1, name: 'Fixture', patent_required: true }], selectedExistingEmployees: [raw] });
        w.vm.addEmployee();
        w.vm.editEmployee({ ...w.emitted('employees-added')[0][0][0], id: 1 });
        await w.vm.$nextTick();
        await w.get('.completion__passport input').setValue('new passport');
        await w.get('.completion__patent input').setValue('new patent');
        expect(w.vm.canAddEmployee).toBe(false);
        expect(w.get('.permission__dropdown-button').attributes('disabled')).toBeUndefined();
        w.vm.selectPermission(w.vm.availablePermissions[0]);
        expect(w.vm.patentNumber).toBe('');
        expect(w.vm.canAddEmployee).toBe(true);
        w.unmount();
    });

    it('carries citizenship patent requirements through catalogue selection and new input', async () => {
        const w = await open();
        await w.setData({ availableCitizenships: [{ id: 1, name: 'Fixture', patent_required: true }], selectedExistingEmployees: [raw] });
        w.vm.addEmployee();
        const row = { ...w.emitted('employees-added')[0][0][0], id: 1 };
        expect(row.citizenshipPatentRequired).toBe(true);
        w.vm.editEmployee(row);
        await w.vm.$nextTick();
        await w.get('.completion__passport input').setValue('new passport');
        expect(w.vm.canAddEmployee).toBe(false, 'required:false patent still depends on citizenship');
        await w.get('.completion__patent input').setValue('new patent');
        expect(w.vm.canAddEmployee).toBe(true);
        w.vm.addEmployee();
        const updated = w.emitted('employee-updated')[0][0];
        expect(updated.citizenshipPatentRequired).toBe(true);
        expect(employeeDocumentErrors([updated], CONFIG)).toEqual([]);
        expect(toEmployeePayload([updated])[0].patent_number).toBe('new patent');
        w.unmount();
    });

});
