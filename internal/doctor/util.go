package doctor

import "strings"

func joinProblems(problems []string) string {
	return strings.Join(problems, "; ")
}
