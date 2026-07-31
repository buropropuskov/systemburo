package models

import "time"

// ArchiveStats - сводка файлового архива для вкладки «Обзор» (#1615, срез B2):
// занятое место реестра, состав диска и разбивка по месяцам. Сервис держит
// готовый ответ в кэше на 5 минут - собирается не на каждый запрос страницы.
type ArchiveStats struct {
	// UsedBytes - SUM(size_bytes) записанных и живых файлов реестра (status=ok).
	UsedBytes int64 `json:"used_bytes"`
	// FreeBytes - свободно на разделе, которому принадлежит корень архива.
	FreeBytes int64 `json:"free_bytes"`
	FileCount int64 `json:"file_count"`
	// Periods - разбивка архива по месяцам, свежий месяц сверху.
	Periods     []ArchiveStatsPeriod `json:"periods"`
	Disk        ArchiveDiskUsage     `json:"disk"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// ArchiveStatsPeriod - один месяц раскладки архива.
type ArchiveStatsPeriod struct {
	// Month - "2026-07".
	Month     string `json:"month"`
	Bytes     int64  `json:"bytes"`
	FileCount int64  `json:"file_count"`
}

// ArchiveDiskUsage - состав занятого места на разделе, которому принадлежит
// корень архива: под верхнюю полосу интерфейса раздела «Обзор» (#1615, B2).
//
// DatabaseBytes - заявленный размер базы (pg_database_size), а не измеренный
// статистикой ЭТОГО раздела: база живёт в соседнем контейнере на своём томе, и
// процесс архива физически не может проверить, на одном ли они разделе с
// архивом. Число показывается как справочное - "здесь тоже есть данные",
// а не как проверенный вклад именно в этот раздел.
type ArchiveDiskUsage struct {
	TotalBytes    int64 `json:"total_bytes"`
	FreeBytes     int64 `json:"free_bytes"`
	ArchiveBytes  int64 `json:"archive_bytes"`
	UploadsBytes  int64 `json:"uploads_bytes"`
	DatabaseBytes int64 `json:"database_bytes"`
	LogsBytes     int64 `json:"logs_bytes"`
	OtherBytes    int64 `json:"other_bytes"`
	// Partitions - разделы, реально видимые процессу (архив, загрузки, логи -
	// база недоступна статистике, см. DatabaseBytes), дедуплицированные по
	// устройству. Материал для будущего выбора раздела на полосе интерфейса.
	Partitions []ArchiveDiskPartition `json:"partitions"`
}

// ArchiveDiskPartition - один физический раздел с подписями каталогов
// приложения, которые на нём оказались.
type ArchiveDiskPartition struct {
	Labels     []string `json:"labels"`
	TotalBytes int64    `json:"total_bytes"`
	FreeBytes  int64    `json:"free_bytes"`
}
