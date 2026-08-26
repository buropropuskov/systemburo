import { describe, it, expect } from 'vitest'
import {
  participantRoleLabel,
  participantRoleVariant,
  approvalBadgeVariant,
  participantDisplayName,
  secondaryRoleLabels,
} from '../participantRoles'

describe('utils/participantRoles', () => {
  it('подписи ролей совпадают с ключами бэкенда', () => {
    expect(participantRoleLabel('sender')).toBe('Отправитель')
    expect(participantRoleLabel('acceptor')).toBe('Принимающий')
    expect(participantRoleLabel('approver')).toBe('Согласующий')
    expect(participantRoleLabel('reader')).toBe('Читатель')
  })

  it('незнакомая роль остаётся участником, а не сырым ключом', () => {
    expect(participantRoleLabel('guest')).toBe('Участник')
    expect(participantRoleLabel(undefined)).toBe('Участник')
  })

  it('роль не красится цветами голоса - они заняты бейджем решения рядом', () => {
    const voteColors = ['success', 'danger', 'warning']
    for (const role of ['sender', 'acceptor', 'approver', 'reader']) {
      expect(voteColors).not.toContain(participantRoleVariant(role))
    }
  })

  it('вариант бейджа голоса соответствует решению', () => {
    expect(approvalBadgeVariant('approved')).toBe('success')
    expect(approvalBadgeVariant('rejected')).toBe('danger')
    expect(approvalBadgeVariant('pending')).toBe('warning')
    expect(approvalBadgeVariant(null)).toBe('neutral')
  })

  it('скрытый по ПД работник не показывает ни ФИО, ни логина', () => {
    const name = participantDisplayName({
      full_name: '',
      username: 'i.ivanov',
      pd_hidden: true,
    })

    expect(name).toBe('Имя скрыто')
    expect(name).not.toContain('ivanov')
  })

  it('без ФИО показывается логин, без обоих - явная заглушка', () => {
    expect(participantDisplayName({ full_name: '', username: 'pt_reader' })).toBe('pt_reader')
    expect(participantDisplayName({})).toBe('Без имени')
  })

  it('вторые роли перечисляются без старшей - она уже на бейдже', () => {
    expect(secondaryRoleLabels({ roles: ['sender', 'approver'], primary_role: 'sender' }))
      .toEqual(['Согласующий'])
    expect(secondaryRoleLabels({ roles: ['reader'], primary_role: 'reader' })).toEqual([])
    expect(secondaryRoleLabels({})).toEqual([])
  })
})
