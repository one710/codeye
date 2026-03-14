package version

var (
	Version = "0.1.0"
	Commit  = "dev"
	Date    = "unknown"
)

func String() string {
	return Version
}
