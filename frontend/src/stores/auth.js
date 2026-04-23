import { defineStore } from 'pinia';

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
    isAdmin() {
      return this.userType === 6;
    },
    username() {
      return this.userPayload?.username || null;
    },
  },

  actions: {
    setTokens(token) {
      this.token = token;
    },
    clearTokens() {
      this.token = null;
    },
  },
});
