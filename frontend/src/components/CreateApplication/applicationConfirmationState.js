export function applicationConfirmationState(...compositionFields) {
    return {
        data: () => ({ consentGiven: null, confirmationRefresh: 0 }),
        watch: Object.fromEntries(compositionFields.map(field => [
            field, { deep: true, handler: 'resetConfirmation' },
        ])),
        methods: {
            resetConfirmation() {
                this.consentGiven = null;
            },
            refreshConfirmation() {
                this.resetConfirmation();
                this.confirmationRefresh++;
            },
        },
    };
}
