package services

import "testing"

func TestUserDisplayPolicyLabel(t *testing.T) {
	last, first, middle := "Фамилия", "Имя", "Отчество"
	for _, tc := range []struct {
		name   string
		policy userDisplayPolicy
		want   string
	}{
		{"visible name", userDisplayPolicy{}, "Фамилия Имя Отчество"},
		{"alias", userDisplayPolicy{names: map[int]string{7: "@Alias"}}, "@Alias"},
		{"restriction overrides alias", userDisplayPolicy{consent: map[int]string{7: hiddenUserLabel}, names: map[int]string{7: "@Alias"}}, hiddenUserLabel},
		{"other account restriction", userDisplayPolicy{consent: map[int]string{8: hiddenUserLabel}}, "Фамилия Имя Отчество"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.label(7, &last, &first, &middle); got != tc.want {
				t.Fatalf("label = %q, want %q", got, tc.want)
			}
		})
	}
	if got := (userDisplayPolicy{}).label(7, nil, nil, nil); got != unnamedUserLabel {
		t.Fatalf("empty identity = %q", got)
	}
	if last != "Фамилия" || first != "Имя" || middle != "Отчество" {
		t.Fatal("display changed source identity")
	}
}
