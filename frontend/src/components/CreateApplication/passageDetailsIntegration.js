import { setModalOpen } from '@/utils/modalStack';
import { CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS, historyActionText, isPassageHistoryAction, correctionStatus } from '@/utils/passageHistoryActions';
export default {
  computed: { overlayZIndex() { return this.source === 'history' ? 14000 : this.source === 'application' ? 10003 : 10001; } },
  data() { return { currentPassageState: null }; },
  watch: { overlayZIndex(value) { if (this.employee) setModalOpen(this, this.show, value); }, show() { this.currentPassageState = null; }, vehicle() { this.currentPassageState = null; }, employee() { this.currentPassageState = null; } },
  methods: {
    isPassageHistoryAction, correctionStatus,
    getActionClass(item) {
      if (!item.user_id) return 'dot-system';
      return { entry: 'dot-entry', exit: 'dot-exit' }[item.action_type] || 'dot-default';
    },
    passageHistoryText(item) { return historyActionText(item, this.vehicle ? CAR_HISTORY_ACTIONS : EMPLOYEE_HISTORY_ACTIONS); },
  },
};
