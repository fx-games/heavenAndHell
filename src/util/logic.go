package util

import (
	"regexp"
	"strconv"
	"strings"
)

// pair with: https://github.com/Knetic/govaluate
// for full effect
// or checkout (b/c still maintained) https://github.com/expr-lang/expr

type Arguments []string

func (a Arguments) Get(index int) string {
	if index >= len(a) || index < 0 {
		return ""
	}
	return a[index]
}

func (a Arguments) GetInt(index int) int {
	if index >= len(a) || index < 0 {
		return 0
	}
	result, err := strconv.Atoi(a[index])
	if err != nil {
		return 0
	}
	return result
}

func (a Arguments) GetFloat(i int) float64 {
	if i >= len(a) || i < 0 {
		return 0
	}
	result, err := strconv.ParseFloat(a[i], 64)
	if err != nil {
		return 0
	}
	return result
}

func (a Arguments) IsEmpty() bool {
	return len(a) == 0
}

func (a Arguments) GetIntOrDefault(index int, defaultValue int) int {
	if index >= len(a) || index < 0 {
		return defaultValue
	}
	result, err := strconv.Atoi(a[index])
	if err != nil {
		return defaultValue
	}
	return result
}

func LooksLikeAFunction(line string) bool {
	regexpPattern := regexp.MustCompile(`([A-Za-z_]+)\((.*)\)`)
	return regexpPattern.MatchString(line)
}
func NewArgumentsFromString(args string) Arguments {
	return trimAll(strings.Split(args, "|"))
}
func GetNameAndArgs(line string) (string, Arguments) {
	regexpPattern := regexp.MustCompile(`([A-Za-z_]+)\((.*)\)`)
	matches := regexpPattern.FindStringSubmatch(line)
	if matches == nil {
		return line, Arguments{}
	}
	name := matches[1]
	argString := matches[2]
	if argString == "" {
		return name, Arguments{}
	}
	args := trimAll(strings.Split(argString, "|"))
	return name, args
}

func trimAll(split []string) []string {
	for i, s := range split {
		split[i] = strings.TrimSpace(s)
	}
	return split
}
