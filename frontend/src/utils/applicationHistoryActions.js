/**
 * Словари действий в ленте истории заявки.
 *
 * Вынесены из ApplicationHistory.vue отдельным модулем: сами по себе это декларации
 * без логики, а блок script компонента давно сверх порога размера, и каждое новое
 * действие системы упиралось в гейт. Ветвления (отказ в согласовании против отказа
 * принять, подпись пересылки, номер раунда дополнения) остались в компоненте - они
 * читают поля записи, а не просто сопоставляют ключ.
 *
 * Ключи обязаны совпадать со значениями AuditLog.Action на бэке
 * (internal/models/audit_log.go): неизвестный ключ ленту не роняет, но выводит
 * пользователю сырой action_type вместо подписи.
 */

/** Класс цветной точки слева от записи. */
export const ACTION_DOT_CLASS = {
  create: 'dot-create',
  read: 'dot-read',
  approve: 'dot-approve',
  reject: 'dot-reject',
  revoke_approval: 'dot-revoke',
  take_to_work: 'dot-success',
  revoke_from_work: 'dot-warning',
  restore_to_work: 'dot-info',
  completed: 'dot-success',
  employees_bulk_added: 'dot-create',
  supplement_created: 'dot-create',
  supplement_cancelled: 'dot-warning',
  supplement_approve: 'dot-approve',
  supplement_reject: 'dot-reject',
  supplement_revoke_approval: 'dot-revoke',
  supplement_confirmation_change: 'dot-system',
  supplement_accepted: 'dot-success',
  supplement_refused: 'dot-reject',
  supplement_cancelled_by_author: 'dot-warning',
  withdraw: 'dot-reject',
  assigned_responsible: 'dot-assign',
  assigned_viewer: 'dot-view',
  forwarded: 'dot-assign',
  confirmation_change: 'dot-system',
  status_change: 'dot-system',
  blacklist_override: 'dot-success',
  element_removed: 'dot-reject',
  blacklist_override_revoke: 'dot-warning',
  question_created: 'dot-info',
  // Заметку бюро ведут принимающие; эти записи бэк отдаёт только им.
  bureau_note_created: 'dot-info',
  bureau_note_updated: 'dot-info',
  bureau_note_cleared: 'dot-warning',
};

/** Подпись записи. Действия с ветвлением сюда не попадают. */
export const ACTION_TEXT = {
  create: 'Создал(-а) заявку',
  read: 'Прочитал(-а) заявку',
  approve: 'Согласовал(-а) заявку',
  revoke_approval: 'Отозвал(-а) согласование',
  take_to_work: 'Принял(-а) в работу',
  revoke_from_work: 'Отозвал(-а) из работы',
  restore_to_work: 'Вернул(-а) в работу',
  completed: 'Заявка завершена: срок действия истёк',
  employees_bulk_added: 'Добавил(-а) сотрудников списком',
  supplement_created: 'Подал(-а) дополнение',
  supplement_cancelled: 'Дополнение снято: заявка закрыта',
  supplement_approve: 'Согласовал(-а) дополнение',
  supplement_reject: 'Не согласовал(-а) дополнение',
  supplement_revoke_approval: 'Отозвал(-а) согласование дополнения',
  supplement_confirmation_change: 'Статус согласования дополнения изменился',
  supplement_accepted: 'Принял(-а) дополнение',
  supplement_refused: 'Отклонил(-а) дополнение',
  supplement_cancelled_by_author: 'Снял(-а) своё дополнение',
  withdraw: 'Отозвал(-а) заявку',
  assigned_responsible: 'Назначен(-а) ответственным получателем',
  assigned_viewer: 'Получил(-а) доступ к просмотру заявки',
  confirmation_change: 'Статус согласования изменился',
  status_change: 'Статус заявки изменился',
  blacklist_override: 'Подтвердил(-а) пропуск (возможный обход ЧС)',
  blacklist_override_revoke: 'Отменил(-а) подтверждение пропуска',
  element_removed: 'Убрал(-а) из заявки',
  // Текста заметки в ленте нет и не будет: журнал читают мониторинг и выгрузки.
  bureau_note_created: 'Оставил(-а) заметку бюро',
  bureau_note_updated: 'Изменил(-а) заметку бюро',
  bureau_note_cleared: 'Снял(-а) заметку бюро',
};
