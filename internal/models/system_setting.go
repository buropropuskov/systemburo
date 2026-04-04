package models

type SystemSetting struct {
	ID    int    `json:"id"`
	Key   string `gorm:"uniqueIndex;size:100" json:"key"`
	Value string `gorm:"type:text" json:"value"`
	Type  string `gorm:"size:20" json:"type"`
}

func (SystemSetting) TableName() string { return "system_settings" }

type UpdateSettingRequest struct {
	Value string `json:"value" validate:"required"`
}
