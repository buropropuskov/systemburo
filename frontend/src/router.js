import { createRouter, createWebHistory } from 'vue-router';
import { confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { isErrorPageReachable, closeIncidentOnLeave } from '@/utils/errorPageAccess';
import { shouldRedirectToMaintenance } from '@/utils/maintenanceAccess';
import { useAuthStore } from '@/stores/auth';
import { useMaintenanceStore } from '@/stores/maintenance';
import { usePDConsentStore } from '@/stores/pdConsent';
import { buildLoginRedirect } from '@/utils/postLoginRedirect';
import LoginComponent from './components/LoginComponent.vue';
import TablesComponent from './components/TablesComponent.vue';
import AccountComponent from './components/AccountComponent.vue';
import CreateApplication from './components/CreateApplication/CreateApplication.vue';
import CarsView from './views/CarsView.vue';
import ApplicationsCenter from './views/ApplicationsCenter.vue';
import TableConstructor from './components/TableConstructor.vue';
import EmployeeView from './views/EmployeeView.vue';
import NewsAndReview from './views/NewsAndReview.vue';
import FeedbackPage from './views/FeedbackPage.vue';

// Гейтинг прав (#187, Фаза 2): admin/table-маршруты несут meta.permission и
// проверяются в beforeEach через usePermissionsStore.hasPermission. super/admin
// проходят (allowAll), обычный юзер — по гранту. requiresBuro (костыль "только
// супер-админ") снят: админку теперь видит и обычный администратор (is_admin).
// Обычные/контекстные страницы (Центр, Автомобили, Сотрудники, Новости, ЛК)
// остаются requiresAuth-only — их доступ контекстный/базовый, жёсткий route-гейт
// отрезал бы принимающих/согласующих и ловил бы лок-аут при сбое /permissions/my.
//
// meta.permission может быть строкой или функцией (to) => key — функция нужна
// динамическим table.<slug>.<verb> ключам, зависящим от параметра маршрута.
const routes = [
  {
    path: '/',
    name: 'LoginComponent',
    component: LoginComponent,
    meta: { requiresAuth: false }
  },
  {
    path: '/new-application',
    name: 'NewApplication',
    component: CreateApplication,
    meta: { requiresAuth: true }
  },
  {
    path: '/data-processing',
    name: 'DataProcessing',
    component: () => import('./views/DataProcessingView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/submit-form',
    redirect: '/new-application'
  },
  {
    path: '/table',
    redirect: '/personal-cabinet'
  },
  {
    path: '/table/:tableName',
    name: 'DynamicTable',
    component: TablesComponent,
    meta: { requiresAuth: true, permission: (to) => `table.${to.params.tableName}.view` }
  },
  {
    path: '/table/:tableName/trash',
    name: 'TableTrash',
    component: () => import('@/views/TrashView.vue'),
    meta: { requiresAuth: true, permission: (to) => `table.${to.params.tableName}.trash` }
  },
  {
    path: '/table/:tableName/versions',
    name: 'TableVersions',
    component: () => import('@/views/TableVersionsView.vue'),
    meta: { requiresAuth: true, permission: (to) => `table.${to.params.tableName}.versions` }
  },
  {
    path: '/personal-cabinet',
    name: 'Account',
    component: AccountComponent,
    meta: { requiresAuth: true }
  },
  // Тонкая настройка уведомлений (#1748, S8) - своя, не админская: доступна
  // любому авторизованному так же, как ЛК и Автомобили, без permission-гейта.
  {
    path: '/notification-settings',
    name: 'NotificationSettings',
    component: () => import('./views/NotificationSettingsView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/carsview',
    name: 'CarsView',
    component: CarsView,
    meta: { requiresAuth: true }
  },
  {
    path: '/center',
    name: 'ApplicationsCenter',
    component: ApplicationsCenter,
    meta: { requiresAuth: true, permission: 'page.center' }
  },
  // "Доступные мне": вложения согласованных заявок, совпавших по месту с местами
  // охранника. Доступ - охранник (user_type code 'security') или супер-админ.
  // Гейт по коду типа - в beforeEach (контекстная авторизация, не permission-key).
  {
    path: '/accessible-attachments',
    name: 'AccessibleAttachments',
    component: () => import('./views/AccessibleAttachmentsView.vue'),
    meta: { requiresAuth: true, requiresSecurityOrAdmin: true }
  },
  {
    path: '/table-constructor',
    name: 'TableConstructor',
    component: TableConstructor,
    meta: { requiresAuth: true, permission: 'page.admin.tables_constructor' }
  },
  {
    path: '/number-format',
    redirect: '/admin/number-formats'
  },
  {
    path: '/employeesview',
    name: 'EmployeeView',
    component: EmployeeView,
    meta: { requiresAuth: true }
  },
  {
    path: '/news',
    name: 'NewsAndReview',
    component: NewsAndReview,
    meta: { requiresAuth: true }
  },
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
    component: () => import('./views/RequestsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.monitoring' }
  },
  {
    path: '/admin/data-processing',
    name: 'AdminDataProcessing',
    component: () => import('./views/admin/DataProcessingView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin' }
  },
  // page.admin.settings (#7): точечный ключ каталога прав, не super-only -
  // администраторы получают его через adminAll, а конкретному администратору
  // доступ можно точечно отобрать личным deny-override. Раньше бэкенд требовал
  // именно супер-админа (checkSuper в settings_service.go), это отменено.
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('./views/AdminSettings.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.settings' }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('./views/admin/UserControlView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.users' }
  },
  {
    path: '/admin/blacklist',
    name: 'AdminBlacklist',
    component: () => import('./views/admin/BlacklistView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.blacklist' }
  },
  {
    path: '/admin/permission-groups',
    name: 'AdminPermissionGroups',
    component: () => import('./views/admin/AdminPermissionGroups.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/roles',
    name: 'AdminRoles',
    component: () => import('./views/admin/AdminRoles.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.manage' }
  },
  {
    path: '/admin/pd-audit',
    name: 'PdAuditLog',
    component: () => import('./views/admin/PdAuditLog.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.pd_audit' }
  },
  {
    path: '/admin/access-denials',
    name: 'AccessDenialsLog',
    component: () => import('./views/admin/AccessDenialsLog.vue'),
    meta: { requiresAuth: true, permission: 'permission.audit.read' }
  },
  {
    path: '/admin/organizations',
    name: 'AdminOrganizations',
    component: () => import('./views/admin/OrganizationsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/companies',
    name: 'AdminCompanies',
    component: () => import('./views/admin/CompaniesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/unload-places',
    name: 'AdminUnloadPlaces',
    component: () => import('./views/admin/UnloadPlacesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/number-formats',
    name: 'AdminNumberFormats',
    component: () => import('./views/admin/NumberFormatsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/citizenship',
    name: 'AdminCitizenship',
    component: () => import('./views/admin/CitizenshipView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/marks',
    name: 'AdminMarks',
    component: () => import('./views/admin/MarksView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/attachment-types',
    name: 'AdminAttachmentTypes',
    component: () => import('./views/admin/AttachmentTypesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/user-types',
    name: 'AdminUserTypes',
    component: () => import('./views/admin/UserTypesView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/approvers',
    name: 'AdminApprovers',
    component: () => import('./views/admin/ApproversView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/documents',
    name: 'AdminDocuments',
    component: () => import('./views/admin/DocumentsView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/news',
    name: 'AdminNews',
    component: () => import('./views/admin/NewsManagement.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.directories' }
  },
  {
    path: '/admin/guide',
    name: 'AdminGuide',
    component: () => import('./views/admin/GuideManagementView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin' }
  },
  {
    path: '/admin/file-archive',
    name: 'AdminFileArchive',
    component: () => import('./views/admin/FileArchiveView.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.file_archive' }
  },
  {
    path: '/analytics',
    name: 'analytics',
    component: () => import('./views/StatisticsView.vue'),
    meta: { requiresAuth: true, permission: 'page.statistics' }
  },
  {
    path: '/500',
    name: 'Error500',
    component: () => import('./views/Error500.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/maintenance',
    name: 'Maintenance',
    component: () => import('./views/Maintenance.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/admin/system-control',
    name: 'SystemControl',
    component: () => import('./views/admin/SystemControl.vue'),
    meta: { requiresAuth: true, permission: 'page.admin.system_control' }
  },
  {
    path: '/403',
    name: 'Forbidden',
    component: () => import('./views/Forbidden.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/404',
    name: 'NotFound',
    component: () => import('./views/NotFound.vue'),
    meta: { requiresAuth: false }
  },
  // Ловушка неизвестных адресов - последней в списке, иначе перехватит всё выше.
  // Без неё опечатка в URL давала пустой экран и "No match found" в консоли.
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFoundCatchAll',
    component: () => import('./views/NotFound.vue'),
    meta: { requiresAuth: false }
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

// Guard синхронный - tryRestoreSession вызывается в main.js ДО mount,
// поэтому к моменту первой navigation auth store уже hydrated (token
// в памяти если refresh cookie жив). На F5 guard сразу видит реальное
// состояние без async гонок.
router.beforeEach(async (to, from, next) => {
  // Защита от потери несохранённых изменений: если на странице есть форма
  // с pending-правками - спросить подтверждение перед navigation. Применяется
  // ко всем переходам, ВКЛЮЧАЯ программные next() ниже - кроме первого
  // монтирования (from.name === undefined).
  if (from.name !== undefined && !(await confirmIfAnyDirty())) {
    next(false);
    return;
  }

  const authStore = useAuthStore();
  const maintenanceStore = useMaintenanceStore();
  const isAuthenticated = authStore.isAuthenticated;
  const isSuperAdmin = authStore.isSuperAdmin;

  // Maintenance: режим включён и юзер не супер-админ - на /maintenance.
  // Исключения (страницы-ошибки, вход по `/?admin`) - в maintenanceAccess.
  if (shouldRedirectToMaintenance(to, {
    enabled: maintenanceStore.enabled,
    isSuperAdmin,
  })) {
    next({ name: 'Maintenance' });
    return;
  }

  // Страницы-ошибки открываются только по факту события: прямой заход по URL
  // на /500, /maintenance или /403 показал бы юзеру аварию, которой не было.
  closeIncidentOnLeave(to, from);
  if (!isErrorPageReachable(to)) {
    next('/');
    return;
  }

  if (to.meta.requiresAuth && !isAuthenticated) {
    // #974: сохраняем адрес, на который метил переход, в query - push-уведомление
    // приводит человека спустя дни, когда сессия давно протухла, и без этого
    // логин всегда высаживал на дефолтную ленту вместо заявки из уведомления.
    next(buildLoginRedirect(to.fullPath));
    return;
  }

  // Состояние гейта согласия (#1567) должно быть известно ДО того, как
  // смонтируется страница. Иначе между входом и ответом гейта страница успевает
  // отрисоваться и запросить свои данные, все запросы получают отказ, а окно
  // согласия встаёт поверх уже нарисованного интерфейса - на живом стенде это
  // мигание страницы и волна отказов на каждом входе.
  //
  // Ждать безопасно: refresh() дедуплицируется, после первого ответа становится
  // no-op, а сетевую ошибку глушит сам - навигация не залипнет и при недоступном
  // сервере.
  if (to.meta.requiresAuth) {
    await usePDConsentStore().refresh();
  }
  // Гейт "Доступные мне": супер-админ проходит сразу; иначе резолвим код типа
  // пользователя (один раз, лениво) и пускаем только охранника. userTypeCode
  // приходит из /users/me - на F5 он ещё null, поэтому подгружаем до проверки.
  if (to.meta.requiresSecurityOrAdmin) {
    if (!isSuperAdmin && authStore.userTypeCode === null) {
      await authStore.loadUserTypeCode();
    }
    // Грант page.available приходит ролью/группой через permissions store, но на
    // F5/прямой ссылке permissions ещё не загружены (fetch ниже), поэтому
    // подгружаем до getter - иначе обычный юзер с грантом ошибочно уедет в ЛК.
    if (!isSuperAdmin && !authStore.isSecurity) {
      const { usePermissionsStore } = await import('@/stores/permissions');
      const permStore = usePermissionsStore();
      if (permStore.isStale) await permStore.fetchPermissions();
    }
    if (!authStore.canViewAccessibleAttachments) {
      next('/personal-cabinet');
      return;
    }
  }
  // Push-уведомление (#974): service worker не знает прав пользователя (живёт
  // вне вкладки, вне Pinia) и потому не может сам решить Центр vs личный
  // кабинет - ведёт на нейтральный /?open_application=<id>, а маршрут выбирает
  // ЭТОТ гард, тем же кодом, что и клик по карточке уведомления
  // (useNotificationNavigation.resolveApplicationRoute). Гость уходит на вход
  // через уже существующий #974 механизм (query.redirect) - после логина
  // снова попадёт на этот же адрес, но авторизованным.
  if (to.path === '/' && to.query.open_application) {
    if (!isAuthenticated) {
      next(buildLoginRedirect(to.fullPath));
      return;
    }
    // Права на старте подгружаются асинхронно (#187e) - без ожидания носитель
    // page.center иногда попадал бы в личный кабинет (isStale -> hasPermission
    // на пустом кэше false), баг ловится только на холодном заходе и нестабильно.
    const { usePermissionsStore } = await import('@/stores/permissions');
    const permStore = usePermissionsStore();
    if (permStore.isStale) await permStore.fetchPermissions();
    const { useNotificationNavigation } = await import('@/composables/useNotificationNavigation');
    const { resolveApplicationRoute } = useNotificationNavigation();
    next(resolveApplicationRoute(to.query.open_application));
    return;
  }

  if (to.path === '/' && isAuthenticated) {
    next('/news');
    return;
  }

  // Permission polling (#187e): refresh permissions при stale-cache на любой
  // протектед-странице. Бан/изменение прав администратором заметится в
  // течение 30s максимум. meta.permission может быть функцией (to) => key
  // (динамические table.<slug>.<verb>).
  if (isAuthenticated && (to.meta.requiresAuth || to.meta.permission)) {
    const { usePermissionsStore } = await import('@/stores/permissions');
    const store = usePermissionsStore();
    if (store.isStale) await store.fetchPermissions();
    // Забаненного уводим в ЛК (а не в /403): там BanOverlay блокирует
    // взаимодействие, но единственная доступная ему страница - личный кабинет.
    if (store.banned && to.name !== 'Account') {
      next('/personal-cabinet');
      return;
    }
    const requiredPermission = typeof to.meta.permission === 'function'
      ? to.meta.permission(to)
      : to.meta.permission;
    if (requiredPermission && !store.hasPermission(requiredPermission)) {
      next({ name: 'Forbidden', query: { permission: requiredPermission, from: to.fullPath } });
      return;
    }
  }

  next();
});

export default router;
