import { defineStore } from 'pinia';

function decodeToken(token) {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch {
    return null;
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    refreshToken: localStorage.getItem('refreshToken') || null,
  }),

  getters: {
    isAuthenticated() {
      if (!this.token) return false;
      const payload = decodeToken(this.token);
      return payload && payload.exp > Math.floor(Date.now() / 1000);
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
    setTokens(token, refreshToken) {
      this.token = token;
      this.refreshToken = refreshToken;
      localStorage.setItem('token', token);
      localStorage.setItem('refreshToken', refreshToken);
    },
    clearTokens() {
      this.token = null;
      this.refreshToken = null;
      localStorage.removeItem('token');
      localStorage.removeItem('refreshToken');
    },
  },
});
