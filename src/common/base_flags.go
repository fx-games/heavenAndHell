package common

import (
	"fmt"
)

type Flags struct {
	flags map[string]int
}

func NewFlags() *Flags {
	return &Flags{make(map[string]int)}
}

func (f *Flags) SetTrue(flag string) {
	f.flags[flag] = 1
}

func (f *Flags) SetFalse(flag string) {
	delete(f.flags, flag)
}
func (f *Flags) IsSet(flag string) bool {
	_, exists := f.flags[flag]
	return exists
}

func (f *Flags) Increment(flag string) {
	if !f.IsSet(flag) {
		f.flags[flag] = 1
	} else {
		f.flags[flag]++
	}
}

func (f *Flags) AsStrings() []string {
	result := make([]string, 0)
	for flag, value := range f.flags {
		if value >= 0 {
			result = append(result, fmt.Sprintf("%s for %d turns", flag, value))
		} else {
			result = append(result, flag)
		}
	}
	return result
}

func (f *Flags) AllEntries() map[string]int {
	return f.flags
}

func (f *Flags) SetToValue(key string, value int) {
	f.flags[key] = value
}

func (f *Flags) Set(enabled string, value bool) {
	if value {
		f.SetTrue(enabled)
	} else {
		f.SetFalse(enabled)
	}
}
