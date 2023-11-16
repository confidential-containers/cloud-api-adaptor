// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package provider

import "testing"

func TestEmptyKeyValueFlag_Set(t *testing.T) {
	// Empty KeyValueFlag will result in error
	var flag KeyValueFlag
	err := flag.Set("")
	if err == nil {
		t.Errorf("Expect error, got nil")
	}
}

func TestKeyValueFlag_Set(t *testing.T) {
	tests := []struct {
		// Add test name
		name          string
		input         string
		expectedValue KeyValueFlag
		expectedError bool
	}{
		{
			name:  "valid key value pair",
			input: "key1=value1,key2=value2,key3=value3",
			expectedValue: KeyValueFlag{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			expectedError: false,
		},
		{
			name:          "invalid key value pair 1",
			input:         "invalid",
			expectedValue: nil,
			expectedError: true,
		},
		{
			name:          "invalid key value pair with 2",
			input:         "key:value",
			expectedValue: nil,
			expectedError: true,
		},
		// Add more test cases "key1=value1, key2=value2" and "key1=value1,key2=value2, key3=value3"
		// to cover all the cases
		{
			name:  "valid key value pair with spaces 1",
			input: "key1=value1, key2=value2",
			expectedValue: KeyValueFlag{
				"key1": "value1",
				"key2": "value2",
			},
			expectedError: false,
		},
		{
			name:  "valid key value pair with spaces 2",
			input: "key1=value1,key2=value2, key3=value3",
			expectedValue: KeyValueFlag{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			expectedError: false,
		},
		// Add test case for "key1=value1 key2=value2"
		{
			name:  "valid key value pair separated with spaces",
			input: "key1=value1 key2=value2",
			expectedValue: KeyValueFlag{
				"key1": "value1 key2=value2",
			},
			expectedError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a new KeyValueFlag
			k := make(KeyValueFlag)

			// Set the flag value
			err := k.Set(test.input)

			// Check the result
			if (err != nil) != test.expectedError {
				t.Errorf("Unexpected error, got: %v, expected error: %v, received value: %v", err, test.expectedError, k)
			}

			if test.expectedError {
				return
			}

			// Compare the KeyValueFlag value
			if !isEqual(k, test.expectedValue) {
				t.Errorf("Unexpected KeyValueFlag value, got: %v, expected: %v", k, test.expectedValue)
			}
		})
	}
}

func isEqual(a, b KeyValueFlag) bool {
	if len(a) != len(b) {
		return false
	}

	for key, value := range a {
		if bValue, ok := b[key]; !ok || value != bValue {
			return false
		}
	}

	return true
}
