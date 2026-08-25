package models

import (
	"time"

	"systemburo/internal/crypto"

	"gorm.io/gorm"
)

type Employee struct {
	ID           int         `json:"id"`
	AttachmentID *int        `gorm:"index" json:"attachment_id"`
	Attachment   *Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// SupplementID - каким дополнением заявки добавлен сотрудник (#1685). NULL - пришёл с
	// исходной подачей. По нему принятие дополнения активирует только его строки, а
	// интерфейс выделяет новых. Без FK: дополнения не удаляются, отмена у них - статус.
	SupplementID             *int         `gorm:"index" json:"supplement_id"`
	LastName                 *string      `gorm:"size:100" json:"last_name"`
	FirstName                *string      `gorm:"size:100" json:"first_name"`
	MiddleName               *string      `gorm:"size:100" json:"middle_name"`
	CitizenshipID            *int         `gorm:"index" json:"citizenship_id"`
	Citizenship              *Citizenship `json:"-"`
	Position                 *string      `gorm:"size:100;column:position" json:"position"`
	PassportSeriesNumber     *string      `gorm:"type:text" json:"passport_series_number"`
	PatentNumber             *string      `gorm:"type:text" json:"patent_number"`
	PassportSeriesNumberHMAC *string      `gorm:"size:64;index" json:"-"`
	PatentNumberHMAC         *string      `gorm:"size:64;index" json:"-"`
	OtherPermission          *string      `gorm:"type:text" json:"other_permission"`
	TerritoryEntryTime       *time.Time   `json:"territory_entry_time"`
	TerritoryStatus          *int         `json:"territory_status"`
	Status                   *int         `gorm:"index" json:"status"`
	DateCreated              *time.Time   `json:"date_created"`
	DateDeleted              *time.Time   `json:"date_deleted"`
	// PDConsentAt - когда заявитель подтвердил, что субъект дал согласие на обработку
	// своих персональных данных (152-ФЗ), PDConsentByUserID - кто подтвердил. Время и
	// автор ставит СЕРВЕР: в запросе только флаг, иначе дату согласия можно было бы
	// прислать любую. NULL - отметки нет (запись заведена до введения поля или
	// администратор снял обязательность в шаблоне вложения).
	PDConsentAt       *time.Time `json:"pd_consent_at"`
	PDConsentByUserID *int       `json:"pd_consent_by_user_id"`
	// IsPurged - финальное удаление из корзины (#186). Запись остаётся в БД для
	// аудита, но скрывается даже из корзины. Восстановление невозможно.
	IsPurged       bool       `gorm:"default:false;index" json:"is_purged"`
	PurgedAt       *time.Time `json:"purged_at,omitempty"`
	PurgedByUserID *int       `json:"purged_by_user_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (e *Employee) BeforeSave(tx *gorm.DB) error {
	if e.PassportSeriesNumber != nil {
		e.PassportSeriesNumberHMAC = crypto.HMACOptional(e.PassportSeriesNumber)
		enc, err := crypto.EncryptOptional(e.PassportSeriesNumber)
		if err != nil {
			return err
		}
		e.PassportSeriesNumber = enc
	}
	if e.PatentNumber != nil {
		e.PatentNumberHMAC = crypto.HMACOptional(e.PatentNumber)
		enc, err := crypto.EncryptOptional(e.PatentNumber)
		if err != nil {
			return err
		}
		e.PatentNumber = enc
	}
	return nil
}

func (e *Employee) AfterFind(tx *gorm.DB) error {
	e.PassportSeriesNumber = crypto.DecryptOptional(e.PassportSeriesNumber)
	e.PatentNumber = crypto.DecryptOptional(e.PatentNumber)
	return nil
}

type UniqueEmployee struct {
	ID                       int           `json:"id"`
	LastName                 *string       `gorm:"size:100" json:"last_name"`
	FirstName                *string       `gorm:"size:100" json:"first_name"`
	MiddleName               *string       `gorm:"size:100" json:"middle_name"`
	CitizenshipID            *int          `gorm:"index" json:"citizenship_id"`
	Citizenship              *Citizenship  `json:"-"`
	Position                 *string       `gorm:"size:100;column:position" json:"position"`
	PassportSeriesNumber     *string       `gorm:"type:text" json:"passport_series_number"`
	PatentNumber             *string       `gorm:"type:text" json:"patent_number"`
	PassportSeriesNumberHMAC *string       `gorm:"size:64;index" json:"-"`
	PatentNumberHMAC         *string       `gorm:"size:64;index" json:"-"`
	OtherPermission          *string       `gorm:"type:text" json:"other_permission"`
	OrganizationID           *int          `gorm:"index" json:"organization_id"`
	Organization             *Organization `json:"-"`
	CompanyID                *int          `gorm:"index" json:"company_id"`
	Company                  *Company      `json:"-"`
	UserID                   *int          `gorm:"index" json:"user_id"`
	User                     *User         `json:"-"`
	// Согласие субъекта на обработку персональных данных: см. Employee.PDConsentAt.
	// На записи реестра отметка живёт своей жизнью - реестр и строки заявок не связаны,
	// подача заявки в реестр не пишет.
	PDConsentAt       *time.Time `json:"pd_consent_at"`
	PDConsentByUserID *int       `json:"pd_consent_by_user_id"`
	Status            *bool      `gorm:"default:false" json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (e *UniqueEmployee) BeforeSave(tx *gorm.DB) error {
	if e.PassportSeriesNumber != nil {
		e.PassportSeriesNumberHMAC = crypto.HMACOptional(e.PassportSeriesNumber)
		enc, err := crypto.EncryptOptional(e.PassportSeriesNumber)
		if err != nil {
			return err
		}
		e.PassportSeriesNumber = enc
	}
	if e.PatentNumber != nil {
		e.PatentNumberHMAC = crypto.HMACOptional(e.PatentNumber)
		enc, err := crypto.EncryptOptional(e.PatentNumber)
		if err != nil {
			return err
		}
		e.PatentNumber = enc
	}
	return nil
}

func (e *UniqueEmployee) AfterFind(tx *gorm.DB) error {
	e.PassportSeriesNumber = crypto.DecryptOptional(e.PassportSeriesNumber)
	e.PatentNumber = crypto.DecryptOptional(e.PatentNumber)
	return nil
}

type ApplicationEmployee struct {
	ID                       int        `json:"id"`
	AttachmentID             int        `gorm:"index" json:"attachment_id"`
	Attachment               Attachment `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	LastName                 *string    `gorm:"size:100" json:"last_name"`
	FirstName                *string    `gorm:"size:100" json:"first_name"`
	MiddleName               *string    `gorm:"size:100" json:"middle_name"`
	Position                 *string    `gorm:"size:100;column:position" json:"position"`
	CitizenshipID            *int       `json:"citizenship_id"`
	PassportSeriesNumber     *string    `gorm:"type:text" json:"passport_series_number"`
	PatentNumber             *string    `gorm:"type:text" json:"patent_number"`
	PassportSeriesNumberHMAC *string    `gorm:"size:64;index" json:"-"`
	PatentNumberHMAC         *string    `gorm:"size:64;index" json:"-"`
	OtherPermission          *string    `gorm:"type:text" json:"other_permission"`
	OrderIndex               *int       `json:"order_index"`
	CreatedAt                time.Time  `json:"created_at"`
}

func (e *ApplicationEmployee) BeforeSave(tx *gorm.DB) error {
	if e.PassportSeriesNumber != nil {
		e.PassportSeriesNumberHMAC = crypto.HMACOptional(e.PassportSeriesNumber)
		enc, err := crypto.EncryptOptional(e.PassportSeriesNumber)
		if err != nil {
			return err
		}
		e.PassportSeriesNumber = enc
	}
	if e.PatentNumber != nil {
		e.PatentNumberHMAC = crypto.HMACOptional(e.PatentNumber)
		enc, err := crypto.EncryptOptional(e.PatentNumber)
		if err != nil {
			return err
		}
		e.PatentNumber = enc
	}
	return nil
}

func (e *ApplicationEmployee) AfterFind(tx *gorm.DB) error {
	e.PassportSeriesNumber = crypto.DecryptOptional(e.PassportSeriesNumber)
	e.PatentNumber = crypto.DecryptOptional(e.PatentNumber)
	return nil
}

type EmployeeFile struct {
	ID         int       `json:"id"`
	EmployeeID int       `gorm:"index" json:"employee_id"`
	Employee   Employee  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	FilePath   string    `gorm:"size:500" json:"file_path"`
	FileType   *string   `gorm:"size:50" json:"file_type"`
	FileName   *string   `gorm:"size:255" json:"file_name"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// EmployeeTargetTable -- привязка сотрудника к таблице проходной. Source различает
// откуда взялась привязка (#1227): application - из поданной заявки (submit пишет
// строку сырым SQL, дефолт колонки application подставится автоматически), manual -
// ручное добавление/перенос/групповая операция. Зеркало CarTargetTable.
type EmployeeTargetTable struct {
	ID         int    `json:"id"`
	EmployeeID int    `gorm:"index" json:"employee_id"`
	TableID    int    `gorm:"index" json:"table_id"`
	OrderIndex *int   `json:"order_index"`
	Source     string `gorm:"type:varchar(20);not null;default:application" json:"source"`
}
