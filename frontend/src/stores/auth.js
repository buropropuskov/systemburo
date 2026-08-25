import { defineStore } from 'pinia';
import { getMe } from '@/api/auth';
// Циклический импорт с permissions.js (он импортит useAuthStore) безопасен:
// обе стороны зовут стор только в рантайме (getter / parsePermissionsResponse),
// не на этапе инициализации модуля - ESM live bindings уже заполнены к вызову.
import { usePermissionsStore } from './permissions';
// Тот же случай с client.js: он импортит этот стор, а мы зовём его восстановление
// сессии только внутри действия. Обе стороны - объявления функций, к моменту
// вызова связывания уже заполнены.
import { tryRestoreSession } from '@/api/client';
import { stopImpersonation } from '@/api/impersonation';
import { useDeletionsStore } from './deletions';

// In-flight промис загрузки типа пользователя - см. loadUserTypeCode.
let userTypePromise = null;

function decodeToken(token) {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch {
    return null;
  }
}

// Access token хранится только в памяти (Pinia state).
// Refresh token живёт в HttpOnly cookie и JS его не видит - поэтому
// isAuthenticated определяется наличием access и попыткой refresh на старте.
export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: null,
    // Код типа пользователя из /users/me (стабильный, не type_id).
    // null до первой загрузки.
    userTypeCode: null,
    /**
     * Режим «войти как пользователь» (#1912): кем администратор сейчас работает.
     * null - обычная работа. Живёт только в памяти: после перезагрузки страницы
     * маркер режима теряется, а восстановление сессии по cookie возвращает
     * администратора в его собственную учётную запись. Это безопасный исход -
     * остаться в чужой учётной записи без полосы было бы худшим.
     *
     * @type {null | { id: number, username: string, fullName: string, expiresAt: string }}
     */
    impersonation: null,
  }),

  getters: {
    isAuthenticated() {
      if (!this.token) return false;
      const payload = decodeToken(this.token);
      return !!(payload && payload.exp > Math.floor(Date.now() / 1000));
    },
    userPayload() {
      return this.token ? decodeToken(this.token) : null;
    },
    userType() {
      return this.userPayload?.type_id || null;
    },
    /**
     * @deprecated используется для legacy-проверок. Новый код должен
     * использовать `isSuperAdmin` (читается из claim `is_super_admin`).
     * После #187 type_id=6 не идентифицирует супер-админа.
     */
    isAdmin() {
      return this.isSuperAdmin;
    },
    isSuperAdmin() {
      return this.userPayload?.is_super_admin === true;
    },
    isImpersonating() {
      return this.impersonation !== null;
    },
    // Имя для полосы: ФИО, если оно заполнено, иначе логин.
    impersonatedName() {
      return this.impersonation?.fullName || this.impersonation?.username || '';
    },
    username() {
      return this.userPayload?.username || null;
    },
    /** Идентификатор работника из маркера доступа (claim user_id). */
    userId() {
      return this.userPayload?.user_id ?? null;
    },
    isSecurity() {
      return this.userTypeCode === 'security';
    },
    // Гибрид: охранник и супер-админ видят раздел по типу/роли, плюс любая роль
    // с грантом page.available. Тумблер только добавляет доступ - отозвать его
    // у охранника нельзя (доступ по типу пользователя).
    canViewAccessibleAttachments() {
      return (
        this.isSuperAdmin
        || this.isSecurity
        || usePermissionsStore().hasPermission('page.available')
      );
    },
  },

  actions: {
    setTokens(token) {
      this.token = token;
    },
    clearTokens() {
      this.token = null;
      this.userTypeCode = null;
      this.impersonation = null;
    },

    /**
     * Открывает сеанс работы от чужого имени: подменяет маркер доступа и
     * перечитывает всё, что зависит от личности. Свой маркер администратора не
     * сохраняется - возврат идёт обновлением сессии по его же cookie, которую
     * режим не трогает.
     *
     * @param {{ token: string, expires_at: string, target: { id: number, username: string, full_name: string } }} session
     */
    async beginImpersonation(session) {
      this.token = session.token;
      this.impersonation = {
        id: session.target.id,
        username: session.target.username,
        fullName: session.target.full_name,
        expiresAt: session.expires_at,
      };
      await this.reloadIdentity();
    },

    /**
     * Возвращает администратора в свою учётную запись.
     *
     * Признак режима снимается ДО обращения к бэкенду намеренно: запрос выхода
     * идёт через общий клиент, а тот при 401 сам зовёт завершение режима - с
     * поднятым признаком получилась бы бесконечная рекурсия.
     *
     * @param {{ recordExit?: boolean }} [options] recordExit=false, когда маркер
     *   режима уже истёк: записывать выход нечем, запрос всё равно вернёт 401.
     * @returns {Promise<boolean>} удалось ли вернуть свою учётную запись
     */
    async endImpersonation({ recordExit = true } = {}) {
      if (!this.impersonation) return false;
      this.impersonation = null;

      if (recordExit) {
        // Запись выхода в журнал - смысл всей затеи, поэтому её провал не
        // проглатывается молча: администратор узнаёт, что окно режима осталось
        // в журнале незакрытым, и может сказать об этом.
        const res = await stopImpersonation().catch(() => null);
        if (!res || !res.ok) {
          useDeletionsStore().notify({
            prefix: 'Выход из режима не записан в журнал',
            type: 'warning',
          });
        }
      }

      const restored = await tryRestoreSession();
      if (!restored) {
        this.clearTokens();
        return false;
      }
      await this.reloadIdentity();
      return true;
    },

    /**
     * Перечитывает всё, что зависит от того, кем мы работаем: тип пользователя и
     * права. Без этого после смены личности интерфейс рисуется по чужому набору
     * прав - и показывает разделы, которых у человека нет.
     */
    async reloadIdentity() {
      this.userTypeCode = null;
      const permissions = usePermissionsStore();
      permissions.clearPermissions();
      await Promise.all([
        this.loadUserTypeCode(),
        permissions.fetchPermissions(true),
      ]);
    },
    async loadUserTypeCode() {
      // Общий in-flight промис: App на старте сессии и онбординг перед выбором
      // тура спрашивают тип пользователя одновременно - без него это два
      // одинаковых GET /users/me.
      if (userTypePromise) return userTypePromise;
      userTypePromise = (async () => {
        try {
          // getMe() уже снимает envelope и возвращает данные (как getMyPermissions);
          // повторный .json() здесь дал бы TypeError и userTypeCode остался бы null.
          const data = await getMe();
          this.userTypeCode = data?.user_type_code ?? null;
        } catch {
          // сеть упала или токен протух - оставляем null, не блокируем рендер
        } finally {
          userTypePromise = null;
        }
      })();
      return userTypePromise;
    },
  },
});
