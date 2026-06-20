import { defineStore } from 'pinia';
import { getMe } from '@/api/auth';

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
    canViewAccessibleAttachments() {
      return this.isSecurity || this.isSuperAdmin;
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
      try {
        const res = await getMe();
        const data = await res.json();
        this.userTypeCode = data?.user_type_code ?? null;
      } catch {
        // сеть упала или токен протух - оставляем null, не блокируем рендер
      }
    },
  },
});
