package otelnative

const (
	InputsKey     = "inputs"
	ProcessorsKey = "processors"
	OutputsKey    = "outputs"
)

// Translator defines a way to translate a set of Telegraf plugins into its equivalent
// set of OpenTelemetry receivers, processors, and exporters.
type Translator interface {
	// Name is the identifier for plugins that are output from this converter
	Name() string
	// Introduces supplies a map of plugin names that are expected to be added during the
	// different translation functions
	Introduces() map[string][]string
	// Replaces supplies a map of plugin names that are expected to be removed during the
	// different translation functions
	Replaces() map[string][]string
	// RequiresTranslation takes in inputs, processors, and outputs, and does whatever
	// custom logic is necessary to determine if the translated configuration indicates
	// that something must be translated to OTel natively
	RequiresTranslation(in, proc, out map[string]interface{}) bool
	// Receivers translates the telegraf plugins and returns the expected "receivers" as
	// a mutated version of the in map. This should return a map that only contains the
	// relevant plugins defined in Introduces()
	Receivers(in, proc, out map[string]interface{}) map[string]interface{}
	// Processors translates the telegraf plugins and returns the expected "processors" as
	// a mutated version of the proc map. This should return a map that only contains the
	//	// relevant plugins defined in Introduces()
	Processors(in, proc, out map[string]interface{}) map[string]interface{}
	// Exporters translates the telegraf plugins and returns the expected "exporters" as
	// a mutated version of the out map. This should return a map that only contains the
	//	// relevant plugins defined in Introduces()
	Exporters(in, proc, out map[string]interface{}) map[string]interface{}
}
