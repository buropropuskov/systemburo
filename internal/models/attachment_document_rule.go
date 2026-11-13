package models

// AttachmentDocumentRule allows applicants to choose a document check at the post.
type AttachmentDocumentRule struct {
	UniqueAttachmentID int    `gorm:"primaryKey;autoIncrement:false"`
	Directory          string `gorm:"primaryKey;size:16"`
	DirectoryID        int    `gorm:"primaryKey;autoIncrement:false"`
}
