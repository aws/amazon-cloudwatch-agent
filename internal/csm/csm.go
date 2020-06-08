package csm

// These constants represent default values and agent configuration
// keys.
const (
	DefaultPort            = int(31000)
	DefaultMemoryLimitInMb = int(20)
	DefaultLogLevel        = int(0)

	PortKey             = "port"
	MemoryLimitInMbKey  = "memory_limit_in_mb"
	LogLevelKey         = "log_level"
	EndpointOverrideKey = "endpoint_override"
	ServiceAddressesKey = "service_addresses"
	DataFormatKey       = "data_format"

	JSONSectionKey = "csm"
)
