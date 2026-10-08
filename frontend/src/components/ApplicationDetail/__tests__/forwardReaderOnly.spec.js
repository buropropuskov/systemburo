import { describe, expect, it } from 'vitest';
import { resolveForwardReaderOnly } from '../forwardReaderOnly.js';

describe('resolveForwardReaderOnly', () => {
  const context = {
    applicationData: { status: 'На согласовании', sender_user_id: 10 },
    currentUserId: 20,
    isResponsibleUser: false,
    isSuperAdmin: false,
    isApprover: false,
  };

  it('restricts an ordinary reader to viewing', () => {
    expect(resolveForwardReaderOnly(context)).toBe(true);
  });

  it.each(['isResponsibleUser', 'isSuperAdmin', 'isApprover'])(
    'preserves active application authority for %s', (role) => {
      expect(resolveForwardReaderOnly({ ...context, [role]: true })).toBe(false);
    },
  );

  it('preserves the active sender authority', () => {
    expect(resolveForwardReaderOnly({ ...context, currentUserId: 10 })).toBe(false);
  });

  it.each(['isResponsibleUser', 'isSuperAdmin', 'isApprover'])(
    'withdrawal takes priority over %s', (role) => {
      expect(resolveForwardReaderOnly({
        ...context,
        [role]: true,
        applicationData: { ...context.applicationData, status: 'Отозвана' },
      })).toBe(true);
    },
  );

  it('withdrawal takes priority over sender authority', () => {
    expect(resolveForwardReaderOnly({
      ...context,
      currentUserId: 10,
      applicationData: { ...context.applicationData, status: 'Отозвана' },
    })).toBe(true);
  });
});
