package pkg

import "github.com/Azure/golden"

var _ golden.Config = &MetaProgrammingTFConfig{}

type MetaProgrammingTFConfig struct {
	*golden.BaseConfig
}
