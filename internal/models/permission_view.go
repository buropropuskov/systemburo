package models

// MyPermissionItem -- одно эффективное право текущего пользователя.
type MyPermissionItem struct {
	Key    string `json:"key"`
	Value  string `json:"value"`  // allow (для normal-режима)
	Source string `json:"source"` // role|group|override
}

// MyPermissionsResponse -- ответ GET /permissions/my для фронтового стора прав.
// mode определяет трактовку: super -> всё (тумблеры readonly), admin -> всё кроме
// denied и super-only, normal -> только permissions, banned -> ничего.
type MyPermissionsResponse struct {
	Mode        string             `json:"mode"`
	Permissions []MyPermissionItem `json:"permissions"`
	Denied      []string           `json:"denied"`
	Banned      bool               `json:"banned"`
	BanReason   string             `json:"ban_reason,omitempty"`
}
