package postgres

import (
	"strings"
	"testing"
)

// A connection string reaches a log line whenever the database is not up yet, and
// in a compose stack "not up yet" is the normal first few seconds. Redaction is
// therefore on the path an operator walks first, and it is the one thing in this
// package that can be checked without a database.

func TestThePasswordNeverReachesTheLog(t *testing.T) {
	cases := []struct {
		name        string
		connection  string
		secret      string
		keptVisible string
	}{
		{
			name:        "the usual URL",
			connection:  "postgres://energy:s3cr3t@localhost:5432/energy?sslmode=disable",
			secret:      "s3cr3t",
			keptVisible: "localhost:5432/energy",
		},
		{
			// The shape that broke the first version: the first colon belongs to the
			// scheme, and treating that as the password separator printed the rest of
			// the URL, password included.
			name:        "a password that looks like the next field",
			connection:  "postgres://user:pass@localhost/db",
			secret:      "pass@localhost",
			keptVisible: "localhost/db",
		},
		{
			name:        "a user with no password",
			connection:  "postgres://energy@localhost:5432/energy",
			keptVisible: "energy@localhost:5432/energy",
		},
		{
			name:        "a DSN instead of a URL",
			connection:  "host=localhost port=5432 user=energy password=s3cr3t dbname=energy",
			secret:      "s3cr3t",
			keptVisible: "host=localhost",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := redact(testCase.connection)
			if testCase.secret != "" && strings.Contains(got, testCase.secret) {
				t.Errorf("redacting %s left the secret in %s", testCase.connection, got)
			}
			if testCase.keptVisible != "" && !strings.Contains(got, testCase.keptVisible) {
				t.Errorf("redacting %s dropped the part that identifies the database: %s", testCase.connection, got)
			}
		})
	}
}
