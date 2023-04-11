// Package viper provides convenience functions over the official spf13/viper library.
// In particular, it satisfies the need of providing a custom pre-configured global viper instance.
package viper

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Default is the default, pre-configured instance of viper.
var Default = viper.NewWithOptions(viper.KeyDelimiter("::"))

// AddConfigPath adds a path for Viper to search for the config file in.
// Can be called multiple times to define multiple search paths.
func AddConfigPath(in string) { Default.AddConfigPath(in) }

// BindPFlag binds a specific key to a pflag (as used by cobra).
// Example (where serverCmd is a Cobra instance):
//
//	serverCmd.Flags().Int("port", 1138, "Port to run Application server on")
//	Viper.BindPFlag("port", serverCmd.Flags().Lookup("port"))
func BindPFlag(key string, flag *pflag.Flag) error { return Default.BindPFlag(key, flag) }

// ReadInConfig will discover and load the configuration file from disk
// and key/value stores, searching in one of the defined paths.
func ReadInConfig() error { return Default.ReadInConfig() }

// Set sets the value for the key in the override register.
// Set is case-insensitive for a key.
// Will be used instead of values obtained via
// flags, config file, ENV, default, or key/value store.
func Set(key string, value interface{}) { Default.Set(key, value) }

// SetConfigName sets name for the config file.
// Does not include extension.
func SetConfigName(in string) { Default.SetConfigName(in) }

// Unmarshal unmarshals the config into a Struct. Make sure that the tags
// on the fields of the structure are properly set.
func Unmarshal(rawVal interface{}, opts ...viper.DecoderConfigOption) error {
	return Default.Unmarshal(rawVal, opts...)
}
