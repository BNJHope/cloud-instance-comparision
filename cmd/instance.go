package cmd

import (
	"strconv"
	"strings"
)

type instance struct {
	cores int
	mem   int
	cost  float64
}

type result struct {
	score float64
	inst  *instance
}

func (inst *instance) constructCustomMachineType() string {
	var sb strings.Builder
	sb.WriteString("custom-")
	sb.WriteString(strconv.Itoa(inst.cores))
	sb.WriteString("-")
	sb.WriteString(strconv.Itoa(inst.mem))
	return sb.String()
}

func getDefaultInstanceConfigs() [][]instance {
	return [][]instance{
		{
			//{2, 8192, 0.066},
			{4, 8192, 0.109},
			//{8, 8192, 0.196},
		},
		{
			{2, 12288, 0.066},
			//			{4, 12, 0.109},
			//			{8, 12, 0.196},
		},
	}
}

func getDefaultInstanceConfigsChan() chan (*instance) {
	instanceChan := make(chan *instance, 6)
	for _, instanceRow := range getDefaultInstanceConfigs() {
		for _, instance := range instanceRow {
			instanceChan <- &instance
		}
	}

	return instanceChan
}
