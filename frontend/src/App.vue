<template>
  <div id="app">
    <NavMenu 
      v-if="isAuthenticated"
      :is-buropropuskov="isBuropropuskov"
      @logout="logout"
    />
    <div class="content">
      <TheHeader class="theheader" v-if="isAuthenticated"/>
       <router-view class="content__container" @login-success="handleSuccessfulLogin" /> 
    </div>
   
  </div>
</template>

<script>
import NavMenu from './components/NavMenu.vue';
import TheHeader from './components/TheHeader/TheHeader.vue';

export default {
  name: "App",
  components: {
    NavMenu,
    TheHeader
  },
  data() {
    return {
      isAuthenticated: false,
      isBuropropuskov: false
    };
  },
  methods: {
    async checkAuthStatus() {
      const token = localStorage.getItem("token");
      
      this.isAuthenticated = !!token;
      
      if (token) {
        try {
          const payload = JSON.parse(atob(token.split('.')[1]));
          this.isBuropropuskov = payload.type_id === 6;
        } catch (e) {
          console.error("Token decode error:", e);
          this.isBuropropuskov = false;
        }
      } else {
        this.isBuropropuskov = false;
      }
    },
    handleSuccessfulLogin(token) {
      localStorage.setItem("token", token);
      this.checkAuthStatus();
    },
    logout() {
      localStorage.removeItem("token");
      this.checkAuthStatus();
      this.$router.push("/");
    }
  },
  created() {
    this.checkAuthStatus();
    this.$router.afterEach(() => {
      this.checkAuthStatus();
    });
  },
  watch: {
    $route() {
      this.checkAuthStatus();
    }
  }
};
</script>

<style>
* {
    font-family: 'Montserrat', sans-serif;
    padding: 0;
    margin: 0;
    box-sizing: border-box;
    scroll-behavior: smooth;
}

::-webkit-scrollbar {
  width: 0;
}

/* Динамический отступ только для авторизованных пользователей */
body.auth-active #app {
  margin-left: 25px;
}

body:not(.auth-active) #app {
  margin-left: 0;
}

.form-input-sm {
  border-radius: 10px !important;
}

.blue {
  color: #4F5BDF;
}

.red {
  color: rgb(241, 76, 76);
}

</style>