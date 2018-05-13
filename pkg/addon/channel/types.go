package channel

type ChannelSpec struct {
	Versions []*ChannelVersion `json:"versions,omitempty"`
}

type ChannelVersion struct {
	// Version is a semver version
	Version string `json:"version,omitempty"`
	// Path is the relative path to the addon
	Path string `json:"path,omitempty"`
}
