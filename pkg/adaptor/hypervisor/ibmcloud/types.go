package ibmcloud

import (
	"fmt"
	"reflect"
	"sort"
)

const (
	keyMask = "********"
)

type Config struct {
	ApiKey                   string
	IamServiceURL            string
	VpcServiceURL            string
	ResourceGroupID          string
	ProfileName              string
	ZoneName                 string
	ImageID                  string
	PrimarySubnetID          string
	PrimarySecurityGroupID   string
	SecondarySubnetID        string
	SecondarySecurityGroupID string
	KeyID                    string
	VpcID                    string
}

var configFields []string

// Use reflect to manage list of fields in the Config struct
func init() {
	typeOf := reflect.TypeOf(Config{})
	configFields = make([]string, typeOf.NumField())
	for i := 0; i < typeOf.NumField(); i++ {
		configFields[i] = typeOf.Field(i).Name
	}
	sort.Strings(configFields)
}

// Add Stringer interface to return keyMask when outputting ApiKey field
func (config *Config) String() string {
	toMask := config
	toMask.ApiKey = keyMask
	return fmt.Sprint(toMask)
}

// Add custom Formatter to handle different output cases
// Also return keyMask if the field name is ApiKey
func (config Config) Format(state fmt.State, verb rune) {
	switch verb {
	case 's', 'q':
		val := config.String()
		if verb == 'q' {
			val = fmt.Sprintf("%q", val)
		}
		fmt.Fprint(state, val)
	case 'v':
		if state.Flag('#') {
			fmt.Fprintf(state, "%T", config)
		}
		fmt.Fprint(state, "{")
		val := reflect.ValueOf(config)
		for i, name := range configFields {
			if state.Flag('#') || state.Flag('+') {
				fmt.Fprintf(state, "%s:", name)
			}
			fld := val.FieldByName(name)
			// If the field is ApiKey, apply the keyMask
			if name == "ApiKey" {
				fmt.Fprint(state, keyMask)
			} else {
				fmt.Fprint(state, fld)
			}
			if i < len(configFields)-1 {
				fmt.Fprint(state, " ")
			}
		}
		fmt.Fprint(state, "}")
	}
}
