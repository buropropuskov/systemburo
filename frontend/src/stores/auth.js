import { defineStore } from 'pinia';
import { getMe } from '@/api/auth';
// Циклический импорт с permissions.js (он импортит useAuthStore) безопасен:
// обе стороны зовут стор только в рантайме (getter / parsePermissionsResponse),
// не на этапе инициализации модуля - ESM live bindings уже заполнены к вызову.
import { usePermissionsStore } from './permissions';

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
    username() {
      return this.userPayload?.username || null;
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
