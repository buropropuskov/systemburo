import { DEMO_CENTER_APPLICATION_ID, DEMO_CENTER_ATTACHMENT_ID, DEMO_CENTER_PEOPLE_ID } from './demoCenterApplication';
import { DEMO_CENTER_SUPPLEMENT_ID, DEMO_CENTER_SUPPLEMENT_CAR_ID } from './demoCenterScenarios';
export const DEMO_ACTION_MESSAGE = 'Это учебный пример. Действие не выполнено.';
const clone = value => JSON.parse(JSON.stringify(value));
const ok = (data, meta) => ({ success: true, data: clone(data), ...(meta ? { meta } : {}) });
const refused = () => new Response(JSON.stringify({ success: false, error: DEMO_ACTION_MESSAGE }), {
  status: 409, headers: { 'Content-Type': 'application/json' },
});

/** Exact fictional IDs only: unknown owned paths must never fall through to fetch. */
export function createCenterDemoResponder(snapshot) {
  if (!snapshot?.actorId || !snapshot.application) throw new TypeError('A known center scene is required');
  const ids = new Set([DEMO_CENTER_APPLICATION_ID, DEMO_CENTER_ATTACHMENT_ID, DEMO_CENTER_PEOPLE_ID,
    DEMO_CENTER_SUPPLEMENT_ID, DEMO_CENTER_SUPPLEMENT_CAR_ID,
    ...snapshot.cars.map(v => v.id), ...snapshot.employees.map(v => v.id), ...snapshot.files.map(v => v.id),
    ...snapshot.supplements.map(v => v.id), snapshot.application.organization_id]);
  const application = `/applications/${DEMO_CENTER_APPLICATION_ID}`;
  return (path, method = 'GET') => {
    const pathname = path.split('?')[0];
    const own = pathname.split('/').some(part => /^\d+$/.test(part) && ids.has(Number(part)));
    if (method === 'POST' && pathname === `${application}/read`) return ok(null);
    if (method !== 'GET') return own ? refused() : null;
    if (pathname === '/applications') return ok([snapshot.application], { total: 1, page: 1, per_page: 30 });
    if (!own) return null;
    if (pathname === application || pathname === `${application}/details`) return ok(snapshot.application);
    const reads = { attachments: snapshot.attachments, 'responsible-users': snapshot.responsibleUsers,
      supplements: snapshot.supplements, files: snapshot.files, viewers: [], participants: [], history: [], questions: [], 'forward-messages': [] };
    const suffix = pathname.slice(application.length + 1);
    if (pathname.startsWith(`${application}/`) && Object.hasOwn(reads, suffix)) return ok(reads[suffix]);
    if (pathname === `/attachments/${DEMO_CENTER_ATTACHMENT_ID}/cars`) return ok(snapshot.cars);
    if (pathname === `/attachments/${DEMO_CENTER_PEOPLE_ID}/employees`) return ok(snapshot.employees);
    return refused();
  };
}
