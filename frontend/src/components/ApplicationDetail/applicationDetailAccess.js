import { apiRequest } from '@/api/client';
import eventStream from '@/services/eventStream';
import { useDeletionsStore } from '@/stores/deletions';

const DETAIL_ACCESS_REFRESH_MS = 60_000;

export default {
    data() {
        return {
            detailUnavailable: false,
            accessReadSeq: 0,
            confirmedAccessSeq: 0,
            lastDetailRead: 0,
            accessCheckTimer: null,
            accessProbeId: null,
            eventStreamOff: null,
        };
    },
    mounted() {
        eventStream.connect();
        document.addEventListener('visibilitychange', this.recheckOpenApplication);
        window.addEventListener('online', this.recheckOpenApplication);
        // Участник, которого убрали из заявки, может уже не входить в аудиторию SSE.
        this.accessCheckTimer = setInterval(() => {
            if (Date.now() - this.lastDetailRead >= DETAIL_ACCESS_REFRESH_MS) this.recheckOpenApplication();
        }, DETAIL_ACCESS_REFRESH_MS);
    },
    beforeUnmount() {
        this.eventStreamOff?.();
        eventStream.disconnect();
        this.visible = false;
        this.loadDetailSeq++;
        this.loadAttachmentSeq++;
        clearInterval(this.accessCheckTimer);
        document.removeEventListener('visibilitychange', this.recheckOpenApplication);
        window.removeEventListener('online', this.recheckOpenApplication);
    },
    methods: {
        subscribeToApplication(id) {
            this.eventStreamOff?.();
            this.eventStreamOff = id
                ? eventStream.subscribe(`application:${id}`, () => this.refreshLiveDetail(), { fallback: true })
                : null;
        },

        activateDetail(id, previousId) {
            if (previousId !== id) this.confirmedAccessSeq = this.accessReadSeq;
            if (!this.detailUnavailable) return true;
            if (previousId === id) return false;
            this.detailUnavailable = false;
            this.visible = true;
            return true;
        },

        async readOpenApplication(id) {
            const seq = ++this.accessReadSeq;
            this.lastDetailRead = Date.now();
            const response = await apiRequest(`/applications/${id}/details`, { method: 'GET', silent403: true });
            if (!this.visible || id !== this.applicationData.id || seq <= this.confirmedAccessSeq) return null;
            // Только успешное более новое чтение отменяет поздний отказ для этой заявки.
            if (response.ok) this.confirmedAccessSeq = seq;
            if (response.status !== 403 && response.status !== 404) return response;
            this.detailUnavailable = true;
            this.visible = false;
            this.loadDetailSeq++;
            this.applicationData = {};
            this.attachments = [];
            this.clearAttachmentSelection();
            useDeletionsStore().notify({ bold: 'Заявка больше недоступна', type: 'warning' });
            this.$emit('close');
            return null;
        },

        async recheckOpenApplication() {
            const id = this.applicationData.id;
            if (!this.visible || !id || document.hidden || this.accessProbeId === id) return;
            this.accessProbeId = id;
            try {
                await this.readOpenApplication(id);
            } catch {
                // Временная ошибка соединения не меняет право на открытую заявку.
            } finally {
                if (this.accessProbeId === id) this.accessProbeId = null;
            }
        },

        clearAttachmentSelection() {
            this.loadAttachmentSeq++;
            this.loadingAttachmentDetails = false;
            this.selectedAttachment = null;
            this.attachmentCars = [];
            this.attachmentEmployees = [];
            this.attachmentItems = [];
            this.showVehicleModal = false;
            this.showEmployeeModal = false;
            this.selectedVehicle = null;
            this.selectedEmployee = null;
            this.showRemovalModal = false;
            this.showOverrideModal = false;
        },
    }
};
