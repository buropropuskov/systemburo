<template>
  <AdminPageShell>
    <UserControl
      :all-users="allUsers"
      @fetch-users="fetchAllUsers"
      @user-updated="handleUserUpdated"
    />
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import UserControl from '@/components/UserControl.vue';
import { apiRequest } from '@/api/client';

/**
 * Страница /admin/users (#510): хостит кабинетный UserControl как самостоятельную
 * страницу. Логика списка вынесена из AccountComponent - держим allUsers, грузим
 * на маунте и по @fetch-users, синхронизируем запись по @user-updated. Сам
 * UserControl не меняется.
 */
export default {
  name: 'UserControlView',
  components: { AdminPageShell, UserControl },
  data() {
    return {
      allUsers: [],
    };
  },
  mounted() {
    this.fetchAllUsers();
  },
  methods: {
    async fetchAllUsers() {
      try {
        // include_archived=true: UserControl фильтрует активных/архивных клиентски.
        const response = await apiRequest('/users/all?include_archived=true', { method: 'GET' });
        if (!response.ok) {
          console.error('Не удалось загрузить пользователей:', response.status);
          return;
        }
        const data = await response.json();
        if (!Array.isArray(data)) {
          // success:false приходит c HTTP 200 -> response.ok не ловит, проверяем форму.
          console.error('Не удалось загрузить пользователей:', data?.message || data);
          return;
        }
        this.allUsers = data.map(user => ({ ...user, newPassword: '' }));
      } catch (error) {
        console.error('Ошибка сети при загрузке пользователей:', error);
      }
    },
    handleUserUpdated(updatedUser) {
      const index = this.allUsers.findIndex(u => u.username === updatedUser.username);
      if (index !== -1) {
        this.allUsers[index] = { ...this.allUsers[index], ...updatedUser };
      }
    },
  },
};
</script>

<style scoped>
/* UserControl сверстан под кабинетную карточку с фикс-высотой тела (258px).
   На full-height странице тянем мастер-детейл и тело таблицы на высоту карточки
   (flex-grow у .users-body уже есть в компоненте) - чтобы не оставался пустой
   хвост снизу. Сам компонент не трогаем, оверрайд скоуплен на /admin/users. */
:deep(.users-container) {
  flex: 1 1 auto;
  min-height: 0;
  max-height: none;
  height: auto;
}

:deep(.users-body) {
  height: auto;
  max-height: none;
}
</style>
