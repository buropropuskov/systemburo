package services

// Кворум круга согласования - один на два круга (#1685).
//
// Кругов в системе два: основной круг заявки (application_responsible_users -> запись в
// applications.confirmation) и круг раунда дополнения (application_supplement_approvals ->
// запись в application_supplements.status). Правило подсчёта у них обязано совпадать: два
// движка кворума расходятся при первой же правке одного из них, и заявка с раундом начинают
// по-разному отвечать на один и тот же расклад голосов. Поэтому подсчёт живёт здесь чистой
// функцией, а каждый круг только переводит её исход в свой словарь.

// Словарь голосов, общий для обоих кругов: колонка approval_status в обеих таблицах.
const (
	voteStatusPending  = "pending"
	voteStatusApproved = "approved"
	voteStatusRejected = "rejected"
)

// approvalVote - голос, обрезанный до того, что нужно кворуму: обязателен ли голосующий и
// как он проголосовал. NULL в статусе равнозначен pending - строку могли завести до того,
// как у колонки появился default.
type approvalVote struct {
	Required bool
	Status   *string
}

// tallyApprovals - исход круга согласования по его голосам, в словаре voteStatus*.
//
// Правило: любой обязательный отказ хоронит круг; иначе, если обязательные есть - круг
// согласован только когда согласны все они; если обязательных нет - решают необязательные
// (хоть один «за» и ни одного «против»). Во всех прочих раскладах круг ещё идёт.
//
// Пустой список даёт pending, но у него нет собственного смысла: круг без голосующих
// вердикта не производит, и вызывающий такой круг не трогает.
func tallyApprovals(votes []approvalVote) string {
	var required, optional []approvalVote
	for _, v := range votes {
		if v.Required {
			required = append(required, v)
		} else {
			optional = append(optional, v)
		}
	}

	for _, v := range required {
		if voted(v, voteStatusRejected) {
			return voteStatusRejected
		}
	}

	if len(required) > 0 {
		for _, v := range required {
			if !voted(v, voteStatusApproved) {
				return voteStatusPending
			}
		}
		return voteStatusApproved
	}

	hasApproved, hasRejected := false, false
	for _, v := range optional {
		if voted(v, voteStatusApproved) {
			hasApproved = true
		}
		if voted(v, voteStatusRejected) {
			hasRejected = true
		}
	}
	switch {
	case hasRejected:
		// Отказ необязательного весомее согласия соседа: один «против» хоронит круг.
		return voteStatusRejected
	case hasApproved:
		return voteStatusApproved
	}
	return voteStatusPending
}

// voted - проголосовал ли участник ровно так.
func voted(v approvalVote, status string) bool {
	return v.Status != nil && *v.Status == status
}
