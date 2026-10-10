import { beforeEach, describe, expect, it, vi } from 'vitest';
import { loadHistoryEntity } from '../historyEntity';
import { apiRequest } from '../client';
vi.mock('../client', () => ({ apiRequest: vi.fn() }));
const response = (data, status = 200) => ({ ok: status === 200, json: async () => data });
describe('current history card visibility #2674', () => {
  beforeEach(() => vi.clearAllMocks());
  it('opens only the exact current ID from the authorized table, not the old name snapshot', async () => {
    apiRequest.mockResolvedValue(response([{ id: 18, car_number: 'OLD' }, { id: 17, car_number: 'CURRENT', car_brand: 'Fixture', entry_date_to: null }]));
    expect(await loadHistoryEntity('car', {car_id:17,car_number:'OLD',table_id:4})).toMatchObject({id:17,plateNumber:'CURRENT',entry_date_to:null});
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it.each([{car_id:'17'}, {car_id:17,entity_deleted:true}, {car_number:'OLD'}])('does not search for missing/deleted IDs', async row => {
    await expect(loadHistoryEntity('car', row)).rejects.toThrow('недоступна');
    expect(apiRequest).not.toHaveBeenCalled();
  });
  it('does not bypass a denied current table read by trying another scope', async () => {
    apiRequest.mockResolvedValue(response(null,403));
    await expect(loadHistoryEntity('car',{car_id:17,table_id:4,application_id:42})).rejects.toThrow('недоступна');
    expect(apiRequest).toHaveBeenCalledTimes(1);
  });
  it('reads application attachments using existing visibility and exact IDs', async () => {
    apiRequest.mockResolvedValueOnce(response([{id:2,attachment_type:'people',entry_date_to:'2099-10-15'}]))
      .mockResolvedValueOnce(response([{id:17,last_name:'Current',entry_date_to:null}]));
    const entity=await loadHistoryEntity('employee',{employee_id:17,application_id:42});
    expect(entity).toMatchObject({id:17,last_name:'Current',entry_date_to:null});
    expect(apiRequest.mock.calls.map(call=>call[0])).toEqual(['/applications/42/attachments','/attachments/2/employees']);
  });
  it('refuses a record which has left all available authorized scopes', async () => {
    apiRequest.mockResolvedValue(response([]));
    await expect(loadHistoryEntity('employee',{employee_id:17,table_id:4})).rejects.toThrow('недоступна');
  });
});
