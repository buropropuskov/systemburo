<template>
    <button @click="handleRefresh" class="refresh-btn">
        <img src="@/assets/icons/refresh.png" class="refresh-btn__icon" />
        <p class="refresh-btn__text">Обновить</p>
    </button>
</template>
<script>
export default {
  emits: ['refresh'],
  methods: {
    handleRefresh() {
      this.$emit('refresh');
    }
  }
}
</script>
<style scoped>
    .refresh-btn {
        width: 100px;
        height: 25px;
        border-radius: 50px;
        border: 1px solid #e6e6e6;
        outline: none;
        background-color: #FFF;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 5px;
        cursor: pointer;
        transition: .2s;
    }

    .refresh-btn:hover {
        background-color: #f2f2f2;
    }

    .refresh-btn__icon {
        width: 15px;
        height: 15px;
    }

    .refresh-btn__text {
        color: #4F5BDF;
        font-size: 12px;
        font-weight: 500;
    }
</style>