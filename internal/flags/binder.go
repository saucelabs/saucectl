package flags

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"

	"github.com/saucelabs/saucectl/internal/viper"
)

// SnakeCharmer because Cobra and Viper. Get it?
// It's a convenience wrapper around cobra and viper, allowing the user to declare and bind flags at the same time.
//
// Example:
//
//		sc := flags.SnakeCharmer{Fmap: map[string]*pflag.Flag{}}
//		sc.Fset = cmd.Flags()
//		sc.String("name", "suite.name", "", "Set the name of the job as it will appear on Sauce Labs")
//	 sc.BindAll()
type SnakeCharmer struct {
	Fset *pflag.FlagSet
	// Fmap maps field names (key) to flags (value).
	Fmap map[string]*pflag.Flag
}

// BindAll binds all previously added flags to their respective fields.
func (s *SnakeCharmer) BindAll() {
	for fieldName, flag := range s.Fmap {
		if err := viper.BindPFlag(fieldName, flag); err != nil {
			log.Fatal().Msgf("Failed to bind flags and config fields: %v", err)
		}
	}
}

// Bool defines a bool flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) Bool(flagName, fieldName string, value bool, usage string) {
	s.Fset.Bool(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// BoolP is like Bool(), but accepts a shorthand letter.
func (s *SnakeCharmer) BoolP(flagName, shorthand, fieldName string, value bool, usage string) {
	s.Fset.BoolP(flagName, shorthand, value, usage)
	s.addBind(flagName, fieldName)
}

// Duration defines a duration flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) Duration(flagName string, fieldName string, value time.Duration, usage string) {
	s.Fset.Duration(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// Float64 defines a float64 flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) Float64(flagName, fieldName string, value float64, usage string) {
	s.Fset.Float64(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// Int defines an int flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) Int(flagName, fieldName string, value int, usage string) {
	s.Fset.Int(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// String defines a string flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) String(flagName, fieldName, value, usage string) {
	s.Fset.String(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// StringP is like String(), but accepts a shorthand letter.
func (s *SnakeCharmer) StringP(flagName, shorthand, fieldName, value, usage string) {
	s.Fset.StringP(flagName, shorthand, value, usage)
	s.addBind(flagName, fieldName)
}

// StringSlice defines a []string flag with specified flagName, default value, usage string and then binds it to fieldName.
func (s *SnakeCharmer) StringSlice(flagName, fieldName string, value []string, usage string) {
	s.Fset.StringSlice(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// StringToString defines a map[string]string flag with specified flagName, default value, usage string and then binds
// it to fieldName.
func (s *SnakeCharmer) StringToString(flagName, fieldName string, value map[string]string, usage string) {
	s.Fset.StringToString(flagName, value, usage)
	s.addBind(flagName, fieldName)
}

// StringToStringP is like StringToString(), but accepts a shorthand letter.
func (s *SnakeCharmer) StringToStringP(flagName, shorthand, fieldName string, value map[string]string, usage string) {
	s.Fset.StringToStringP(flagName, shorthand, value, usage)
	s.addBind(flagName, fieldName)
}

func (s *SnakeCharmer) addBind(flagName, fieldName string) {
	s.Fmap[fieldName] = s.Fset.Lookup(flagName)
}
