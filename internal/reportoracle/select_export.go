package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/godror/godror"
)

// QuerySelect executes one validated SELECT with bound arguments. The caller
// owns the returned rows and must close them.
func (adapter *Adapter) QuerySelect(ctx context.Context, statement string, arguments ...interface{}) (*sql.Rows, error) {
	if adapter == nil || adapter.db == nil || ctx == nil {
		return nil, fmt.Errorf("query oracle export select: adapter is closed")
	}
	if !ValidateSelect(statement) {
		return nil, fmt.Errorf("%w: export query must contain one SELECT statement", ErrInvalidConfiguration)
	}
	arguments = append(arguments, godror.PrefetchCount(adapter.prefetchRows), godror.FetchArraySize(adapter.fetchArraySize), godror.ClobAsString())
	rows, err := adapter.db.QueryContext(ctx, strings.TrimSpace(statement), arguments...)
	if err != nil {
		return nil, fmt.Errorf("query oracle export select: %w", err)
	}
	return rows, nil
}

// ValidateSelect keeps configured SQL at a read-only boundary. Values must be
// supplied separately as named binds by the caller.
func ValidateSelect(statement string) bool {
	_, valid := AnalyzeSelect(statement)
	return valid
}

// SelectAnalysis describes bind variables found in one validated read-only
// SELECT. SQL comments and quoted text do not contribute bind variables.
type SelectAnalysis struct {
	NamedBinds        map[string]struct{}
	HasPositionalBind bool
}

// AnalyzeSelect validates the read-only SELECT boundary and extracts its bind
// variables. Oracle line comments, block comments and quoted text are ignored
// while checking statement separators and locking reads.
func AnalyzeSelect(statement string) (SelectAnalysis, bool) {
	code, ok := executableSQL(statement)
	if !ok {
		return SelectAnalysis{}, false
	}
	fields := strings.Fields(strings.ToLower(code))
	if len(fields) < 2 || fields[0] != "select" || strings.Contains(code, ";") {
		return SelectAnalysis{}, false
	}
	normalized := " " + strings.Join(fields, " ") + " "
	if strings.Contains(normalized, " for update ") {
		return SelectAnalysis{}, false
	}
	analysis := SelectAnalysis{NamedBinds: make(map[string]struct{})}
	for index := 0; index < len(code); index++ {
		if code[index] != ':' || index+1 >= len(code) {
			continue
		}
		next := code[index+1]
		if next >= '0' && next <= '9' {
			analysis.HasPositionalBind = true
			continue
		}
		if !selectBindStart(next) {
			continue
		}
		end := index + 2
		for end < len(code) && selectBindPart(code[end]) {
			end++
		}
		analysis.NamedBinds[strings.ToLower(code[index+1:end])] = struct{}{}
		index = end - 1
	}
	return analysis, true
}

func executableSQL(statement string) (string, bool) {
	code := []byte(statement)
	const (
		plainSQL = iota
		singleQuotedSQL
		doubleQuotedSQL
		lineCommentSQL
		blockCommentSQL
	)
	state := plainSQL
	for index := 0; index < len(code); index++ {
		character := code[index]
		switch state {
		case plainSQL:
			switch {
			case character == '\'':
				code[index] = ' '
				state = singleQuotedSQL
			case character == '"':
				code[index] = ' '
				state = doubleQuotedSQL
			case character == '-' && index+1 < len(code) && code[index+1] == '-':
				code[index], code[index+1] = ' ', ' '
				index++
				state = lineCommentSQL
			case character == '/' && index+1 < len(code) && code[index+1] == '*':
				code[index], code[index+1] = ' ', ' '
				index++
				state = blockCommentSQL
			}
		case singleQuotedSQL:
			code[index] = ' '
			if character == '\'' {
				if index+1 < len(code) && code[index+1] == '\'' {
					code[index+1] = ' '
					index++
				} else {
					state = plainSQL
				}
			}
		case doubleQuotedSQL:
			code[index] = ' '
			if character == '"' {
				if index+1 < len(code) && code[index+1] == '"' {
					code[index+1] = ' '
					index++
				} else {
					state = plainSQL
				}
			}
		case lineCommentSQL:
			if character == '\n' || character == '\r' {
				state = plainSQL
			} else {
				code[index] = ' '
			}
		case blockCommentSQL:
			code[index] = ' '
			if character == '*' && index+1 < len(code) && code[index+1] == '/' {
				code[index+1] = ' '
				index++
				state = plainSQL
			}
		}
	}
	return string(code), state == plainSQL || state == lineCommentSQL
}

func selectBindStart(character byte) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func selectBindPart(character byte) bool {
	return selectBindStart(character) || character >= '0' && character <= '9' || character == '_'
}
