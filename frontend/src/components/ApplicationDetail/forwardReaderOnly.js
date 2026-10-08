// Отозванная заявка пересылается только для просмотра независимо от роли.
export function resolveForwardReaderOnly({
  applicationData,
  currentUserId,
  isResponsibleUser,
  isSuperAdmin,
  isApprover,
}) {
  if (applicationData.status === 'Отозвана') return true;
  if (isSuperAdmin || isApprover) return false;
  if (applicationData?.sender_user_id === currentUserId) return false;
  return !isResponsibleUser;
}
