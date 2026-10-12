import HistoryCardNotice from './HistoryCardNotice.vue';
import PassageCardRevertAction from '../PassageCardRevertAction.vue';
import DetailCardHeader from './DetailCardHeader.vue';
import { setModalOpen } from '@/utils/modalStack';
import { CAR_HISTORY_ACTIONS, EMPLOYEE_HISTORY_ACTIONS, historyActionText, isPassageHistoryAction, correctionStatus } from '@/utils/passageHistoryActions';
export default {
  components: { PassageCardRevertAction, DetailCardHeader, HistoryCardNotice },
  props: { loading: { type: Boolean, default: false }, loadError: { type: String, default: '' } },
  computed: { overlayZIndex() { return this.source === 'history' ? 14000 : this.source === 'application' ? 10003 : 10001; } },
  data() { return { currentPassageState: null, currentRevertStatus: null, statusReadVersion: 0 }; },
  watch: { overlayZIndex(value) { if (this.$props?.employee) setModalOpen(this, this.show, value); }, show() { this.statusReadVersion++; this.currentPassageState = null; this.currentRevertStatus = null; }, vehicle() { this.statusReadVersion++; this.currentPassageState = null; this.currentRevertStatus = null; }, employee() { this.statusReadVersion++; this.currentPassageState = null; this.currentRevertStatus = null; } },
  methods: {
    refreshPassage() { this.currentRevertStatus = null; (this.$props?.vehicle ? this.loadCarStatus : this.loadEmployeeStatus)(); this.$emit('passage-changed'); },
    isPassageHistoryAction, correctionStatus,
    getActionClass(item) {
      if (!item.user_id) return 'dot-system';
      return { entry: 'dot-entry', exit: 'dot-exit' }[item.action_type] || 'dot-default';
    },
    passageHistoryText(item) { return historyActionText(item, this.$props?.vehicle ? CAR_HISTORY_ACTIONS : EMPLOYEE_HISTORY_ACTIONS); },
  },
};
