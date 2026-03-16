package version

var (
	Version = "0.2.0"
	Commit  = "dev"
	Date    = "unknown"
)

func String() string {
	return Version
}
