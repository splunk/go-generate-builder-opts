/*
 * Copyright 2020 Splunk Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"io/ioutil"
	"testing"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		input           *flagOpts
		expectedOutFile string
		errExpected     bool
	}{
		// Regular case, exported fn type, exported fields only
		{
			input: &flagOpts{
				definitionFile:              "./testdata/1.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.1.go.out",
		},

		// Include setters for unexported fields
		{
			input: &flagOpts{
				definitionFile:              "./testdata/1.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.2.go.out",
		},

		// Unexported function type
		{
			input: &flagOpts{
				definitionFile:              "./testdata/1.go.in",
				structTypeName:              "A",
				exportFnType:                false,
				generateForUnexportedFields: true,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.3.go.out",
		},

		// Error result - type missing
		{
			input: &flagOpts{
				definitionFile:              "./testdata/1.go.in",
				structTypeName:              "B",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           true,
			},
			errExpected: true,
		},

		// Error - type exists, but is string-based, not struct
		{
			input: &flagOpts{
				definitionFile:              "./testdata/2.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           true,
			},
			errExpected: true,
		},

		// Error - unparsable file
		{
			input: &flagOpts{
				definitionFile:              "./testdata/3.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           true,
			},
			errExpected: true,
		},

		// Error - embedded field
		{
			input: &flagOpts{
				definitionFile:              "./testdata/4.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           false,
			},
			errExpected: true,
		},

		// Unexported struct w/ unexported field
		{
			input: &flagOpts{
				definitionFile:              "./testdata/5.go.in",
				structTypeName:              "a",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.4.go.out",
		},

		// Field w/ pointer type
		{
			input: &flagOpts{
				definitionFile:              "./testdata/6.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.5.go.out",
		},

		// Going wild here
		{
			input: &flagOpts{
				definitionFile:              "./testdata/7.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           false,
			},
			expectedOutFile: "./testdata/expected.6.go.out",
		},

		// But can we import tho?? No :(
		{
			input: &flagOpts{
				definitionFile:              "./testdata/8.go.in",
				structTypeName:              "a",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           false,
			},
			errExpected: true,
		},

		// Pointer import
		{
			input: &flagOpts{
				definitionFile:              "./testdata/9.go.in",
				structTypeName:              "a",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           false,
			},
			errExpected: true,
		},

		// Nested import
		{
			input: &flagOpts{
				definitionFile:              "./testdata/10.go.in",
				structTypeName:              "a",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           false,
			},
			errExpected: true,
		},

		// Imported and embedded but ignored
		{
			input: &flagOpts{
				definitionFile:              "./testdata/11.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           true,
			},
			expectedOutFile: "./testdata/expected.7.go.out",
		},

		// Imported and embedded and no fields left over
		{
			input: &flagOpts{
				definitionFile:              "./testdata/12.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: true,
				ignoreUnsupported:           true,
			},
			errExpected: true,
		},

		// With two struct fields skipped
		{
			input: &flagOpts{
				definitionFile:              "./testdata/13.go.in",
				structTypeName:              "A",
				exportFnType:                true,
				generateForUnexportedFields: false,
				ignoreUnsupported:           false,
				skipStructFields: flagStringSet{
					"C": struct{}{},
					"D": struct{}{},
				},
			},
			expectedOutFile: "./testdata/expected.8.go.out",
		},
	}

	for _, testCase := range testCases {
		out, err := run(testCase.input)
		if testCase.errExpected {
			checkNil(t, out)
			checkNotNil(t, err)
		} else {
			checkNil(t, err)

			bytes, err := ioutil.ReadAll(out)

			expectedContent, err := ioutil.ReadFile(testCase.expectedOutFile)
			if err != nil {
				panic(err)
			}

			checkStringsEqual(t, string(expectedContent), string(bytes))
		}
	}
}

func checkNil(t *testing.T, x interface{}) {
	if x != nil {
		t.Errorf("%+v is not equal to nil", x)
	}
}

func checkNotNil(t *testing.T, x interface{}) {
	if x == nil {
		t.Error("value unexpectedly nil")
	}
}

func checkStringsEqual(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("strings unequal:\nexpected:\n%s\n\nactual:\n%s\n",
			expected, actual)
	}
}
