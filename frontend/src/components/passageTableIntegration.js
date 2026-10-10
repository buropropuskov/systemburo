import PassageMarkButton from './PassageMarkButton.vue';
import PassageValidityCell from './PassageValidityCell.vue';
import PassageTableTools from './PassageTableTools.vue';
import PassageStateIndicator from './PassageStateIndicator.vue';
import { passageAllowed, passageExpired, passageFields, mergePassageStatus, expectedPassageEvent } from '@/utils/passageProjection';
export default {
  components: { PassageMarkButton, PassageTableTools, PassageStateIndicator, PassageValidityCell },
  methods: { passageAllowed, passageExpired, passageFields, mergePassageStatus, expectedPassageEvent },
};
