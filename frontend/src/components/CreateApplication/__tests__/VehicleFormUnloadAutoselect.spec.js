import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import VehicleForm from '../VehicleForm.vue';

// Автовыбор мест разгрузки, привязанных к организации/компании в Админке.
// Баг: org/company-эндпоинт отдаёт места без поля status (SELECT id/name/description),
// а форма фильтровала place.status === 'active' -> автовыбор был пуст (у людей работало
// из-за нормализации status в 'active'). Фикс - статус сверяем с allUnloadingPlaces.

// Полный список мест (как из /unload-places): id 1,3 - активны, 2 - на обслуживании.
const ALL_PLACES = [
    { id: 1, name: 'Ворота 1', status: 'active' },
    { id: 2, name: 'Ворота 2', status: 'maintenance' },
    { id: 3, name: 'Ворота 3', status: 'active' },
];
// Привязка организации #5 (как из /organizations/5/unload-places) - БЕЗ поля status.
const ORG_ATTACHED = [
    { id: 1, name: 'Ворота 1', description: null },
    { id: 2, name: 'Ворота 2', description: null },
];

vi.mock('@/api/client', () => ({
    apiRequest: vi.fn((url) => {
        if (url === '/unload-places') {
            return Promise.resolve({ ok: true, json: async () => ALL_PLACES });
        }
        if (url === '/organizations/5/unload-places') {
            return Promise.resolve({ ok: true, json: async () => ORG_ATTACHED });
        }
        return Promise.resolve({ ok: true, json: async () => [] });
    }),
}));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn(), enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

describe('VehicleForm - автовыбор мест разгрузки организации/компании', () => {
    beforeEach(() => { vi.clearAllMocks(); });

    it('activeAttachedIds: берёт только активные по allUnloadingPlaces', () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        w.vm.allUnloadingPlaces = ALL_PLACES;
        // id 2 - maintenance, исключается; 1 и 3 - активны.
        expect(w.vm.activeAttachedIds([{ id: 1 }, { id: 2 }, { id: 3 }])).toEqual([1, 3]);
    });

    it('activeAttachedIds: общий список пуст (гонка) -> возвращает все привязанные', () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        w.vm.allUnloadingPlaces = [];
        expect(w.vm.activeAttachedIds([{ id: 1 }, { id: 2 }])).toEqual([1, 2]);
    });

    it('при organizationId места разгрузки автоподставляются (только активные)', async () => {
        const w = mount(VehicleForm, {
            props: { userOrganizationId: 5 },
            attachTo: document.body,
        });
        await flushPromises();
        // Привязаны 1 и 2, но 2 на обслуживании -> выбирается только 1.
        expect(w.vm.selectedUnloadingPlaces).toEqual([1]);
    });

    it('без organizationId/companyId автоподстановки нет', async () => {
        const w = mount(VehicleForm, { props: {}, attachTo: document.body });
        await flushPromises();
        expect(w.vm.selectedUnloadingPlaces).toEqual([]);
    });
});
