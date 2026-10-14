/** Errors emitted by native module imports / Vite CSS preload, not API failures. */
export function isAssetLoadError(error) {
  return /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed|Unable to preload CSS|Loading (?:CSS )?chunk [\w-]+ failed/i.test(error?.message || '');
}

/** Recovery is explicit; failed imports must never automatically reload a form. */
export function installAssetLoadRecovery({ router, ready, confirm, notify, isDirty, isConfirmOpen, reload, target = window }) {
  const seen = new WeakSet();
  let pending = false;
  let stopped = false;
  let reloading = false;
  async function recover(error) {
    if (!isAssetLoadError(error) || seen.has(error) || pending || stopped || reloading) return;
    seen.add(error);
    pending = true;
    try {
      await ready;
      if (stopped) return;
      if (target.navigator.onLine === false) {
        notify({ type: 'error', prefix: 'Не удалось загрузить часть сайта. Проверь соединение и повтори действие.' });
        return;
      }
      if (isDirty()) {
        notify({ type: 'error', prefix: 'Не удалось загрузить часть сайта. Сохрани изменения перед обновлением страницы.' });
        return;
      }
      if (isConfirmOpen()) {
        notify({ type: 'error', prefix: 'Не удалось загрузить часть сайта. Повтори действие или обнови страницу после завершения текущего действия.' });
        return;
      }
      const accepted = await confirm({
        title: 'Не удалось загрузить страницу',
        message: 'Проверь соединение. Если сайт был обновлён, обновление страницы загрузит новую версию.',
        confirmText: 'Обновить', cancelText: 'Остаться', danger: false,
      });
      if (!accepted || stopped || target.navigator.onLine === false) return;
      // Состояние формы могло измениться, пока подтверждение было открыто.
      if (isDirty()) {
        notify({ type: 'error', prefix: 'Сначала сохрани изменения, затем обнови страницу.' });
        return;
      }
      reloading = true;
      reload();
    } finally {
      pending = false;
    }
  }
  // Оставляем исходную ошибку Vite видимой router/компоненту: preventDefault
  // превратил бы отклонённый import в успешное undefined.
  function handle(error) { void recover(error).catch(console.error); }
  const onPreload = (event) => handle(event.payload);
  const removeRouterHandler = router.onError(handle);
  target.addEventListener('vite:preloadError', onPreload);
  return () => {
    stopped = true;
    removeRouterHandler();
    target.removeEventListener('vite:preloadError', onPreload);
  };
}