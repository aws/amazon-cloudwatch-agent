package otelnative

const (
	InputsKey     = "inputs"
	ProcessorsKey = "processors"
	OutputsKey    = "outputs"
)

// NativeOTel defines a way to translate a set of Telegraf plugins into its equivalent
// set of OpenTelemetry receivers, processors, and exporters.
type NativeOTel interface {
	// Name is the identifier for plugins that are output from this converter
	Name() string
	// Introduces supplies a map of plugin names that are expected to be added during the
	// different translation functions
	Introduces() map[string][]string
	// Replaces supplies a map of plugin names that are expected to be removed during the
	// different translation functions
	Replaces() map[string][]string
	// Receivers translates the telegraf plugins and returns the expected "receivers" as
	// a mutated version of the in map. This should only modify the plugins listed out in
	// the Introduces() and Replaces() functions
	Receivers(in, proc, out map[string]interface{}) map[string]interface{}
	// Processors translates the telegraf plugins and returns the expected "processors" as
	// a mutated version of the proc map. This should only modify the plugins listed out in
	// the Introduces() and Replaces() functions
	Processors(in, proc, out map[string]interface{}) map[string]interface{}
	// Exporters translates the telegraf plugins and returns the expected "exporters" as
	// a mutated version of the out map. This should only modify the plugins listed out in
	// the Introduces() and Replaces() functions
	Exporters(in, proc, out map[string]interface{}) map[string]interface{}
}
