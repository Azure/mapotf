package pkg

import "github.com/Azure/golden"

type ResourceData struct {
	*BaseData
	*golden.BaseBlock

	Type string `json:"type" hcl:"type,optional"`
	Name string `json:"name" hcl:"name,optional"`
}
