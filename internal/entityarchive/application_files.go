package entityarchive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
)

// Уничтожение файлов заявки, обезличенной по сроку хранения (#2355).
//
// Затирание полей в базе диска не касается, а на диске те же сведения лежат дважды.
// Первое - приложенные к заявке документы (UPLOAD_PATH/application_files): скан
// паспорта обезличить по полям нельзя, его можно только уничтожить. Второе -
// корпоративная копия в файловом архиве (ARCHIVE_PATH): заполненный бланк и слепок
// заявка.json, где серия и номер паспорта лежат ОТКРЫТЫМ текстом (решение эпика
// #1615: каталог архива - хранилище того же класса защиты, что и база), а в имени
// самого файла стоит фамилия заявителя. Обезличивание, оставляющее это на диске, -
// обезличивание на словах.
//
// Порядок «сначала диск, потом база» тот же, что у переезда папки заявки в
// blank_export_writer: падение после удаления файла чинится следующим прогоном
// (повторное удаление несуществующего файла - успех), а обратный порядок оставил бы
// на диске файл, про который в системе не осталось записи.

// FilePaths - каталоги, где заявка хранится файлами. Пустое значение означает «этот
// каталог установке не настроен»: чистить нечего, но и молчать нельзя - оператор
// увидит это отдельной строкой предупреждения, а не решит, что файлов не было.
type FilePaths struct {
	// UploadPath - корень загрузок, тот же, что у export/import/purge.
	UploadPath string
	// ArchivePath - корень файлового архива бланков (ARCHIVE_PATH).
	ArchivePath string
}

// FilePurgeResult - что уничтожено (или будет уничтожено при пробном прогоне).
type FilePurgeResult struct {
	// Attached - документы, приложенные к заявке.
	Attached      int
	AttachedBytes int64
	// Archive - файлы файлового архива: бланки и слепок заявка.json.
	Archive      int
	ArchiveBytes int64
	// Skipped - причины, по которым файлы остались на диске.
	Skipped []string
}

// Total - сколько файлов затронуто всего.
func (r FilePurgeResult) Total() int { return r.Attached + r.Archive }

// Bytes - сколько места освобождается.
func (r FilePurgeResult) Bytes() int64 { return r.AttachedBytes + r.ArchiveBytes }

// archiveFileRow - строка реестра файлового архива: что удалять с диска.
type archiveFileRow struct {
	ID       int
	RelDir   string
	FileName string
	Size     int64
}

// purgeApplicationFiles уничтожает файлы заявки за пределами базы.
// apply=false - только подсчёт, ни диск, ни база не меняются.
func purgeApplicationFiles(ctx context.Context, db *gorm.DB, paths FilePaths, id int, apply bool) (FilePurgeResult, error) {
	var res FilePurgeResult

	attached, err := applicationAttachedFiles(ctx, db, id)
	if err != nil {
		return FilePurgeResult{}, err
	}
	res.Attached = len(attached)
	for _, f := range attached {
		res.AttachedBytes += f.Size
	}

	archive, err := applicationArchiveFiles(ctx, db, id)
	if err != nil {
		return FilePurgeResult{}, err
	}
	for _, f := range archive {
		if f.FileName == "" {
			continue
		}
		res.Archive++
		res.ArchiveBytes += f.Size
	}

	if res.Attached > 0 && strings.TrimSpace(paths.UploadPath) == "" {
		res.Skipped = append(res.Skipped, fmt.Sprintf(
			"приложенные к заявке документы (%d) остаются на диске: не задан UPLOAD_PATH", res.Attached))
	}
	if res.Archive > 0 && strings.TrimSpace(paths.ArchivePath) == "" {
		res.Skipped = append(res.Skipped, fmt.Sprintf(
			"файлы архива (%d) остаются на диске: не задан ARCHIVE_PATH", res.Archive))
	}
	if !apply {
		return res, nil
	}

	if res.Attached > 0 && strings.TrimSpace(paths.UploadPath) != "" {
		if err := removeApplicationFiles(paths.UploadPath, attached); err != nil {
			return FilePurgeResult{}, fmt.Errorf("файлы заявки #%d не убраны с диска: %w", id, err)
		}
		if err := db.WithContext(ctx).
			Exec(`DELETE FROM application_files WHERE application_id = ?`, id).Error; err != nil {
			return FilePurgeResult{}, fmt.Errorf("строки файлов заявки #%d: %w", id, err)
		}
	}

	if len(archive) > 0 {
		if err := purgeArchiveFiles(ctx, db, paths.ArchivePath, archive); err != nil {
			return FilePurgeResult{}, err
		}
	}
	return res, nil
}

// applicationAttachedFiles - документы, приложенные к одной заявке. Запрос повторяет
// applicationFileRows, но отбирает по самой заявке, а не по графу организации.
func applicationAttachedFiles(ctx context.Context, db *gorm.DB, id int) ([]appFileRow, error) {
	var rows []appFileRow
	q := `SELECT id, stored_name, file_name, encrypted, file_size AS size
		FROM application_files WHERE application_id = @app ORDER BY id`
	if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("файлы заявки %d: %w", id, err)
	}
	return rows, nil
}

// applicationArchiveFiles - строки реестра файлового архива по заявке. Берутся ВСЕ,
// включая те, у которых файла нет (не настроен бланк, выгрузка выключена): пометку
// «уничтожено» получают все строки заявки, иначе следующий прогон выгрузки завёл бы
// по ним файл заново.
func applicationArchiveFiles(ctx context.Context, db *gorm.DB, id int) ([]archiveFileRow, error) {
	var rows []archiveFileRow
	q := `SELECT id, rel_dir, file_name, size_bytes AS size
		FROM blank_exports WHERE application_id = @app ORDER BY id`
	if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("файлы архива по заявке %d: %w", id, err)
	}
	return rows, nil
}

// purgeArchiveFiles снимает с диска бланки и слепок заявки и гасит строки реестра.
//
// Строки не удаляются, а помечаются: реестр архива и заведён для того, чтобы система
// помнила про файл, которого на диске уже нет (философия модели BlankExport - сироты
// помечаются статусом, а не исчезают молча). Пустой путь в строке при этом обязателен:
// иначе реестр продолжал бы обещать файл по адресу, где его уже нет.
func purgeArchiveFiles(ctx context.Context, db *gorm.DB, root string, rows []archiveFileRow) error {
	if strings.TrimSpace(root) != "" {
		writer, err := services.NewArchiveWriter(root)
		if err != nil {
			return fmt.Errorf("файловый архив недоступен: %w", err)
		}
		for _, r := range rows {
			if r.FileName == "" {
				continue
			}
			levels := splitArchiveRelDir(r.RelDir)
			if err := writer.RemoveFile(levels, r.FileName); err != nil {
				return fmt.Errorf("файл архива %s/%s: %w", r.RelDir, r.FileName, err)
			}
			writer.PruneEmptyDirs(levels)
		}
	}

	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	err := db.WithContext(ctx).Exec(`
		UPDATE blank_exports
		SET status = ?, rel_dir = '', file_name = '', size_bytes = 0, content_hash = '',
		    last_error = '', next_attempt_at = NULL, updated_at = now()
		WHERE id IN ?`, models.BlankExportPurged, ids).Error
	if err != nil {
		return fmt.Errorf("пометка строк архива: %w", err)
	}
	return nil
}

// splitArchiveRelDir разбирает путь реестра на уровни каталогов. Пустые элементы
// выбрасываются: писатель проверяет каждый уровень отдельно и на "" вернёт ошибку
// пути, а не молча положит файл в корень архива.
func splitArchiveRelDir(relDir string) []string {
	parts := strings.Split(relDir, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
