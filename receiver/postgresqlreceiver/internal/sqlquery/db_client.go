// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sqlquery is a minimal inline copy of the types from
// github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery
// that the PostgreSQL receiver needs. Only the DbClient / DbWrapper /
// TelemetryConfig surface is kept; everything else is omitted.
package sqlquery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// ErrNullValueWarning is returned when a scanned column is NULL.
var ErrNullValueWarning = errors.New("NULL value")

// StringMap is a row represented as column-name → string-value.
type StringMap = map[string]string

// ---- Db abstraction (so we can wrap *sql.DB) ----

type Db interface {
	QueryContext(ctx context.Context, query string, args ...any) (rows, error)
}

type rows interface {
	ColumnTypes() ([]colType, error)
	Next() bool
	Scan(dest ...any) error
}

type colType interface {
	Name() string
}

// DbWrapper wraps a real *sql.DB.
type DbWrapper struct{ Db *sql.DB }

func (d DbWrapper) QueryContext(ctx context.Context, query string, args ...any) (rows, error) {
	r, err := d.Db.QueryContext(ctx, query, args...)
	return rowsWrapper{r}, err
}

type rowsWrapper struct{ rows *sql.Rows }

func (r rowsWrapper) ColumnTypes() ([]colType, error) {
	types, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	out := make([]colType, len(types))
	for i, ct := range types {
		out[i] = colWrapper{ct}
	}
	return out, nil
}
func (r rowsWrapper) Next() bool            { return r.rows.Next() }
func (r rowsWrapper) Scan(dest ...any) error { return r.rows.Scan(dest...) }

type colWrapper struct{ ct *sql.ColumnType }

func (c colWrapper) Name() string { return c.ct.Name() }

// ---- DbClient ----

type DbClient interface {
	QueryRows(ctx context.Context, args ...any) ([]StringMap, error)
}

type TelemetryConfig struct {
	Logs TelemetryLogsConfig `mapstructure:"logs"`
}

type TelemetryLogsConfig struct {
	Query bool `mapstructure:"query"`
}

type dbSQLClient struct {
	db        Db
	sql       string
	logger    *zap.Logger
	telemetry TelemetryConfig
}

func NewDbClient(db Db, sql string, logger *zap.Logger, telemetry TelemetryConfig) DbClient {
	return dbSQLClient{db: db, sql: sql, logger: logger, telemetry: telemetry}
}

func (cl dbSQLClient) QueryRows(ctx context.Context, args ...any) ([]StringMap, error) {
	if cl.telemetry.Logs.Query {
		cl.logger.Debug("Running query", zap.String("query", cl.sql), zap.Any("parameters", args))
	}
	sqlRows, err := cl.db.QueryContext(ctx, cl.sql, args...)
	if err != nil {
		return nil, err
	}
	colTypes, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	scanner := newRowScanner(colTypes)
	var out []StringMap
	var warnings []error
	for sqlRows.Next() {
		if err = scanner.scan(sqlRows); err != nil {
			return nil, err
		}
		sm, scanErr := scanner.toStringMap()
		if scanErr != nil {
			warnings = append(warnings, scanErr)
		}
		out = append(out, sm)
	}
	return out, errors.Join(warnings...)
}

// ---- row scanner ----

type rowScanner struct {
	cols       map[string]func() (string, error)
	scanTarget []any
}

func newRowScanner(colTypes []colType) *rowScanner {
	rs := &rowScanner{cols: map[string]func() (string, error){}}
	for _, sqlType := range colTypes {
		colName := sqlType.Name()
		var v any
		rs.cols[colName] = func() (string, error) {
			if v == nil {
				return "", ErrNullValueWarning
			}
			if t, ok := v.(time.Time); ok {
				return t.Format(time.RFC3339Nano), nil
			}
			format := "%v"
			if reflect.TypeOf(v).Kind() == reflect.Slice {
				format = "%s"
			}
			return fmt.Sprintf(format, v), nil
		}
		rs.scanTarget = append(rs.scanTarget, &v)
	}
	return rs
}

func (rs *rowScanner) scan(sqlRows rows) error { return sqlRows.Scan(rs.scanTarget...) }

func (rs *rowScanner) toStringMap() (StringMap, error) {
	out := StringMap{}
	var errs error
	for k, f := range rs.cols {
		s, err := f()
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("column %q: %w", k, err))
		} else {
			out[k] = s
		}
	}
	return out, errs
}
