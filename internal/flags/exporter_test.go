package flags

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

type mockedValue struct {
	content     string
	contentType string
}

func (m *mockedValue) Set(string) error {
	return nil
}

func (m *mockedValue) Type() string {
	return m.contentType
}

func (m *mockedValue) String() string {
	return m.content
}

func Test_redactStringValue(t *testing.T) {
	tests := []struct {
		name string
		flag *pflag.Flag
		want interface{}
	}{
		{
			name: "Basic Test",
			flag: &pflag.Flag{
				Name:    "keyName",
				Changed: true,
				Value: &mockedValue{
					content:     "sensitive",
					contentType: "string",
				},
			},
			want: "***REDACTED***",
		},
		{
			name: "Sensitive Test - Empty",
			flag: &pflag.Flag{
				Name:    "cypress.key",
				Changed: true,
				Value: &mockedValue{
					content:     "",
					contentType: "string",
				},
			},
			want: "***EMPTY***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactStringValue(tt.flag); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("redactValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_redactStringToStringValue(t *testing.T) {
	tests := []struct {
		name string
		flag *pflag.Flag
		want interface{}
	}{
		{
			name: "Sensitive Test - stringToString",
			flag: &pflag.Flag{
				Name:    "cypress.key",
				Changed: true,
				Value: &mockedValue{
					content:     "[KEY1=myValue,KEY2=myValue,KEY3=myValue]",
					contentType: "stringToString",
				},
			},
			want: map[string]string{
				"KEY1": "***REDACTED***",
				"KEY2": "***REDACTED***",
				"KEY3": "***REDACTED***",
			},
		},
		{
			name: "Emtpy stringToString",
			flag: &pflag.Flag{
				Name:    "cypress.key",
				Changed: true,
				Value: &mockedValue{
					content:     "",
					contentType: "stringToString",
				},
			},
			want: map[string]string{},
		},
		{
			name: "Multi = stringToString",
			flag: &pflag.Flag{
				Name:    "cypress.key",
				Changed: true,
				Value: &mockedValue{
					content:     "[KEY1=val1=val2]",
					contentType: "stringToString",
				},
			},
			want: map[string]string{
				"KEY1": "***REDACTED***",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactStringToString(tt.flag); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("redactValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sliceContainsString(t *testing.T) {
	type args struct {
		slice []string
		val   string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Found",
			args: args{
				slice: []string{"val1", "val2"},
				val:   "val1",
			},
			want: true,
		},
		{
			name: "Not Found",
			args: args{
				slice: []string{"val1", "val2"},
				val:   "val3",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceContainsString(tt.args.slice, tt.args.val); got != tt.want {
				t.Errorf("sliceContainsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportCommandLineFlagsMap(t *testing.T) {
	type args struct {
		setBuilder func() *pflag.FlagSet
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Redacted argument",
			args: args{
				setBuilder: func() *pflag.FlagSet {
					pf := pflag.NewFlagSet("XXX", pflag.ContinueOnError)
					pf.String("cypress.key", "", "demo-usage")
					err := pf.Parse([]string{"--cypress.key", "sensitive-value"})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return pf
				},
			},
			want: `{"cypress.key":"***REDACTED***"}`,
		},
		{
			name: "Redacted map argument",
			args: args{
				setBuilder: func() *pflag.FlagSet {
					pf := pflag.NewFlagSet("XXX", pflag.ContinueOnError)
					pf.StringToString("env", map[string]string{}, "demo-usage")
					err := pf.Parse([]string{"--env", "KEY1=val1", "--env", "KEY2=val2"})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return pf
				},
			},
			want: `{"env":{"KEY1":"***REDACTED***","KEY2":"***REDACTED***"}}`,
		},
		{
			name: "Not redacted string argument",
			args: args{
				setBuilder: func() *pflag.FlagSet {
					pf := pflag.NewFlagSet("XXX", pflag.ContinueOnError)
					pf.String("name", "", "demo-usage")
					err := pf.Parse([]string{"--name", "myname"})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return pf
				},
			},
			want: `{"name":"myname"}`,
		},
		{
			name: "Not redacted map argument",
			args: args{
				setBuilder: func() *pflag.FlagSet {
					pf := pflag.NewFlagSet("XXX", pflag.ContinueOnError)
					pf.StringToString("xtra", map[string]string{}, "demo-usage")
					err := pf.Parse([]string{"--xtra", "KEY1=val1", "--xtra", "KEY2=val2"})
					if err != nil {
						t.Errorf("failed to parse test args: %v", err)
					}
					return pf
				},
			},
			want: `{"xtra":{"KEY1":"val1","KEY2":"val2"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CaptureCommandLineFlags(tt.args.setBuilder())

			st, err := json.Marshal(got)
			if err != nil {
				t.Errorf("Marshalling failed: %v", err)
			}
			if string(st) != tt.want {
				t.Errorf("CaptureCommandLineFlags() = %v, want %v", string(st), tt.want)
			}
		})
	}
}
