package runtime

type Context struct {
	OsParameter               string //we will do all the validation when setting this OsParameter value, skip all the validation afterwards.
	IsOnPrem                  bool
	WantPerInstanceMetrics    bool //CPU per core
	WantEC2TagDimensions      bool
	MetricsCollectionInterval int //sub minute, high resolution, metric collect interval, unit as sec.

	//linux migration
	HasExistingLinuxConfig bool
	ConfigFilePath         string
}
