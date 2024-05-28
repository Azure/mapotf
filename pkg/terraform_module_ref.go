package pkg

import "github.com/Azure/mapotf/pkg/terraform"

type TerraformModuleRef struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Dir     string `json:"Dir"`
	AbsDir  string
	Version string `json:"Version"`
}

func (r TerraformModuleRef) toTerraformPkgType() terraform.TerraformModuleRef {
	return terraform.TerraformModuleRef{
		Key:     r.Key,
		Source:  r.Source,
		Dir:     r.Dir,
		AbsDir:  r.AbsDir,
		Version: r.Version,
	}
}
