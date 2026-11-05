// Только актуальная загрузка списка обрабатывает накопленные realtime-сигналы.
export default {
  data() {
    return { realtimeRefreshPending: false }
  },
  beforeUnmount() {
    this.realtimeRefreshPending = false
  },
  methods: {
    async handleRealtimeRefresh() {
      if (this.isInitialLoad || this.loading) {
        this.realtimeRefreshPending = true
        return
      }
      await this.refreshFromRealtime()
    },
    async completeRealtimeInitialLoad(seq) {
      if (seq !== this.fetchSeq) return
      this.loading = false
      this.isInitialLoad = false
      if (this.realtimeRefreshPending) {
        this.realtimeRefreshPending = false
        await this.refreshFromRealtime()
      }
    },
  },
}
