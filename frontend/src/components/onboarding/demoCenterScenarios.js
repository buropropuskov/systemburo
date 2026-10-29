import {
  buildDemoCenterApplication,
  buildDemoCenterAttachments,
  buildDemoCenterCars,
  buildDemoCenterEmployees,
  buildDemoCenterResponsibleUsers,
  buildDemoCenterSupplements,
  buildDemoCenterFiles,
} from './demoCenterApplication';
import { moscowDateInput } from '@/utils/datetime';

export const DEMO_CENTER_SCENES = Object.freeze({
  REVIEWER_PENDING: 'reviewer-pending',
  ACCEPT_READY: 'accept-ready',
  ACCEPT_SUPPLEMENT_APPROVED: 'accept-supplement-approved',
});

const DAY = 24 * 60 * 60 * 1000;
const HOUR = 60 * 60 * 1000;
export const DEMO_CENTER_SUPPLEMENT_ID = 999000401;
export const DEMO_CENTER_SUPPLEMENT_CAR_ID = 999000203;

/**
 * Pure, role-scoped snapshots of a fictional application. The caller supplies
 * actor identity and one clock value; no real membership or permission is set.
 * This module does not install an interceptor or advance any workflow.
 */
export function buildDemoCenterScenario({ scene, actorId, now } = {}) {
  if (!Object.values(DEMO_CENTER_SCENES).includes(scene)) throw new TypeError('Unknown center demo scene');
  if (!Number.isSafeInteger(actorId) || actorId <= 0) throw new TypeError('A known demo actor is required');
  if (!Number.isFinite(now) || !Number.isFinite(new Date(now).getTime())) throw new TypeError('A finite demo clock is required');

  const reviewing = scene === DEMO_CENTER_SCENES.REVIEWER_PENDING;
  const supplementReady = scene === DEMO_CENTER_SCENES.ACCEPT_SUPPLEMENT_APPROVED;
  const fakeUserIds = [];
  for (let id = 999000900; fakeUserIds.length < 3; id++) {
    if (id !== actorId) fakeUserIds.push(id);
  }
  const [senderId, reviewerId, secondReviewerId] = fakeUserIds;
  const responsibleUsers = buildDemoCenterResponsibleUsers(now).map((user, index) => ({
    ...user,
    id: reviewing && index === 0 ? actorId : index === 0 ? reviewerId : secondReviewerId,
    ...(reviewing && index === 0 ? {
      username: 'example_current_reviewer', last_name: 'Вы', first_name: null, middle_name: null,
    } : {}),
    required_approval: true,
    approval_status: reviewing ? 'pending' : 'approved',
    approval_comment: reviewing ? null : user.approval_comment,
    approval_datetime: reviewing ? null : user.approval_datetime,
  }));
  const cars = buildDemoCenterCars().map((car, index) => ({
    ...car,
    id: 999000201 + index,
    blacklist_similar: !reviewing && !supplementReady && car.blacklist_similar
      ? { ...car.blacklist_similar, overridden: false } : null,
  }));
  if (supplementReady) cars.push({
    ...cars[0], id: DEMO_CENTER_SUPPLEMENT_CAR_ID, car_number: 'К613МТ77', car_brand: 'ГАЗель',
    unload_places: [], target_tables: [], is_pending: true,
    supplement_id: DEMO_CENTER_SUPPLEMENT_ID, supplement_number: 1, supplement_status: 'approved',
  });
  const employees = buildDemoCenterEmployees().map((employee, index) => ({ ...employee, id: 999000301 + index }));
  const files = buildDemoCenterFiles().map((file, index) => ({ ...file, id: 999000501 + index }));
  const blacklistCount = cars.filter(car => car.blacklist_similar && !car.blacklist_similar.overridden).length;
  const application = {
    // The legacy builder uses `ctx.now || Date.now()`. A one-millisecond seed
    // keeps epoch-zero deterministic; all scene timestamps are explicit below.
    ...buildDemoCenterApplication({ now: now === 0 ? 1 : now }),
    application_number: `№ ${moscowDateInput(new Date(now - 3 * DAY)).replace(/-/g, '')}/001`,
    sender_user_id: senderId,
    confirmation: reviewing ? 'Согласование' : 'Согласовано',
    confirmation_datetime: reviewing ? null : new Date(now - 3 * HOUR).toISOString(),
    sending_datetime: new Date(now - 3 * DAY).toISOString(),
    status: supplementReady ? 'В работе' : 'Непрочитано',
    responsible_user_id: supplementReady ? actorId : null,
    responsible_full_name: supplementReady ? 'Пример: принимающий' : '',
    responsible_name: supplementReady ? 'Пример' : '',
    organization_moderation_status: reviewing ? 'approved' : 'pending',
    has_open_supplement: supplementReady,
    supplements_count: supplementReady ? 1 : 0,
    open_supplement: supplementReady ? { id: DEMO_CENTER_SUPPLEMENT_ID, number: 1, status: 'approved' } : null,
    has_files: files.length > 0,
    blacklist_flags_count: blacklistCount,
    has_unoverridden_blacklist_flags: blacklistCount > 0,
    responsible_users: responsibleUsers,
  };
  const supplements = supplementReady ? buildDemoCenterSupplements(now).map(round => {
    // The previous fixture used presentation aliases, not SupplementInfo fields.
    const { round: unusedRound, author_full_name: unusedAuthor, elements_count: unusedCount, ...base } = round;
    void unusedRound; void unusedAuthor; void unusedCount;
    return {
      ...base,
      id: DEMO_CENTER_SUPPLEMENT_ID,
      number: 1,
      status: 'approved',
      created_by_user_id: senderId,
      created_by_name: 'Пример: заявитель',
      confirmation_datetime: new Date(now - HOUR).toISOString(),
      decided_by_user_id: null,
      decision_comment: null,
      decided_at: null,
      counts: { vehicles: 1, employees: 0, items: 0 },
      approvals: responsibleUsers.map(user => ({
        user_id: user.id, username: user.username,
        full_name: [user.last_name, user.first_name, user.middle_name].filter(Boolean).join(' '),
        required_approval: user.required_approval,
        approval_status: 'approved', approval_comment: null,
        approval_datetime: new Date(now - HOUR).toISOString(),
      })),
    };
  }) : [];

  return { scene, actorId, now, application, responsibleUsers,
    attachments: buildDemoCenterAttachments(now), cars, employees, supplements, files };
}
