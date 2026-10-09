package services

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"systemburo/internal/models"
	"time"
)

type PassageOpenService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewPassageOpenService(db *gorm.DB) *PassageOpenService {
	return &PassageOpenService{db: db, now: func() time.Time { return time.Now().UTC() }}
}

type PassageOpenFilter struct {
	View           string
	AttentionOnly  bool
	Search         string
	OrganizationID *int
	Page           int
	PerPage        int
}

type PassageOpenItem struct {
	PassageResult
	AttachmentID      int     `json:"attachment_id"`
	ApplicationID     *int    `json:"application_id"`
	ApplicationNumber *string `json:"application_number"`
	OrganizationID    *int    `json:"organization_id"`
	Organization      *string `json:"organization"`
	DisplayName       string  `json:"display_name"`
}

type PassageOpenCounts struct {
	AllOpen     int64 `json:"all_open"`
	Attention   int64 `json:"attention"`
	UnknownTime int64 `json:"unknown_time"`
}
type PassageOpenList struct {
	Items     []PassageOpenItem `json:"items"`
	Counts    PassageOpenCounts `json:"counts"`
	Page      int               `json:"page"`
	PerPage   int               `json:"per_page"`
	Total     int64             `json:"total"`
	ServerNow time.Time         `json:"server_now"`
}

// List with tableID is strictly bound to that table. A nil table selects the
// distinct admin-summary source and requires both managed correction permission
// and the existing admin/receiver element visibility. Totals never escape scope.
func (s *PassageOpenService) List(ctx context.Context, actor int, kind ElementKind, tableID *int, f PassageOpenFilter) (*PassageOpenList, error) {
	if err := passageIdentity(actor, kind, 1); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("passage list database required")
	}
	if f.View == "" {
		f.View = "open"
	}
	if f.View != "open" && f.View != "corrections" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректный вид списка")
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 50
	}
	if f.PerPage > 100 {
		f.PerPage = 100
	}
	if f.Page > 1000000 || len([]rune(f.Search)) > 200 || f.OrganizationID != nil && *f.OrganizationID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Некорректные фильтры")
	}
	var now time.Time
	out := &PassageOpenList{Items: []PassageOpenItem{}, Page: f.Page, PerPage: f.PerPage, ServerNow: now}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolver := NewPermissionResolver(tx)
		set, err := resolver.Resolve(ctx, actor)
		if err != nil {
			return err
		}
		if set.IsBanned() {
			return echo.NewHTTPError(http.StatusForbidden, "Нет доступа")
		}
		// Resolve has established the repeatable-read snapshot before the clock.
		now = s.now().UTC()
		out.ServerNow = now
		join, err := PassageProjectionSQL(kind, "e", "p")
		if err != nil {
			return err
		}
		period, err := EntityEffectivePeriodSQL("e", "a")
		if err != nil {
			return err
		}
		name := "COALESCE(e.car_number,'')"
		removed := "e.date_removed IS NULL"
		if kind == ElementEmployee {
			name = "CONCAT_WS(' ',e.last_name,e.first_name,e.middle_name)"
			removed = "e.date_deleted IS NULL"
		}
		base := tx.Table(string(kind) + " e").Joins("JOIN attachments a ON a.id=e.attachment_id").Joins("LEFT JOIN applications app ON app.id=a.application_id").Joins("LEFT JOIN organizations o ON o.id=COALESCE(app.organization_id,a.organization_id)").Joins(join).Where("NOT e.is_purged")
		tableName := ""
		if tableID != nil {
			var table models.SystemTable
			if *tableID <= 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "Некорректная таблица")
			}
			if err := tx.Select("id,name,table_type,is_active,status").First(&table, *tableID).Error; err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
			}
			if !table.IsActive || table.Status != "active" || !set.Has("table."+table.Name+".view") || kind == ElementCar && table.TableType != models.TableTypeCars || kind == ElementEmployee && table.TableType != models.TableTypePeople {
				return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
			}
			tableName = table.Name
			binding := elementBindingTables[kind]
			base = base.Where("EXISTS(SELECT 1 FROM "+binding.table+" b WHERE b."+binding.column+"=e.id AND b.table_id=?)", *tableID).Where(removed)
			archive, args := archivedApplicationCond("app")
			base = base.Where("a.is_manual OR (app.status IN ? AND app.confirmation=? AND NOT "+archive+")", append([]any{[]string{models.StatusInWork, models.StatusCompleted}, models.ConfirmationApproved}, args...)...)
		} else {
			if !set.Has(passageCorrectionPermission) {
				return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
			}
			scopes := NewElementScopeResolver(tx, resolver)
			scope, err := scopes.Resolve(ctx, actor)
			if err != nil {
				return err
			}
			if !set.IsAdmin() && !set.IsSuperAdmin() && !scope.Approver {
				return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к административной сводке")
			}
			visible, args := scope.Predicate(kind, "e", "a", "app")
			base = base.Where(visible, args...)
		}
		if f.View == "corrections" && !set.Has(passageCorrectionPermission) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		openSQL := "(p.action='entry' OR (p.id IS NULL AND e.territory_status=1))"
		attentionSQL := "(p.action='entry' AND p.created_at < ?)"
		if err := base.Session(&gorm.Session{}).Select("COUNT(*) FILTER(WHERE "+openSQL+") AS all_open,COUNT(*) FILTER(WHERE "+attentionSQL+") AS attention,COUNT(*) FILTER(WHERE p.id IS NULL AND e.territory_status=1) AS unknown_time", now.Add(-48*time.Hour)).Scan(&out.Counts).Error; err != nil {
			return err
		}
		filtered := base.Session(&gorm.Session{})
		if f.View == "corrections" {
			filtered = filtered.Where("p.action=? AND p.created_at>=?", PassageCorrectionAction, now.Add(-passageRevertWindow))
		} else {
			filtered = filtered.Where(openSQL)
			if f.AttentionOnly {
				filtered = filtered.Where(attentionSQL, now.Add(-48*time.Hour))
			}
		}
		if f.OrganizationID != nil {
			filtered = filtered.Where("COALESCE(app.organization_id,a.organization_id)=?", *f.OrganizationID)
		}
		if search := strings.TrimSpace(f.Search); search != "" {
			filtered = filtered.Where("("+name+" ILIKE ? OR COALESCE(app.application_number,'') ILIKE ?)", "%"+escapeLikePattern(search)+"%", "%"+escapeLikePattern(search)+"%")
		}
		if err := filtered.Session(&gorm.Session{}).Count(&out.Total).Error; err != nil {
			return err
		}
		var rows []struct {
			Projection                                             passageProjectionRow `gorm:"embedded"`
			AttachmentID                                           int
			ApplicationID                                          *int
			ApplicationNumber                                      *string
			OrganizationID                                         *int
			Organization                                           *string
			DisplayName                                            string
			EntryDateFrom, EntryDateTo, EntryTimeFrom, EntryTimeTo *string
			Bounded                                                bool
			PeriodSource                                           string
			ValidMode                                              bool
			Status                                                 *int
			AttachmentStatus                                       *int
		}
		selection := "e.id AS entity_id,e.territory_status,p.*,e.attachment_id,app.id AS application_id,app.application_number,COALESCE(app.organization_id,a.organization_id) AS organization_id,o.name AS organization," + name + " AS display_name," + period.DateFrom + " AS entry_date_from," + period.DateTo + " AS entry_date_to," + period.TimeFrom + " AS entry_time_from," + period.TimeTo + " AS entry_time_to," + period.Bounded + " AS bounded," + period.Source + " AS period_source," + period.ValidMode + " AS valid_mode,e.status,a.status AS attachment_status"
		if err := filtered.Session(&gorm.Session{}).Select(selection).Order("p.created_at ASC NULLS FIRST,e.id ASC").Limit(f.PerPage).Offset((f.Page - 1) * f.PerPage).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			state, err := projectPassageRow(row.Projection, now)
			if err != nil {
				return err
			}
			effective := models.EffectivePeriod{EntryPeriod: models.EntryPeriod{EntryDateFrom: row.EntryDateFrom, EntryDateTo: row.EntryDateTo, EntryTimeFrom: row.EntryTimeFrom, EntryTimeTo: row.EntryTimeTo}, Bounded: row.Bounded, Source: row.PeriodSource}
			state.CanCorrect = state.Open && set.Has(passageCorrectionPermission)
			state.CanRevertCorrection = row.Projection.Action == PassageCorrectionAction && row.Projection.CreatedAt != nil && now.Sub(*row.Projection.CreatedAt) <= passageRevertWindow && set.Has(passageCorrectionPermission)
			admission := PassageAdmission{}
			if tableID != nil {
				valid, reason := passageWindow(effective, now)
				if !row.ValidMode {
					valid = false
					reason = "invalid_period"
				}
				active := row.Status != nil && *row.Status == 1 && row.AttachmentStatus != nil && *row.AttachmentStatus == 1
				admission = PassageAdmission{CanEnter: valid && active && !state.Open && set.Has("table."+tableName+".entry"), CanExit: state.Open && set.Has("table."+tableName+".exit"), Reason: reason}
			}
			out.Items = append(out.Items, PassageOpenItem{PassageResult: PassageResult{EntityID: row.Projection.EntityID, EntityKind: kind, PassageState: state, EffectivePeriod: effective, Admission: admission, ServerNow: now}, AttachmentID: row.AttachmentID, ApplicationID: row.ApplicationID, ApplicationNumber: row.ApplicationNumber, OrganizationID: row.OrganizationID, Organization: row.Organization, DisplayName: row.DisplayName})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return out, nil
}
