package model

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/labulaka521/crocodile/common/db"
	"github.com/labulaka521/crocodile/common/log"
	"github.com/labulaka521/crocodile/core/config"
	"go.uber.org/zap"
)

// columnMigrate describe a column that need to be widened
type columnMigrate struct {
	table      string
	column     string
	targetType string
}

// columnMigrates is the schema migration list for old installations.
// utils.GetID generates 19-digit snowflake IDs since 2025-11-26, but
// old installations define ID columns as CHAR(18), so MySQL in strict
// mode rejects the inserts with "Error 1406: Data too long for column".
var columnMigrates = []columnMigrate{
	{TBUser, "id", "char(20)"},
	{TBHost, "id", "char(20)"},
	{TBHostgroup, "id", "char(20)"},
	{TBHostgroup, "createByID", "char(20)"},
	{TBLog, "taskid", "char(20)"},
	{TBLog, "errtaskid", "char(20)"},
	{TBNotify, "notifyuid", "char(20)"},
	{TBOperate, "uid", "char(20)"},
	{TBTask, "id", "char(20)"},
	{TBTask, "createByID", "char(20)"},
	{TBTask, "hostGroupID", "char(20)"},
	{TBTask, "parentTaskIds", "varchar(420)"},
	{TBTask, "childTaskIds", "varchar(420)"},
	{TBTask, "alarmUserIds", "varchar(220)"},
}

var columnTypeRe = regexp.MustCompile(`^(char|varchar)\((\d+)\)$`)

// MigrateDB widens ID columns of old installations in place.
// It is a no-op for new installations and for sqlite
// (sqlite does not enforce the declared CHAR length).
func MigrateDB(ctx context.Context) error {
	if config.CoreConf.Server.DB.Drivename != "mysql" {
		return nil
	}
	conn, err := db.GetConn(ctx)
	if err != nil {
		return fmt.Errorf("db.GetConn failed: %w", err)
	}
	defer conn.Close()
	for _, c := range columnMigrates {
		err = migrateColumn(ctx, conn, c)
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateColumn(ctx context.Context, conn *sql.Conn, c columnMigrate) error {
	var (
		columnType    string
		isNullable    string
		columnDefault sql.NullString
		charset       sql.NullString
		collation     sql.NullString
		comment       sql.NullString
	)
	query := `SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, CHARACTER_SET_NAME, COLLATION_NAME, COLUMN_COMMENT
				FROM information_schema.COLUMNS
				WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?`
	err := conn.QueryRowContext(ctx, query, c.table, c.column).Scan(&columnType,
		&isNullable, &columnDefault, &charset, &collation, &comment)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query column %s.%s failed: %w", c.table, c.column, err)
	}
	if !needWiden(columnType, c.targetType) {
		return nil
	}

	definition := fmt.Sprintf("`%s` %s", c.column, c.targetType)
	if charset.Valid && charset.String != "" {
		definition += " CHARACTER SET " + charset.String
		if collation.Valid && collation.String != "" {
			definition += " COLLATE " + collation.String
		}
	}
	if isNullable == "YES" {
		definition += " NULL"
	} else {
		definition += " NOT NULL"
	}
	if columnDefault.Valid {
		definition += " DEFAULT '" + escapeSQLString(columnDefault.String) + "'"
	} else if isNullable == "YES" {
		definition += " DEFAULT NULL"
	}
	if comment.Valid && comment.String != "" {
		definition += " COMMENT '" + escapeSQLString(comment.String) + "'"
	}

	altersql := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN %s", c.table, definition)
	_, err = conn.ExecContext(ctx, altersql)
	if err != nil {
		return fmt.Errorf("migrate column %s.%s failed: %w", c.table, c.column, err)
	}
	log.Info("migrate column success",
		zap.String("table", c.table),
		zap.String("column", c.column),
		zap.String("from", columnType),
		zap.String("to", c.targetType),
	)
	return nil
}

// needWiden reports whether currentType is the same char type as targetType
// but has a smaller length.
func needWiden(currentType, targetType string) bool {
	current := columnTypeRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(currentType)))
	target := columnTypeRe.FindStringSubmatch(targetType)
	if current == nil || target == nil || current[1] != target[1] {
		return false
	}
	currentLen, _ := strconv.Atoi(current[2])
	targetLen, _ := strconv.Atoi(target[2])
	return currentLen < targetLen
}

func escapeSQLString(s string) string {
	s = strings.Replace(s, `\`, `\\`, -1)
	return strings.Replace(s, `'`, `''`, -1)
}
