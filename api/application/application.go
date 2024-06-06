package application

const (
	Port = 25796
)


const (
	TypeAny = `Any`
)

const (
	CommandLookup = `Lookup`
)

type ClientOptions struct {
	ApplicationType string `plist:"ApplicationType"`
}

type Request struct {
	*ClientOptions `plist:"ClientOptions"`
	Command        string `plist:"Command"`
}

