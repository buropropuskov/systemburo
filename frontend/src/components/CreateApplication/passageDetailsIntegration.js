import { CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS, historyActionText, isPassageHistoryAction, correctionStatus } from '@/utils/passageHistoryActions';
export default {
  data() { return { currentPassageState: null }; },
  watch: { show() { this.currentPassageState = null; }, vehicle() { this.currentPassageState = null; }, employee() { this.currentPassageState = null; } },
  methods: {
    isPassageHistoryAction, correctionStatus,
    getActionClass(item) {
      if (!item.user_id) return 'dot-system';
      return { entry: 'dot-entry', exit: 'dot-exit' }[item.action_type] || 'dot-default';
    },
    passageHistoryText(item) { return historyActionText(item, this.vehicle ? CAR_HISTORY_ACTIONS : EMPLOYEE_HISTORY_ACTIONS); },
  },
};
