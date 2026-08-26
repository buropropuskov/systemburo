import { describe, it, expect } from 'vitest';
import { shouldRedirectToMaintenance } from '../maintenanceAccess';

const ON = { enabled: true, isSuperAdmin: false };

describe('shouldRedirectToMaintenance', () => {
  it('уводит обычного пользователя с любой страницы', () => {
    expect(shouldRedirectToMaintenance({ name: 'ApplicationsCenter', query: {} }, ON)).toBe(true);
    expect(shouldRedirectToMaintenance({ name: 'LoginComponent', query: {} }, ON)).toBe(true);
  });

  it('не трогает страницы работ и ошибки, иначе редирект зациклится', () => {
    expect(shouldRedirectToMaintenance({ name: 'Maintenance', query: {} }, ON)).toBe(false);
    expect(shouldRedirectToMaintenance({ name: 'Error500', query: {} }, ON)).toBe(false);
  });

  it('пускает на форму входа по ссылке /?admin - иначе супер-админ не выключит режим', () => {
    expect(shouldRedirectToMaintenance({ name: 'LoginComponent', query: { admin: '' } }, ON)).toBe(false);
    expect(shouldRedirectToMaintenance({ name: 'ApplicationsCenter', query: { admin: '' } }, ON)).toBe(true);
  });

  it('супер-админа не трогает, режим выключен - никого не трогает', () => {
    expect(shouldRedirectToMaintenance({ name: 'ApplicationsCenter', query: {} }, { enabled: true, isSuperAdmin: true })).toBe(false);
    expect(shouldRedirectToMaintenance({ name: 'ApplicationsCenter', query: {} }, { enabled: false, isSuperAdmin: false })).toBe(false);
  });
});
