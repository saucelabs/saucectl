package flags

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// SnakeCharmer because Cobra and Viper. Get it?
// It's a convenience wrapper around cobra and viper, allowing the user to declare and bind flags at the same time.
type SnakeCharmer struct {
	Fset *pflag.FlagSet
	Fmap map[string]*pflag.Flag
}

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

func (s *SnakeCharmer) addBind(flagName, fieldName string) {
	s.Fmap[fieldName] = s.Fset.Lookup(flagName)
}
