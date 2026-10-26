// Package layout defines the stable writable directory contract.
package layout

const Root = "/storage/heya"
const (
	Apps      = Root + "/apps"
	Data      = Root + "/data"
	Cache     = Root + "/cache"
	Logs      = Root + "/logs"
	Themes    = Root + "/themes"
	Downloads = Root + "/downloads"
)

// Directories lists directories created for a first boot.
var Directories = []string{Apps, Data, Cache, Logs, Themes, Downloads}
