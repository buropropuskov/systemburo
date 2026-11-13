import { apiRequest } from '@/api/client';

export default {
    data() {
        return {
            documentAtPostAllowed: false,
            documentRuleSequence: 0,
        };
    },
    computed: {
        documentRuleKey() {
            const attachment = this.selectedAttachment;
            if (attachment?.attachment_type !== 'people') return '';
            return attachment.template_id || attachment.unique_attachment_id || attachment.id;
        },
    },
    watch: {
        documentRuleKey: {
            immediate: true,
            handler() { this.loadDocumentRule(); },
        },
    },
    methods: {
        async loadDocumentRule() {
            const sequence = ++this.documentRuleSequence;
            this.documentAtPostAllowed = false;
            if (!this.documentRuleKey) return;
            const templateId = this.documentRuleKey;
            try {
                const response = await apiRequest(`/attachments/${templateId}/document-at-post`, { silent403: true });
                const result = await response.json();
                if (sequence === this.documentRuleSequence) this.documentAtPostAllowed = result.allowed === true;
            } catch (error) {
                if (sequence === this.documentRuleSequence) console.error('Не удалось проверить настройку предъявления документа:', error);
            }
        },
    },
};
