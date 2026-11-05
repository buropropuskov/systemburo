import { recoverApplicationCommand } from '@/services/applicationCommand'

// Обычная подача и восстановленная команда переходят в одно состояние успеха.
export default {
  methods: {
    async recoverSubmittedCommand(options) {
      const result = await recoverApplicationCommand('application.submit', options);
      if (!result) return false;
      this.acceptSubmittedApplication(result);
      return true;
    },
    acceptSubmittedApplication(result) {
      // Файлы уже привязаны к заявке - список в форме больше не нужен.
      this.applicationFileIds = [];
      this.$refs.filesUpload?.reset();
      this.createdApplicationNumber = result.application_number;
      this.createdAttachmentsData = this.attachments.map(att => {
        const dateData = this.attachmentDatesByAttachment[this.attachmentKey(att)] || this.getDefaultDateData();
        let period = '';
        let time = '';

        if (dateData.isOneDay) {
          period = dateData.singleDate || '';
        } else if (dateData.startDate || dateData.endDate) {
          period = `${dateData.startDate || ''} - ${dateData.endDate || ''}`.trim();
        }

        if (dateData.startTime && dateData.endTime) {
          time = `${dateData.startTime} - ${dateData.endTime}`;
        }

        return {
          display_name: att.display_name || att.name,
          period,
          time
        };
      });
      this.showSuccessModal = true;
    },
  },
}
