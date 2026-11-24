package config

import (
	"net/url"
	"testing"
)

// Parolda @, :, / kabi belgilar bo'lsa ham DSN to'g'ri yig'ilishi kerak.
func TestPostgresURLEscapesPassword(t *testing.T) {
	cfg := Config{PostgresHost: "db", PostgresPort: 5432, PostgresUser: "app", PostgresPassword: "p@ss:w/rd", PostgresDatabase: "om"}

	u, err := url.Parse(cfg.PostgresURL())
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	if u.Host != "db:5432" || u.User.Username() != "app" || pass != "p@ss:w/rd" || u.Path != "/om" {
		t.Fatalf("DSN noto'g'ri yig'ildi: host=%s user=%s path=%s", u.Host, u.User.Username(), u.Path)
	}
}
