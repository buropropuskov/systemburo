// Drafts keep stable references only; the recipient list supplies current labels.
export function recipientReferences(readers) {
  if (!Array.isArray(readers)) return []
  return readers
    .filter(reader => Number.isSafeInteger(reader?.user_id) && reader.user_id > 0)
    .map(reader => ({ user_id: reader.user_id }))
}
