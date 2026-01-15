<template>
  <div class="account__settings">
    <div class="settings__container">
      <img src="@/assets/icons/settings.png" class="settings__icon" />
      <h2 class="settings__title">Управление и настройка системы</h2>
    </div>
    <ul class="settings__navigation">
      <li class="navigation__link">
        <a href="#users" class="link">Пользователи</a>
      </li>
      <li class="navigation__link">
        <a href="#organizations" class="link">Организации</a>
      </li>
      <li class="navigation__link">
        <a href="#companies" class="link">Компании</a>
      </li>
      <li class="navigation__link">
        <a href="#" class="link disabled">Доступ</a>
      </li>
      <li class="navigation__link">
        <a href="#tables" class="link">Таблицы</a>
      </li>
      <li class="navigation__link">
        <a href="#number" class="link">Авто-номера</a>
      </li>
      <li class="navigation__link">
        <a href="#unload_place" class="link">Разгрузка</a>
      </li>
      <li class="navigation__link">
        <a href="#" class="link disabled">Уведомления</a>
      </li>
      <li class="navigation__link">
        <a href="#attachments" class="link">Бланки</a>
      </li>
       <li class="navigation__link">
        <a href="#" class="link disabled">Карта</a>
      </li>
      <li class="navigation__link">
        <a href="#user_types" class="link">Типы аккаунта</a>
      </li>
    </ul>
  </div>
</template>

<script>
export default {
  name: 'AccountSettings',
  data() {
    return {};
  }
};
</script>

<style scoped>
.account__settings {
  display: flex;
  width: 100%;
  gap: 0;
  position: relative;
  background: transparent;
}

.settings__container {
  min-width: 360px;
  height: 50px;
  background-color: #4F5BDF;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.settings__icon {
  width: 22px;
  height: 22px;
  animation: settings_rotate 8s linear infinite;
}

.settings__title {
  font-size: 16px;
  color: #FFF;
}

.settings__navigation {
  width: 100%;
  height: 30px;
  border-top: 2px solid #4F5BDF;
  margin-top: auto;
  display: flex;
  list-style: none;
  justify-content: space-between;
  padding: 5px 10px;
}

.link {
  font-size: 11px;
  font-weight: 600;
  color: #939CFF;
  transition: .2s;
  text-decoration: none;
}

.link:hover:not(.disabled) {
  color: #4F5BDF;
}

.disabled {
  color: #e6e6e6;
  cursor: not-allowed;
}

@keyframes settings_rotate {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Адаптивность */
@media (max-width: 1200px) {
  .settings__container {
    min-width: 300px;
  }
}

@media (max-width: 768px) {
  .account__settings {
    flex-direction: column;
    gap: 10px;
  }
  
  .settings__container {
    min-width: auto;
    width: 100%;
  }
  
  .settings__navigation {
    flex-wrap: wrap;
    height: auto;
    gap: 10px;
  }
}

@media (max-width: 480px) {
  .settings__title {
    font-size: 16px;
  }
  
  .link {
    font-size: 11px;
  }
}
</style>