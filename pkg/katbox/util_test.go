/*
Copyright 2020 PayPal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package katbox

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func Test_pressureFactor(t *testing.T) {
    tests := []struct {
        name string
        total uint64
        free uint64
        headroom       float64
        expectedFactor float64
        expectErr      bool
    }{
        {"underUsage", 1000, 110, .10, 1.0, false},
        {"overUsage", 1000, 99, .10, .99, false},
        {"atUsage", 1000, 100, .10, 1.0, false},
        {"negativeHeadroom", 1000, 100, -.10, 0.0, true},
        {"greaterThan100", 1000, 100, 10, 0.0, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pf, err := pressureFactor(tt.total, tt.free, tt.headroom)

            if tt.expectErr {
                assert.Error(t, err, "expected an error")
            } else {
                assert.EqualValues(t, tt.expectedFactor, pf, "Incorrect pressure factor")
                assert.NoError(t, err, "unexpected error")
            }
        })
    }
}