package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValidatePhaseNumber validates phase number format (e.g., "5" or "5.1")
// Returns the phase string and error if invalid
func ValidatePhaseNumber(arg string) (string, error) {
	// Match integer (5) or decimal (5.1) format
	matched, err := regexp.MatchString(`^\d+(\.\d+)?$`, arg)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid phase number: %s (expected format: 5 or 5.1)", arg)
	}
	return arg, nil
}

// ParsePhaseComponents splits "5.1" into major=5, minor=1 (0 if integer phase)
func ParsePhaseComponents(phase string) (major int, minor int, err error) {
	parts := strings.Split(phase, ".")
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major phase: %s", parts[0])
	}
	if len(parts) == 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid minor phase: %s", parts[1])
		}
	}
	return major, minor, nil
}
