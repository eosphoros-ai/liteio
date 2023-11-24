/*
Copyright 2018 The OpenEBS Authors.

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

package misc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	diskList := make([]string, 0)
	diskList = append(diskList, "Key1")
	diskList = append(diskList, "Key3")
	tests := map[string]struct {
		diskName string
		expected bool
	}{
		"giving a key which is not present in slice": {diskName: "Key0", expected: false},
		"giving a key which is present in slice":     {diskName: "Key3", expected: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, Contains(diskList, test.diskName))
		})
	}
}

func TestContainsIgnoredCase(t *testing.T) {
	diskList := make([]string, 0)
	diskList = append(diskList, "Key1")
	diskList = append(diskList, "Key3")
	tests := map[string]struct {
		diskName string
		expected bool
	}{
		"giving a key which is not present in slice": {diskName: "keY0", expected: false},
		"giving a key which is present in slice":     {diskName: "KEy3", expected: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, ContainsIgnoredCase(diskList, test.diskName))
		})
	}
}

func TestMatchIgnoredCase(t *testing.T) {
	mkList := make([]string, 0)
	mkList = append(mkList, "loop")
	mkList = append(mkList, "/dev/sr0")
	tests := map[string]struct {
		diskPath string
		expected bool
	}{
		"diskPath contains one of the keys ": {diskPath: "/dev/loop0", expected: true},
		"diskPath matches complete key":      {diskPath: "/dev/sr0", expected: true},
		"diskPath does not match any keys":   {diskPath: "/dev/sdb", expected: false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, MatchIgnoredCase(mkList, test.diskPath))
		})
	}
}

func TestRemoveString(t *testing.T) {
	slice1 := []string{"val1", "val2", "val3"}
	slice2 := []string{"val1", "val2", "val3", "val1"}
	slice3 := []string{"val1", "val2", "val1", "val3"}
	slice4 := []string{"val2", "val1", "val1", "val3"}
	tests := map[string]struct {
		actual      []string
		removeValue string
		expected    []string
	}{
		"value is at start":                 {slice1, "val1", []string{"val2", "val3"}},
		"value is at end":                   {slice1, "val3", []string{"val1", "val2"}},
		"value is in between":               {slice1, "val2", []string{"val1", "val3"}},
		"value is twice at start & end":     {slice2, "val1", []string{"val2", "val3"}},
		"value is twice at start & between": {slice3, "val1", []string{"val2", "val3"}},
		"value is twice in between":         {slice4, "val1", []string{"val2", "val3"}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, RemoveString(test.actual, test.removeValue))
		})
	}
}

func TestIsMatchRegex(t *testing.T) {
	tests := map[string]struct {
		regex    string
		str      string
		expected bool
	}{
		"string matches without the need of regex portion": {
			regex:    "/dev/sda(([0-9]*|p[0-9]+))$",
			str:      "/dev/sda",
			expected: true,
		},
		"string matches with regex": {
			regex:    "/dev/sda(([0-9]*|p[0-9]+))$",
			str:      "/dev/sda1",
			expected: true,
		},
		"string does not match the regex": {
			regex:    "/dev/sda(([0-9]*|p[0-9]+))$",
			str:      "/dev/sdaa",
			expected: false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, IsMatchRegex(test.regex, test.str))
		})
	}
}

// TestAddUniqueStringstoSlice get the method which adds only unique devices
func TestUniqueAddStringstoSlice(t *testing.T) {
	var testCases = []struct {
		testName string
		name     string
		names    []string
		exp      []string
	}{
		{
			name:     "sdb1",
			names:    nil,
			testName: "Handling partitions of first disk",
			exp:      []string{"sdb1"},
		},
		{
			name:     "sdb1",
			names:    []string{"sdc1", "sdc2"},
			testName: "Handling partitions of two disks",
			exp:      []string{"sdc1", "sdc2", "sdb1"},
		},
		{
			name:     "sdd1",
			names:    []string{"sdb1", "sdc1", "sdc2"},
			testName: "Handling partitions of three disks",
			exp:      []string{"sdb1", "sdc1", "sdc2", "sdd1"},
		},
	}

	for _, e := range testCases {
		res := AddUniqueStringtoSlice(e.names, e.name)
		if !assert.Equal(t, res, e.exp) {
			t.Errorf(" Test failed : %v , expected : %v  , got : %v", e.testName, e.exp, res)
		}

	}

}
