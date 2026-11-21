package model

import "testing"

func Test_needWiden(t *testing.T) {
	cases := []struct {
		currentType string
		targetType  string
		want        bool
	}{
		{"char(18)", "char(20)", true},
		{"CHAR(18)", "char(20)", true},
		{"char(20)", "char(20)", false},
		{"char(25)", "char(20)", false},
		{"varchar(380)", "varchar(420)", true},
		{"varchar(420)", "varchar(420)", false},
		{"text", "varchar(420)", false},
		{"char(18)", "varchar(420)", false},
	}
	for _, c := range cases {
		got := needWiden(c.currentType, c.targetType)
		if got != c.want {
			t.Errorf("needWiden(%q, %q) = %v, want %v", c.currentType, c.targetType, got, c.want)
		}
	}
}
