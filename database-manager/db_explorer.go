package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultLimit  = 5
	defaultOffset = 0
	maxBodySize   = 1 << 20
)

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return e.Message
}

func newAPIError(status int, format string, args ...interface{}) *apiError {
	return &apiError{Status: status, Message: fmt.Sprintf(format, args...)}
}

type DBExplorer struct {
	db         *sql.DB
	tables     map[string]*table
	tableNames []string
}

func NewDBExplorer(db *sql.DB) (http.Handler, error) {
	tables, names, err := loadSchema(db)
	if err != nil {
		return nil, err
	}
	return &DBExplorer{db: db, tables: tables, tableNames: names}, nil
}

func (e *DBExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result, err := e.route(r)
	if err != nil {
		var apiErr *apiError
		if !errors.As(err, &apiErr) {
			log.Printf("db_explorer: %s %s: %v", r.Method, r.URL.Path, err)
			apiErr = newAPIError(http.StatusInternalServerError, "internal error")
		}
		writeJSON(w, apiErr.Status, map[string]interface{}{"error": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"response": result})
}

func (e *DBExplorer) route(r *http.Request) (interface{}, error) {
	parts := splitPath(r.URL.Path)

	if len(parts) == 0 {
		if r.Method != http.MethodGet {
			return nil, newAPIError(http.StatusMethodNotAllowed, "method not allowed")
		}
		return map[string]interface{}{"tables": e.tableNames}, nil
	}

	if len(parts) > 2 {
		return nil, newAPIError(http.StatusNotFound, "not found")
	}

	t, ok := e.tables[parts[0]]
	if !ok {
		return nil, newAPIError(http.StatusNotFound, "unknown table")
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			return e.list(t, r)
		case http.MethodPut:
			return e.create(t, r)
		}
		return nil, newAPIError(http.StatusMethodNotAllowed, "method not allowed")
	}

	if t.PrimaryKey == nil {
		return nil, newAPIError(http.StatusBadRequest, "table %s has no primary key", t.Name)
	}

	id := parts[1]
	switch r.Method {
	case http.MethodGet:
		return e.get(t, id)
	case http.MethodPost:
		return e.update(t, id, r)
	case http.MethodDelete:
		return e.remove(t, id)
	}
	return nil, newAPIError(http.StatusMethodNotAllowed, "method not allowed")
}

func (e *DBExplorer) list(t *table, r *http.Request) (interface{}, error) {
	query := r.URL.Query()
	limit := parseNonNegative(query.Get("limit"), defaultLimit)
	offset := parseNonNegative(query.Get("offset"), defaultOffset)

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s LIMIT ? OFFSET ?", t.selectList, quoteIdent(t.Name))
	rows, err := e.db.Query(sqlQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []map[string]interface{}{}
	for rows.Next() {
		targets := t.scanTargets()
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		records = append(records, t.toRecord(targets))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]interface{}{"records": records}, nil
}

func (e *DBExplorer) get(t *table, id string) (interface{}, error) {
	sqlQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?",
		t.selectList, quoteIdent(t.Name), quoteIdent(t.PrimaryKey.Name))

	targets := t.scanTargets()
	err := e.db.QueryRow(sqlQuery, id).Scan(targets...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, newAPIError(http.StatusNotFound, "record not found")
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"record": t.toRecord(targets)}, nil
}

func (e *DBExplorer) create(t *table, r *http.Request) (interface{}, error) {
	body, err := decodeBody(r)
	if err != nil {
		return nil, err
	}

	var (
		names        []string
		placeholders []string
		args         []interface{}
		pkValue      interface{}
	)
	for i := range t.Columns {
		c := &t.Columns[i]
		if c.AutoIncrement {
			continue
		}

		raw, present := body[c.Name]
		var value interface{}
		if present {
			v, ok := c.validate(raw)
			if !ok {
				return nil, newAPIError(http.StatusBadRequest, "field %s have invalid type", c.Name)
			}
			value = v
		} else {
			if c.Nullable || c.HasDefault {
				continue
			}
			value = c.zeroValue()
		}

		if c.Primary {
			pkValue = value
		}
		names = append(names, quoteIdent(c.Name))
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}

	sqlQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(t.Name), strings.Join(names, ", "), strings.Join(placeholders, ", "))
	res, err := e.db.Exec(sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	if t.PrimaryKey == nil {
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"inserted": affected}, nil
	}

	if t.PrimaryKey.AutoIncrement {
		lastID, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		pkValue = lastID
	}
	return map[string]interface{}{t.PrimaryKey.Name: pkValue}, nil
}

func (e *DBExplorer) update(t *table, id string, r *http.Request) (interface{}, error) {
	body, err := decodeBody(r)
	if err != nil {
		return nil, err
	}

	var (
		assignments []string
		args        []interface{}
	)
	for i := range t.Columns {
		c := &t.Columns[i]
		raw, present := body[c.Name]
		if !present {
			continue
		}
		if c.Primary {
			return nil, newAPIError(http.StatusBadRequest, "field %s have invalid type", c.Name)
		}
		value, ok := c.validate(raw)
		if !ok {
			return nil, newAPIError(http.StatusBadRequest, "field %s have invalid type", c.Name)
		}
		assignments = append(assignments, quoteIdent(c.Name)+" = ?")
		args = append(args, value)
	}

	if len(assignments) == 0 {
		return map[string]interface{}{"updated": 0}, nil
	}

	args = append(args, id)
	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		quoteIdent(t.Name), strings.Join(assignments, ", "), quoteIdent(t.PrimaryKey.Name))
	res, err := e.db.Exec(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"updated": affected}, nil
}

func (e *DBExplorer) remove(t *table, id string) (interface{}, error) {
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quoteIdent(t.Name), quoteIdent(t.PrimaryKey.Name))
	res, err := e.db.Exec(sqlQuery, id)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": affected}, nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func parseNonNegative(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func decodeBody(r *http.Request) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	err := json.NewDecoder(io.LimitReader(r.Body, maxBodySize)).Decode(&body)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, newAPIError(http.StatusBadRequest, "invalid request body")
	}
	return body, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("db_explorer: marshal response: %v", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		log.Printf("db_explorer: write response: %v", err)
	}
}

type columnKind int

const (
	kindString columnKind = iota
	kindInt
	kindFloat
)

type column struct {
	Name          string
	Kind          columnKind
	Nullable      bool
	HasDefault    bool
	Primary       bool
	AutoIncrement bool
}

type table struct {
	Name       string
	Columns    []column
	PrimaryKey *column
	selectList string
}

func loadSchema(db *sql.DB) (map[string]*table, []string, error) {
	names, err := loadTableNames(db)
	if err != nil {
		return nil, nil, err
	}

	tables := make(map[string]*table, len(names))
	for _, name := range names {
		t, err := loadTable(db, name)
		if err != nil {
			return nil, nil, err
		}
		tables[name] = t
	}
	return tables, names, nil
}

func loadTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("show tables: %w", err)
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}
	return names, nil
}

func loadTable(db *sql.DB, name string) (*table, error) {
	rows, err := db.Query("SHOW FULL COLUMNS FROM " + quoteIdent(name))
	if err != nil {
		return nil, fmt.Errorf("show columns of %s: %w", name, err)
	}
	defer rows.Close()

	fields, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns metadata of %s: %w", name, err)
	}

	t := &table{Name: name}
	for rows.Next() {
		values := make([]sql.NullString, len(fields))
		targets := make([]interface{}, len(fields))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("scan column of %s: %w", name, err)
		}

		info := make(map[string]sql.NullString, len(fields))
		for i, field := range fields {
			info[field] = values[i]
		}

		t.Columns = append(t.Columns, column{
			Name:          info["Field"].String,
			Kind:          kindOf(info["Type"].String),
			Nullable:      strings.EqualFold(info["Null"].String, "YES"),
			HasDefault:    info["Default"].Valid,
			Primary:       info["Key"].String == "PRI",
			AutoIncrement: strings.Contains(strings.ToLower(info["Extra"].String), "auto_increment"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns of %s: %w", name, err)
	}

	quoted := make([]string, len(t.Columns))
	for i := range t.Columns {
		quoted[i] = quoteIdent(t.Columns[i].Name)
		if t.Columns[i].Primary && t.PrimaryKey == nil {
			t.PrimaryKey = &t.Columns[i]
		}
	}
	t.selectList = strings.Join(quoted, ", ")
	return t, nil
}

func kindOf(sqlType string) columnKind {
	base := strings.ToLower(strings.TrimSpace(sqlType))
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "year":
		return kindInt
	case "float", "double", "real", "decimal", "numeric":
		return kindFloat
	default:
		return kindString
	}
}

func (c *column) validate(value interface{}) (interface{}, bool) {
	if value == nil {
		return nil, c.Nullable
	}
	switch c.Kind {
	case kindString:
		s, ok := value.(string)
		return s, ok
	case kindInt:
		f, ok := value.(float64)
		if !ok || f != math.Trunc(f) {
			return nil, false
		}
		return int64(f), true
	case kindFloat:
		f, ok := value.(float64)
		return f, ok
	}
	return nil, false
}

func (c *column) zeroValue() interface{} {
	switch c.Kind {
	case kindInt:
		return int64(0)
	case kindFloat:
		return float64(0)
	default:
		return ""
	}
}

func (c *column) scanTarget() interface{} {
	switch c.Kind {
	case kindInt:
		return &sql.NullInt64{}
	case kindFloat:
		return &sql.NullFloat64{}
	default:
		return &sql.NullString{}
	}
}

func (t *table) scanTargets() []interface{} {
	targets := make([]interface{}, len(t.Columns))
	for i := range t.Columns {
		targets[i] = t.Columns[i].scanTarget()
	}
	return targets
}

func (t *table) toRecord(targets []interface{}) map[string]interface{} {
	record := make(map[string]interface{}, len(t.Columns))
	for i, c := range t.Columns {
		record[c.Name] = unwrapNull(targets[i])
	}
	return record
}

func (t *table) column(name string) *column {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

func unwrapNull(target interface{}) interface{} {
	switch v := target.(type) {
	case *sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
	case *sql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
	case *sql.NullString:
		if v.Valid {
			return v.String
		}
	}
	return nil
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
