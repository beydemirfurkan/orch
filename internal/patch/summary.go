package patch

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

func Summarize(p *models.Patch) string {
	if p == nil || strings.TrimSpace(p.RawDiff) == "" {
		return "no patch available"
	}

	added := 0
	removed := 0
	for _, line := range strings.Split(p.RawDiff, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			added++
		}
		if strings.HasPrefix(line, "-") {
			removed++
		}
	}

	return fmt.Sprintf("files=%d added=%d removed=%d", len(p.Files), added, removed)
}
