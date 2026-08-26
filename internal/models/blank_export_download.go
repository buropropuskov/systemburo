package models

// ArchiveDownloadRequest - выбор периода для потокового ZIP файлового архива
// (#1615, срез B3). Используется и для оценки объёма (POST /estimate), и для
// выдачи билета на скачивание (POST /download-ticket) - оба смотрят на один и
// тот же период, иначе оценка и фактический билет могли бы разойтись.
type ArchiveDownloadRequest struct {
	// DateFrom/DateTo - границы периода включительно, формат YYYY-MM-DD. Сверяются
	// с bucket_date реестра - местной датой подачи заявки, а не с текущим временем.
	DateFrom string `json:"date_from" validate:"required"`
	DateTo   string `json:"date_to" validate:"required"`
}

// ArchiveDownloadEstimate - оценка объёма выгрузки за период до фактического
// скачивания: конструктор на фронте предупреждает о большом архиве, не запуская его.
type ArchiveDownloadEstimate struct {
	FileCount int64 `json:"file_count"`
	Bytes     int64 `json:"bytes"`
	// ExceedsLimit - оценённый объём больше archive.zip_max_bytes. POST
	// /download-ticket на этом же периоде откажет 413 - оценка заранее показывает,
	// почему кнопка скачивания не сработает.
	ExceedsLimit bool `json:"exceeds_limit"`
}

// ArchiveDownloadTicketResponse - одноразовый билет на потоковый ZIP за период.
// Билет привязан к периоду и пользователю на выдаче (см. ArchiveDownloadService) -
// сам GET-запрос на скачивание границы периода повторно не принимает, поэтому
// подменить их после проверки прав и оценки объёма нельзя.
type ArchiveDownloadTicketResponse struct {
	Ticket string `json:"ticket"`
}
