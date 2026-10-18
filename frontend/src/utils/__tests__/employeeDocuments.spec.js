import { describe, it, expect } from 'vitest';
import { employeeDocumentFields, employeeDocumentPayload, employeeDocumentErrors } from '../employeeDocuments';
import { toEmployeePayload } from '../applicationEntityPayload';

const hidden = { documents_hidden: true, passport_series_number: null, patent_number: null, other_permission: null };

describe('employee documents response and request contract', () => {
    it('keeps hidden nulls and marker through both naming conventions', () => {
        const form = employeeDocumentFields(hidden, { toForm: true });
        expect(form).toEqual({ documents_hidden: true, sourceDocumentsHidden: true, passportSeriesNumber: null, patentNumber: null, otherPermission: null });
        expect(employeeDocumentFields(form, { fromForm: true })).toEqual(hidden);
    });

    it('does not expose stale values when the authoritative marker is hidden', () => {
        const stale = { ...hidden, passport_series_number: 'stale fixture' };
        expect(employeeDocumentFields(stale)).toEqual(hidden);
        expect(stale.passport_series_number).toBe('stale fixture');
    });

    it('preserves explicit visible marker and absent legacy marker', () => {
        const visible = { documents_hidden: false, passport_series_number: 'fixture passport', patent_number: null, other_permission: null };
        expect(employeeDocumentFields(visible)).toEqual(visible);
        expect(employeeDocumentFields({ passport_series_number: 'legacy' })).not.toHaveProperty('documents_hidden');
        expect(employeeDocumentPayload(visible)).toEqual({ passport_series_number: 'fixture passport', patent_number: null, other_permission: null });
    });

    it('omits unread documents and the response-only marker from requests', () => {
        expect(employeeDocumentPayload(hidden)).toEqual({});
        const form = { ...employeeDocumentFields(hidden, { toForm: true }), lastName: 'Fixture', firstName: 'Employee', position: 'Role', citizenshipId: 1 };
        const payload = toEmployeePayload([form])[0];
        expect(payload.last_name).toBe('Fixture');
        for (const key of ['passport_series_number', 'patent_number', 'other_permission', 'documents_hidden']) expect(payload).not.toHaveProperty(key);
    });

    it('keeps newly entered local documents with no inherited hidden marker', () => {
        const payload = toEmployeePayload([{ passportSeriesNumber: 'new fixture', patentNumber: null, otherPermission: 'new permit' }])[0];
        expect(payload.passport_series_number).toBe('new fixture');
        expect(payload.other_permission).toBe('new permit');
        expect(payload).not.toHaveProperty('documents_hidden');
    });
    it('validates required template fields before final submission without changing draft input', () => {
        const form = { ...employeeDocumentFields(hidden, { toForm: true }), lastName: 'Fixture', firstName: 'Employee' };
        const config = { passport: { visible: true, required: true } };
        expect(employeeDocumentErrors([form], config)[0]).toContain('паспортные данные для новой заявки');
        expect(form.passportSeriesNumber).toBeNull();
        const entered = { ...form, documents_hidden: false, passportSeriesNumber: 'new fixture' };
        expect(employeeDocumentErrors([entered], config)).toEqual([]);
        expect(entered.passportSeriesNumber).toBe('new fixture');
        expect(employeeDocumentErrors([form], { passport: { visible: true, required: false } })).toEqual([]);
    });

    it('matches default passport and conditional patent rules of the entry form', () => {
        const form = { ...employeeDocumentFields(hidden, { toForm: true }), citizenshipPatentRequired: true };
        expect(employeeDocumentErrors([form], {}).join(' ')).toContain('паспортные данные');
        expect(employeeDocumentErrors([form], {}).join(' ')).toContain('патент или иное разрешение');
        const entered = { ...form, documents_hidden: false, passportSeriesNumber: 'new fixture' };
        const config = { passport: { visible: true, required: false }, patent: { visible: true, required: false } };
        expect(employeeDocumentErrors([entered], config).join(' ')).toContain('патент или иное разрешение');
        expect(employeeDocumentErrors([{ ...entered, otherPermission: 'new permit' }], config)).toEqual([]);
        expect(employeeDocumentErrors([{ ...entered, citizenshipPatentRequired: false }], config)).toEqual([]);
    });

});
