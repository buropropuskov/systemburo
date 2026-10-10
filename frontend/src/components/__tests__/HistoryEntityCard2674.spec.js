import { describe, expect, it } from 'vitest';
import { carsTablePeriodDetails, peopleTablePeriodDetails } from '../entityPeriodParentMixins';

describe('history identity and successful external refresh #2674', () => {
  it.each([
    [carsTablePeriodDetails, 'showVehicleDetails', 'selectedVehicle', 'openVehicleDetails', 'closeVehicleDetails'],
    [peopleTablePeriodDetails, 'showDetailsModal', 'selectedEmployee', 'openEmployeeDetails', 'closeDetailsModal'],
  ])('refreshes only the already open exact card and closes when it leaves the authorized list', (mixin, show, selected, open, close) => {
    const fresh = { id: 17, applicationId: 42, entry_date_to: '2099-10-15' };
    const ctx = {[show]: true, [selected]: {id: 17}, [open](row){this.opened = row;}, [close](){this.closed = true;}};
    mixin.watch.itemsData.call(ctx, [{id: 999}, fresh]);
    expect(ctx.opened).toBe(fresh);
    mixin.watch.itemsData.call(ctx, [{id: 999}]);
    expect(ctx.closed).toBe(true);
    ctx.opened = null; ctx[show] = false;
    mixin.watch.itemsData.call(ctx, [fresh]);
    expect(ctx.opened).toBeNull();
  });
});
