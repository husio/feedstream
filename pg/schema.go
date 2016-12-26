package pg

import (
	"fmt"
	"os"
	"strings"
)

var SchemaSep = "\n---\n"

func MustLoadSchema(e Execer, sqlBlob string) {
	err := LoadSchema(e, sqlBlob)
	if err == nil {
		return
	}

	if err, ok := err.(*SchemaError); ok {
		fmt.Fprintf(os.Stderr, "cannot load schema: %s\n%s\n", err.Err, err.Query)
	} else {
		fmt.Fprintf(os.Stderr, "cannot load schema: %s\n", err)
	}
	os.Exit(1)
}

func LoadSchema(e Execer, sqlBlob string) error {
	queries := strings.Split(sqlBlob, SchemaSep)
	for _, query := range queries {
		if _, err := e.Exec(query); err != nil {
			return &SchemaError{Query: query, Err: err}
		}
	}
	return nil
}

type SchemaError struct {
	Query string
	Err   error
}

func (e *SchemaError) Error() string {
	return fmt.Sprintf("schema error: %s", e.Err.Error())
}
