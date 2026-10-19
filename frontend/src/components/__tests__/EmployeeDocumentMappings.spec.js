import { describe, it, expect, vi } from 'vitest';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(), apiRequestRaw: vi.fn() }));
vi.mock('exceljs', () => ({ default: {} }));
import { peopleTablePeriodDetails, employeesRegistryPeriodDetails, applicationPeriodDetails } from '../entityPeriodParentMixins';
import EmployeesList from '../CreateApplication/EmployeesList.vue';
import ExistingEmployeesModal from '../CreateApplication/ExistingEmployeesModal.vue';
import CreateApplication from '../CreateApplication/CreateApplication.vue';
import { employeeDocumentFields } from '@/utils/employeeDocuments';

const raw = { id: 7, documents_hidden: true, passport_series_number: null, patent_number: null, other_permission: null };
const expectHidden = (value) => expect(value).toMatchObject(raw);

describe('employee document metadata across view transformations', () => {
    it.each([
        ['table', peopleTablePeriodDetails, 'openEmployeeDetails', 'selectedEmployee'],
        ['registry', employeesRegistryPeriodDetails, 'openEmployeeDetails', 'detailsEmployee'],
        ['application', applicationPeriodDetails, 'openEmployeeModal', 'selectedEmployee'],
    ])('%s card retains authoritative hidden state', (_name, mixin, method, property) => {
        const vm = { applicationData: { id: 2 } };
        mixin.methods[method].call(vm, raw);
        expectHidden(vm[property]);
    });

    it('catalogue selection preserves the full response instead of normalising hidden nulls', () => {
        const emit = vi.fn();
        ExistingEmployeesModal.methods.confirmSelection.call({ tempSelectedEmployees: [raw], $emit: emit });
        expect(emit).toHaveBeenCalledWith('employees-selected', [raw]);
    });

    it('local list converts camel case without losing the server marker', () => {
        const vm = { detailInfo: {} };
        EmployeesList.methods.showEmployeeDetails.call(vm, { id: raw.id, ...employeeDocumentFields(raw, { toForm: true }) });
        expectHidden(vm.selectedEmployee);
    });

    it('final submission reports required documents while preserving locally entered values', () => {
        const row = { ...employeeDocumentFields(raw, { toForm: true }), lastName: 'Fixture', firstName: 'Employee' };
        const vm = { attachments: [{ id: 2, attachment_type: 'people', display_name: 'Fixture attachment' }],
            organization: 'Fixture', company: '', responsiblePerson: 'Fixture', phoneNumber: '+70000000000', consentGiven: true,
            fieldConfigByAttachment: { 2: { passport: { visible: true, required: true } } },
            attachmentKey: () => 2, rowsForAttachment: () => [row], attachmentDatesByAttachment: { 2: {} },
            attachmentDatesComplete: () => true, emptyRequiredCustomFields: () => [] };
        expect(CreateApplication.computed.submitValidation.call(vm).join(' ')).toContain('паспортные данные для новой заявки');
        row.documents_hidden = false;
        row.passportSeriesNumber = 'new application passport';
        expect(CreateApplication.computed.submitValidation.call(vm)).toEqual([]);
        expect(row.passportSeriesNumber).toBe('new application passport');
    });
    it('parent catalogue handler retains identity and never offers registry binding for draft documents', async () => {
        const row = { id: 7, isExisting: true, existingEmployeeId: 77, documents_hidden: false, sourceDocumentsHidden: true,
            lastName: 'Fixture', firstName: 'Employee', position: 'Role', passportSeriesNumber: 'new application passport' };
        const vm = { selectedAttachment: { id: 2, attachment_type: 'people' }, attachments: [{ id: 2, attachment_type: 'people' }],
            employeeIdCounter: 1, employeesByAttachment: {}, vehiclesByAttachment: {}, attachmentKey: () => 2,
            canCreateRegistry: vi.fn().mockReturnValue(true),
            saveToLocalStorage: vi.fn(), loadExistingVehicles: vi.fn().mockResolvedValue([]), loadExistingEmployees: vi.fn().mockResolvedValue([]) };
        CreateApplication.methods.handleEmployeesAdded.call(vm, [row]);
        expect(vm.employeesByAttachment[2][0]).toMatchObject({ isExisting: true, existingEmployeeId: 77, sourceDocumentsHidden: true, passportSeriesNumber: 'new application passport' });
        await CreateApplication.methods.collectNewDataForBinding.call(vm);
        expect(vm.newEmployeesToBind).toEqual([]);
        CreateApplication.methods.handleEmployeeAdded.call(vm, row);
        expect(vm.employeesByAttachment[2][1].isExisting).toBe(true);
        await CreateApplication.methods.collectNewDataForBinding.call(vm);
        expect(vm.newEmployeesToBind).toEqual([]);
    });

});
