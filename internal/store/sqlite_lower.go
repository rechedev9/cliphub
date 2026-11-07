package store

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"modernc.org/sqlite"
)

// sqlLower names the SQL function that lowercases text as Go does. SQLite's
// lower() folds only ASCII, so "RÁPIDA" would miss a play tagged "rápida".
const sqlLower = "cliphub_lower"

func init() {
	if err := sqlite.RegisterDeterministicScalarFunction(sqlLower, 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch v := args[0].(type) {
		case string:
			return strings.ToLower(v), nil
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("%s takes text, got %T", sqlLower, v)
		}
	}); err != nil {
		panic(err)
	}
}

// tagFilterSQL keeps the rows whose JSON tags column holds the bound tag, case-insensitively.
func tagFilterSQL(column string) string {
	return ` AND EXISTS (SELECT 1 FROM json_each(` + column + `) WHERE ` + sqlLower + `(json_each.value) = ` + sqlLower + `(?))`
}
