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
            return JSON.stringify([attachment.template_id || attachment.id, this.organizationId, this.companyId]);
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
            const [templateId, organizationId, companyId] = JSON.parse(this.documentRuleKey);
            if (!organizationId && !companyId) return;
            const params = new URLSearchParams();
            if (organizationId) params.set('organization_id', organizationId);
            if (companyId) params.set('company_id', companyId);
            try {
                const response = await apiRequest(`/attachments/${templateId}/document-at-post?${params}`, { silent403: true });
                const result = await response.json();
                if (sequence === this.documentRuleSequence) this.documentAtPostAllowed = result.allowed === true;
            } catch (error) {
                if (sequence === this.documentRuleSequence) console.error('Не удалось проверить настройку предъявления документа:', error);
            }
        },
    },
};
