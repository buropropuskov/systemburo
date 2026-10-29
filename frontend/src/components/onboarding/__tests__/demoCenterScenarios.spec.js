import { describe, expect, it } from 'vitest';
import ApplicationActionBar from '@/components/ApplicationDetail/ApplicationActionBar.vue';
import ApplicationAttachmentDetail from '@/components/ApplicationDetail/ApplicationAttachmentDetail.vue';
import { buildDemoCenterScenario, DEMO_CENTER_SCENES } from '../demoCenterScenarios';
import { buildDemoCenterApplication, buildDemoCenterCars } from '../demoCenterApplication';

const NOW = Date.parse('2026-12-31T23:59:00Z');
const actorId = 123;
const build = scene => buildDemoCenterScenario({ scene, actorId, now: NOW });

// Exercise the actual component predicates without granting a real role/store.
function actionBar(snapshot, { acceptor = false, approvalAllowed = true } = {}) {
  const vm = {
    application: snapshot.application, responsibleUsers: snapshot.responsibleUsers,
    currentUserId: snapshot.actorId, approvers: [], isApprover: acceptor,
    approvalAllowed, supplements: snapshot.supplements,
    hasUnoverriddenBlacklistFlags: snapshot.application.has_unoverridden_blacklist_flags,
    processing: false, updatingConfirmation: false, ready: true,
  };
  for (const [key, getter] of Object.entries(ApplicationActionBar.computed)) {
    Object.defineProperty(vm, key, { get: () => getter.call(vm) });
  }
  return vm;
}

describe('role-scoped fictional center snapshots', () => {
  it.each([
    ['2026-10-26T00:05:00+03:00', '№ 20261026/001'],
    ['2026-10-25T23:55:00+03:00', '№ 20261025/001'],
    ['2027-01-01T00:05:00+03:00', '№ 20270101/001'],
  ])('keeps the application number aligned with its displayed Moscow date: %s', (sent, expected) => {
    const timestamp = Date.parse(sent);
    const snapshot = buildDemoCenterScenario({ scene: DEMO_CENTER_SCENES.REVIEWER_PENDING, actorId, now: timestamp + 3 * 86400000 });
    const legacy = buildDemoCenterApplication({ now: timestamp + 86400000 });
    expect(snapshot.application.application_number).toBe(expected);
    expect(legacy.application_number).toBe(expected);
    expect(snapshot.application.sending_datetime).toBe(new Date(timestamp).toISOString());
  });
  it('requires an explicit scene, actor and controlled clock', () => {
    for (const input of [{}, { scene: 'unknown', actorId, now: NOW },
      { scene: 'reviewer-pending', actorId: 0, now: NOW },
      { scene: 'reviewer-pending', actorId: 1.5, now: NOW },
      { scene: 'reviewer-pending', actorId, now: NaN },
      { scene: 'reviewer-pending', actorId }]) {
      expect(() => buildDemoCenterScenario(input)).toThrow(TypeError);
    }
  });

  it('uses a single clock and returns independent snapshots without changing old builders', () => {
    const beforeApplication = buildDemoCenterApplication({ now: NOW });
    const beforeCars = buildDemoCenterCars();
    const first = build(DEMO_CENTER_SCENES.ACCEPT_READY);
    expect(first).toEqual(build(DEMO_CENTER_SCENES.ACCEPT_READY));
    first.cars[0].target_tables[0].name = 'changed';
    first.responsibleUsers[0].approval_status = 'pending';
    expect(build(DEMO_CENTER_SCENES.ACCEPT_READY).cars[0].target_tables[0].name).toBe('kpp_4');
    expect(buildDemoCenterApplication({ now: NOW })).toEqual(beforeApplication);
    expect(buildDemoCenterCars()).toEqual(beforeCars);
    expect(build(DEMO_CENTER_SCENES.REVIEWER_PENDING).attachments[0].entry_date_to).toBe('2027-01-01');
  });

  it('does not fall back to the ambient clock for epoch zero and keeps number/date aligned', () => {
    const snapshot = buildDemoCenterScenario({ scene: DEMO_CENTER_SCENES.REVIEWER_PENDING, actorId, now: 0 });
    expect(snapshot.application.sending_datetime).toBe('1969-12-29T00:00:00.000Z');
    expect(snapshot.application.application_number).toBe('№ 19691229/001');
    expect(snapshot.attachments[0].entry_date_from).toBe('1970-01-01');
  });

  it.each(Object.values(DEMO_CENTER_SCENES))('keeps %s identities and list/detail flags consistent', scene => {
    const snapshot = build(scene);
    const { application, responsibleUsers, attachments, files, cars, employees, supplements } = snapshot;
    expect(application.responsible_users).toEqual(responsibleUsers);
    expect(application.sender_user_id).not.toBe(actorId);
    expect(application.has_files).toBe(files.length > 0);
    expect(application.supplements_count).toBe(supplements.length);
    expect(application.has_open_supplement).toBe(supplements.length > 0);
    expect(files.every(file => file.application_id === application.id)).toBe(true);
    expect(attachments).toHaveLength(2);
    expect(new Set([...cars, ...employees, ...files, ...supplements].map(row => row.id)).size)
      .toBe(cars.length + employees.length + files.length + supplements.length);
    const flagged = cars.filter(car => car.blacklist_similar && !car.blacklist_similar.overridden);
    expect(application.blacklist_flags_count).toBe(flagged.length);
    expect(application.has_unoverridden_blacklist_flags).toBe(flagged.length > 0);
    expect(snapshot).not.toHaveProperty('permissions');
    expect(snapshot).not.toHaveProperty('isApprover');
  });

  it.each([false, true])('reviewer pending teaches a genuine own vote for acceptor=%s', acceptor => {
    const snapshot = build(DEMO_CENTER_SCENES.REVIEWER_PENDING);
    const vm = actionBar(snapshot, { acceptor });
    expect(snapshot.application.confirmation).toBe('Согласование');
    expect(snapshot.application.confirmation_datetime).toBeNull();
    expect(snapshot.responsibleUsers.filter(user => user.id === actorId)).toHaveLength(1);
    expect(vm.isResponsibleUser).toBe(true);
    expect(vm.hasUserVoted).toBe(false);
    expect(vm.showsApproveButton).toBe(true);
    expect(vm.approvalBlockedByBlacklist).toBe(false);
    expect(vm.approvingCompletesConfirmation).toBe(false);
    expect(vm.canTakeToWork).toBe(false);
    expect(vm.canDecideSupplement).toBe(false);
    expect(actionBar(snapshot, { acceptor, approvalAllowed: false }).showsApproveButton).toBe(false);
  });

  it('accept ready requires real acceptor membership and never invents an own reviewer assignment', () => {
    const snapshot = build(DEMO_CENTER_SCENES.ACCEPT_READY);
    const vm = actionBar(snapshot, { acceptor: true });
    expect(snapshot.application.status).toBe('Непрочитано');
    expect(snapshot.responsibleUsers.every(user => user.id !== actorId && user.approval_status === 'approved')).toBe(true);
    expect(vm.isResponsibleUser).toBe(false);
    expect(vm.isApproverUser).toBe(true);
    expect(vm.canTakeToWork).toBe(true);
    expect(vm.hasNoApprovers).toBe(false);
    expect(vm.canDecideSupplement).toBe(false);
    expect(actionBar(snapshot).isApproverUser).toBe(false);
  });

  it('uses the actual approved supplement DTO on an already working application', () => {
    const snapshot = build(DEMO_CENTER_SCENES.ACCEPT_SUPPLEMENT_APPROVED);
    const [round] = snapshot.supplements;
    expect(snapshot.application.status).toBe('В работе');
    expect(round).toMatchObject({ number: 1, status: 'approved',
      application_id: snapshot.application.id, created_by_user_id: snapshot.application.sender_user_id,
      created_by_name: 'Пример: заявитель', counts: { vehicles: 1, employees: 0, items: 0 },
      decided_by_user_id: null, decided_at: null });
    expect(round.approvals).toHaveLength(snapshot.responsibleUsers.length);
    expect(round.approvals.every(vote => vote.approval_status === 'approved' && vote.user_id !== actorId)).toBe(true);
    expect(round).not.toHaveProperty('round');
    expect(round).not.toHaveProperty('author_full_name');
    expect(round).not.toHaveProperty('elements_count');
    const added = snapshot.cars.filter(car => car.supplement_id === round.id);
    expect(added).toHaveLength(round.counts.vehicles);
    expect(snapshot.cars).toHaveLength(buildDemoCenterCars().length + round.counts.vehicles);
    const marks = ApplicationAttachmentDetail.computed.supplementMarks.call({ rows: snapshot.cars });
    expect(marks[added[0].id]).toMatchObject({ state: 'approved', text: 'Ждёт принятия' });
    expect(marks[added[0].id].hint).toContain('Дополнение №1');
    expect(actionBar(snapshot, { acceptor: true }).canDecideSupplement).toBe(true);
    expect(actionBar(snapshot).canDecideSupplement).toBe(false);
    expect(actionBar(snapshot, { acceptor: true }).canVoteOnSupplement).toBe(false);
  });

  it('keeps fictional users separate even when an actor equals a reserved sample user ID', () => {
    for (const scene of Object.values(DEMO_CENTER_SCENES)) {
      const snapshot = buildDemoCenterScenario({ scene, actorId: 999000900, now: NOW });
      expect(snapshot.application.sender_user_id).not.toBe(snapshot.actorId);
      expect(snapshot.responsibleUsers.filter(user => user.id === snapshot.actorId))
        .toHaveLength(scene === DEMO_CENTER_SCENES.REVIEWER_PENDING ? 1 : 0);
    }
  });
});
