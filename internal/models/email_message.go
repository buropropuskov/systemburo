package models

import (
	"fmt"
	"time"

	"systemburo/internal/crypto"

	"gorm.io/gorm"
)

// Статусы письма в очереди.
const (
	EmailStatusPending = "pending"
	EmailStatusSent    = "sent"
	EmailStatusFailed  = "failed"
)

// EmailMessage - письмо в очереди на отправку.
//
// Очередь здесь не про производительность, а про то, чтобы письмо не потерялось.
// Плановая смена пароля кладёт новый хэш и строку этой таблицы одной транзакцией:
// прошла - на диске и пароль, и задание на письмо; упала посередине - откатилось
// всё вместе. Прямая отправка из сервиса даёт учётную запись с новым паролем и
// потерянным письмом, то есть человека, запертого снаружи.
type EmailMessage struct {
	ID int `json:"id"`
	// ToAddress - адрес получателя на момент постановки в очередь. Хранится строкой,
	// а не берётся из карточки при отправке: адрес могли поменять между постановкой
	// и доставкой, и письмо ушло бы не туда, куда решала система.
	ToAddress string `gorm:"size:255;not null;index" json:"to_address"`
	// UserID - кому предназначалось письмо. Нужен для отчёта администратору
	// «кому не доставлено». Пустой у писем, не связанных с учётной записью
	// (проверочное письмо при настройке почты).
	UserID *int `gorm:"index" json:"user_id,omitempty"`
	User    *User `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"-"`
	// TemplateCode - какой текст отправляли. По нему собирается статистика и
	// отбираются письма для повторной отправки одного вида.
	TemplateCode string `gorm:"size:64;index" json:"template_code"`
	Subject      string `gorm:"size:255;not null" json:"subject"`
	// Body хранится шифротекстом (#2351): в письме плановой смены пароля пароль
	// лежит открытым текстом, а дамп базы иначе отдавал бы действующие пароли за
	// всё время. BeforeSave/AfterFind шифруют и расшифровывают его тем же ключом,
	// что паспорт и патент (internal/crypto, AES-256-GCM). Body не отдаётся наружу
	// и в незашифрованном виде: журнал очереди не должен его показывать.
	Body string `gorm:"type:text;not null" json:"-"`
	// Status: pending, sent, failed.
	Status   string `gorm:"size:16;not null;default:'pending';index" json:"status"`
	Attempts int    `gorm:"not null;default:0" json:"attempts"`
	// LastError - текст последнего отказа сервера. Ради него журнал и читают:
	// 535 это неверный логин, 550 - отправитель не совпадает с ящиком.
	LastError string `gorm:"size:500" json:"last_error,omitempty"`
	// NextAttemptAt - не раньше какого момента пробовать снова. Пустой у писем,
	// которые ещё ни разу не пытались отправить.
	NextAttemptAt *time.Time `gorm:"index" json:"next_attempt_at,omitempty"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName - имя таблицы очереди писем.
func (EmailMessage) TableName() string { return "email_messages" }

// BeforeSave шифрует тело письма перед записью. Пустое тело (письмо уже
// отправлено или отказ исчерпал попытки, см. mail_service.go) не шифруется -
// шифровать нечего, а пустая строка обязана остаться пустой строкой.
func (m *EmailMessage) BeforeSave(tx *gorm.DB) error {
	if m.Body == "" {
		return nil
	}
	enc, err := crypto.EncryptOptional(&m.Body)
	if err != nil {
		return fmt.Errorf("шифрование тела письма: %w", err)
	}
	m.Body = *enc
	return nil
}

// AfterFind расшифровывает тело письма после чтения. Расшифровка использует
// DecryptOptional: неудача (чужой ключ, письмо, поставленное в очередь до
// включения шифрования) не должна ронять всю выборку - воркер разбирает письма
// пачкой, и одна нечитаемая запись не обязана останавливать остальные. Пустое
// тело decrypt не проходит намеренно: у отправленных и окончательно
// недоставленных писем оно уже стёрто, и это не ошибка ключа.
func (m *EmailMessage) AfterFind(tx *gorm.DB) error {
	if m.Body == "" {
		return nil
	}
	m.Body = *crypto.DecryptOptional(&m.Body)
	return nil
}
