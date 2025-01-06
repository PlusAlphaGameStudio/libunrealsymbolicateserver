package platform

import (
	"os"
	"strings"
)

func replaceHome(p string) string {
	if strings.HasPrefix(p, "~" + string(os.PathSeparator)) {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return strings.ReplaceAll(p, "~" + string(os.PathSeparator), homeDir + string(os.PathSeparator))
		}
	}

	return p
}

