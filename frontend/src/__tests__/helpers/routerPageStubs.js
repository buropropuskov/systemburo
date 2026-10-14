import { vi } from 'vitest';

// Эти unit-тесты проверяют router guards, а не дерево страницы. После перехода
// на lazy routes импорт Vue-страницы выполняется внутри навигации; в Vitest её
// обратная зависимость api/client -> router может ожидать тот же импорт.
// Имена/маршруты/guards настоящие; UI и загрузка модулей проверяются отдельно.
vi.mock('@/components/LoginComponent.vue', () => ({ default: { name: 'LoginComponent', render: () => null } }));
vi.mock('@/components/AccountComponent.vue', () => ({ default: { name: 'AccountComponent', render: () => null } }));
vi.mock('@/views/NewsAndReview.vue', () => ({ default: { name: 'NewsAndReview', render: () => null } }));
vi.mock('@/views/ApplicationsCenter.vue', () => ({ default: { name: 'ApplicationsCenter', render: () => null } }));