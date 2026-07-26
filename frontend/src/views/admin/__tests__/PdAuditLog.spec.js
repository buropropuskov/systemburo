import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const listPDAudit = vi.fn();
vi.mock('@/api/pd-audit', () => ({ listPDAudit: (...a) => listPDAudit(...a) }));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

import PdAuditLog from '../PdAuditLog.vue';

function row(over = {}) {
  return {
    id: 1,
    user_id: 7,
    username: 'kafanova',
    user_name: 'Кафанова Мария',
    action: 'view',
    resource: 'attachment_blank',
    ip_address: '10.0.0.5',
    method: 'GET',
    path: '/api/applications/89/blank',
    status_code: 200,
    created_at: '2026-07-26T09:15:00Z',
    ...over,
  };
}

function page(items, over = {}) {
  return {
    items, total: items.length, page: 1, limit: 50, ...over,
  };
}

async function mountLog() {
  const wrapper = mount(PdAuditLog, {
    global: { stubs: { RefreshButton: true, BaseDropdown: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('PdAuditLog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPDAudit.mockResolvedValue(page([row()]));
  });

  it('показывает запись человеческим языком, а не технической строкой', async () => {
    const wrapper = await mountLog();
    const text = wrapper.text();
    expect(text).toContain('Кафанова Мария');
    expect(text).toContain('Просмотр');
    expect(text).toContain('Выгрузка бланка');
    expect(text).toContain('Просмотрено');
    expect(text).toContain('10.0.0.5');
  });

  it('отличает отказ от состоявшегося просмотра', async () => {
    listPDAudit.mockResolvedValue(page([row({ status_code: 403 })]));
    const wrapper = await mountLog();
    expect(wrapper.text()).toContain('Отказано');
    expect(wrapper.find('.badge--warning').exists()).toBe(true);
  });

  it('шлёт фильтры и сбрасывает страницу на первую', async () => {
    // страниц должно быть больше одной, иначе «Вперёд» заблокирована
    listPDAudit.mockResolvedValue(page([row()], { total: 120, limit: 50 }));
    const wrapper = await mountLog();
    await wrapper.find('[data-testid="pda-next"]').trigger('click');
    await flushPromises();
    expect(listPDAudit).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }));

    await wrapper.find('[data-testid="pda-filter-username"]').setValue('kafanova');
    await wrapper.find('[data-testid="pda-filter-denied"]').setValue(true);
    await wrapper.find('[data-testid="pda-apply"]').trigger('submit');
    await flushPromises();

    expect(listPDAudit).toHaveBeenLastCalledWith(expect.objectContaining({
      page: 1, username: 'kafanova', only_denied: true,
    }));
  });

  it('сообщает об ошибке загрузки и не оставляет старые строки', async () => {
    const wrapper = await mountLog();
    expect(wrapper.findAll('[data-testid="pda-row"]')).toHaveLength(1);

    listPDAudit.mockRejectedValue(new Error('Недостаточно прав'));
    await wrapper.find('[data-testid="pda-apply"]').trigger('submit');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error', bold: 'Недостаточно прав',
    }));
    expect(wrapper.findAll('[data-testid="pda-row"]')).toHaveLength(0);
    expect(wrapper.text()).toContain('Записей нет');
  });

  it('листает по страницам исходя из общего числа', async () => {
    listPDAudit.mockResolvedValue(page([row()], { total: 120, limit: 50 }));
    const wrapper = await mountLog();
    expect(wrapper.text()).toContain('Всего: 120');
    expect(wrapper.text()).toContain('Стр. 1 / 3');
  });
});
