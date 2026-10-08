package services

// forwardRecipientDetail captures the purpose of this particular forwarding
// action. Names are deliberately absent: read surfaces resolve privacy-safe names
// using the stable user ID, without inferring historical purpose from current roles.
type forwardRecipientDetail struct {
	UserID           int    `json:"user_id"`
	Purpose          string `json:"purpose"`
	RequiredApproval bool   `json:"required_approval"`
	AccessGranted    bool   `json:"access_granted"`
}

type forwardAttachmentDetail struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// appendForwardRecipient aggregates the roles actually accepted in this action.
// The service may accept both viewing and approval for one ID; approval describes
// that recipient's resulting assignment and must not be hidden by an earlier view.
// Repeated viewing preserves whether this action granted access at least once.
func appendForwardRecipient(details []forwardRecipientDetail, detail forwardRecipientDetail) []forwardRecipientDetail {
	for i, existing := range details {
		if existing.UserID == detail.UserID {
			if existing.Purpose == "view" && detail.Purpose == "approval" {
				details[i] = detail
			} else if existing.Purpose == "view" && detail.Purpose == "view" {
				details[i].AccessGranted = existing.AccessGranted || detail.AccessGranted
			}
			return details
		}
	}
	return append(details, detail)
}

func forwardAttachmentScope(requestedIDs []int) string {
	if len(requestedIDs) == 0 {
		return "all"
	}
	return "selected"
}
