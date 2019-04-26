package cmd

import (
	"strconv"
	"strings"
)

const (
	defaultMem = 12288
)

type instance struct {
	cores int
	mem   int
	cost  float64
}

func (inst *instance) constructCustomMachineType() string {
	var sb strings.Builder
	sb.WriteString("custom-")
	sb.WriteString(strconv.Itoa(inst.cores))
	sb.WriteString("-")
	sb.WriteString(strconv.Itoa(defaultMem))
	return sb.String()
}

func getDefaultInstanceConfigs() [][]instance {
	return [][]instance{
		{
			{2, 8, 0.066},
			{4, 8, 0.109},
			{8, 8, 0.196},
		},
		{
			{2, 12, 0.066},
			{4, 12, 0.109},
			{8, 12, 0.196},
		},
	}
}
