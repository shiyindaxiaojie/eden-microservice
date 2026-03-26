package settings

import "strings"

var validLogLevels = map[string]struct{}{
	"TRACE": {},
	"DEBUG": {},
	"INFO":  {},
	"WARN":  {},
	"ERROR": {},
	"FATAL": {},
	"OFF":   {},
}

func NormalizeLogLevel(level string) string {
	upper := strings.ToUpper(strings.TrimSpace(level))
	if _, ok := validLogLevels[upper]; ok {
		return upper
	}
	return "INFO"
}

func IsValidLogLevel(level string) bool {
	upper := strings.ToUpper(strings.TrimSpace(level))
	_, ok := validLogLevels[upper]
	return ok
}
