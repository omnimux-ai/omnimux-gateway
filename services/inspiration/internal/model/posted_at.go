package model

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const postedAtLayout = "2006-01-02 15:04:05.000"

func init() { schema.RegisterSerializer("posted_at_utc", postedAtSerializer{}) }

// PostedAtValue encodes UTC wall time without triggering a driver's DSN loc conversion.
// The same encoding is required for map updates and query arguments.
func PostedAtValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(postedAtLayout)
}

type postedAtSerializer struct{}

func (postedAtSerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, value any) (any, error) {
	return PostedAtValue(value.(*time.Time)), nil
}

func (postedAtSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, value any) error {
	var parsed *time.Time
	if value != nil {
		var instant time.Time
		var err error
		switch v := value.(type) {
		case time.Time:
			instant = v.UTC()
		case string:
			instant, err = time.Parse("2006-01-02 15:04:05", v)
		case []byte:
			instant, err = time.Parse("2006-01-02 15:04:05", string(v))
		default:
			return fmt.Errorf("invalid posted_at database type %T", value)
		}
		if err != nil {
			return err
		}
		parsed = &instant
	}
	field.ReflectValueOf(ctx, dst).Set(reflect.ValueOf(parsed))
	return nil
}

// ReadPostedAtUTC bypasses driver timezone parsing for this field alone. Parsing UTC wall
// clocks in a DST location can normalize nonexistent local times before Scan.
func ReadPostedAtUTC(tx *gorm.DB) {
	if tx.Statement.Schema == nil || tx.Statement.Schema.Table != "inspirations" || len(tx.Statement.Selects) != 0 || len(tx.Statement.Omits) != 0 {
		return
	}
	if _, exists := tx.Statement.Clauses["SELECT"]; exists {
		return
	}
	columns := make([]clause.Expression, 0, len(tx.Statement.Schema.DBNames))
	for _, name := range tx.Statement.Schema.DBNames {
		column := clause.Column{Table: clause.CurrentTable, Name: name}
		if name == "posted_at" {
			expression := "DATE_FORMAT(?, '%Y-%m-%d %H:%i:%s.%f') AS ?"
			if tx.Dialector.Name() == "postgres" {
				expression = "TO_CHAR(?, 'YYYY-MM-DD HH24:MI:SS.US') AS ?"
			}
			columns = append(columns, clause.Expr{SQL: expression, Vars: []any{column, clause.Column{Name: name}}})
		} else {
			columns = append(columns, clause.Expr{SQL: "?", Vars: []any{column}})
		}
	}
	tx.Statement.AddClause(clause.Select{Expression: clause.CommaExpression{Exprs: columns}})
}
