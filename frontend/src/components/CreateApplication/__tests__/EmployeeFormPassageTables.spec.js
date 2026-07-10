import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { apiRequest } from '@/api/client';
import EmployeeForm from '../EmployeeForm.vue';

// #1036: места прохода/проезда НЕ выбираются автоматически по привязке к организации/
// компании и уведомление об этом не показывается. Привязанные таблицы лишь подсвечиваются
// (attachedTablesIds -> passage__item--attached), выбор делает пользователь.

const SYSTEM_TABLES = [
    { id: 20, name: 'passage-people', display_name: 'Проход для людей', table_type: 'people', status: 'active' },
];

const { notifyMock } = vi.hoisted(() => ({ notifyMock: vi.fn() }));

const defaultApiRequest = (url) => {
    if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
    return Promise.resolve({ ok: true, json: async () => [] });
};

vi.mock('@/api/client', () => ({ apiRequest: vi.fn((url) => defaultApiRequest(url)) }));
vi.mock('@/api/blacklist', () => ({ checkPersonBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: notifyMock, enqueue: vi.fn() })) }));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
    default: { name: 'ExistingEmployeesModal', template: '<div />' },
}));

describe('EmployeeForm - места прохода без автовыбора (#1036)', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
        apiRequest.mockImplementation(defaultApiRequest);
    });

    it('НЕ автовыбирает места прохода по организации и НЕ показывает уведомление', async () => {
        apiRequest.mockImplementation((url) => {
            if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
            if (url === '/organizations/7/tables') return Promise.resolve({
                ok: true,
                json: async () => [
                    { id: 20, name: 'passage-people', display_name: 'Проход для людей', table_type: 'people', status: 'active' },
                ],
            });
            return Promise.resolve({ ok: true, json: async () => [] });
        });

        const w = mount(EmployeeForm, {
            props: { userOrganizationId: 7, userOrganization: 'ООО Ромашка' },
            attachTo: document.body,
        });
        await flushPromises();

        // Таблица привязана к организации, но НЕ выбрана автоматически.
        expect(w.vm.selectedPassageTables).toEqual([]);
        expect(notifyMock).not.toHaveBeenCalledWith(
            expect.objectContaining({ prefix: expect.stringContaining('автоматически') }),
        );
    });

    it('НЕ автовыбирает места прохода по компании и НЕ показывает уведомление', async () => {
        apiRequest.mockImplementation((url) => {
            if (url === '/system-tables') return Promise.resolve({ ok: true, json: async () => SYSTEM_TABLES });
            if (url === '/companies/7/tables') return Promise.resolve({
                ok: true,
                json: async () => [
                    { id: 20, name: 'passage-people', display_name: 'Проход для людей', table_type: 'people', status: 'active' },
                ],
            });
            return Promise.resolve({ ok: true, json: async () => [] });
        });

        const w = mount(EmployeeForm, {
            props: { userCompanyId: 7, userCompany: 'ООО Компания' },
            attachTo: document.body,
        });
        await flushPromises();

        expect(w.vm.selectedPassageTables).toEqual([]);
        expect(notifyMock).not.toHaveBeenCalledWith(
            expect.objectContaining({ prefix: expect.stringContaining('автоматически') }),
        );
    });
});
