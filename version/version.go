package version

import "fmt"

// See http://semver.org/ for more information on Semantic Versioning
var (
	Major      = 1
	Minor      = 5
	Patch      = 0
	PreRelease = "dev"
)

var Version = fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)

func init() {
	if PreRelease != "" {
		Version += "-" + PreRelease
	}
}
