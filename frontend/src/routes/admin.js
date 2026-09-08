/**
 * Маршруты раздела администрирования (#2356).
 *
 * Вынесены из router.js не ради красоты: файл упёрся в порог размера, и добавить в
 * него ещё один маршрут стало нельзя. Админских маршрутов больше двадцати, они
 * однотипны и меняются чаще остальных - вынести именно их дешевле всего.
 *
 * Каталог routes, а не router: рядом с router.js каталог router/ сбивал бы
 * разрешение импорта './router' - он резолвится и в файл, и в каталог.
 */
import FeedbackPage from '../views/FeedbackPage.vue';

export const adminRoutes = [
  {
    path: '/admin/feedback',
    name: 'FeedbackPage',
    component: FeedbackPage,
    meta: { requiresAuth: true, permission: 'page.admin.feedback' }
  },
  {
    path: '/admin/requests',
    name: 'RequestsView',
    // По требованию: вместе с разделом из стартовой загрузки уходит Chart.js.
    component: () => import('../views/RequestsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.monitoring' }
  },
  {
    path: '/admin/data-processing',
    name: 'AdminDataProcessing',
    component: () => import('../views/admin/DataProcessingView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin' }
  },
  // page.admin.settings (#7): точечный ключ каталога прав, не super-only -
  // администраторы получают его через adminAll, а конкретному администратору
  // доступ можно точечно отобрать личным deny-override. Раньше бэкенд требовал
  // именно супер-админа (checkSuper в settings_service.go), это отменено.
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('../views/AdminSettings.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.settings' }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('../views/admin/UserControlView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.users' }
  },
  {
    path: '/admin/blacklist',
    name: 'AdminBlacklist',
    component: () => import('../views/admin/BlacklistView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.blacklist' }
  },
  {
    path: '/admin/permission-groups',
    name: 'AdminPermissionGroups',
    component: () => import('../views/admin/AdminPermissionGroups.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/roles',
    name: 'AdminRoles',
    component: () => import('../views/admin/AdminRoles.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/pd-subject',
    name: 'PdSubjectView',
    component: () => import('../views/admin/PdSubjectView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.pd_subject' }
  },
  {
    path: '/admin/pd-audit',
    name: 'PdAuditLog',
    component: () => import('../views/admin/PdAuditLog.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.pd_audit' }
  },
  {
    path: '/admin/access-denials',
    name: 'AccessDenialsLog',
    component: () => import('../views/admin/AccessDenialsLog.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.read' }
  },
  {
    path: '/admin/organizations',
    name: 'AdminOrganizations',
    component: () => import('../views/admin/OrganizationsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/companies',
    name: 'AdminCompanies',
    component: () => import('../views/admin/CompaniesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/unload-places',
    name: 'AdminUnloadPlaces',
    component: () => import('../views/admin/UnloadPlacesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/number-formats',
    name: 'AdminNumberFormats',
    component: () => import('../views/admin/NumberFormatsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/citizenship',
    name: 'AdminCitizenship',
    component: () => import('../views/admin/CitizenshipView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/marks',
    name: 'AdminMarks',
    component: () => import('../views/admin/MarksView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/attachment-types',
    name: 'AdminAttachmentTypes',
    component: () => import('../views/admin/AttachmentTypesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/user-types',
    name: 'AdminUserTypes',
    component: () => import('../views/admin/UserTypesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/approvers',
    name: 'AdminApprovers',
    component: () => import('../views/admin/ApproversView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/documents',
    name: 'AdminDocuments',
    component: () => import('../views/admin/DocumentsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/news',
    name: 'AdminNews',
    component: () => import('../views/admin/NewsManagement.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/guide',
    name: 'AdminGuide',
    component: () => import('../views/admin/GuideManagementView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin' }
  },
  {
    path: '/admin/file-archive',
    name: 'AdminFileArchive',
    component: () => import('../views/admin/FileArchiveView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.file_archive' }
  },
];
