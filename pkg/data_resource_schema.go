package pkg

import (
	"fmt"
	"github.com/Azure/golden"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/zclconf/go-cty/cty"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var _ Data = &ResourceSchemasData{}

type ResourceSchemasData struct {
	*BaseData
	*golden.BaseBlock

	ProviderName string    `hcl:"resource_type"`
	Source       string    `hcl:"source"`
	Version      string    `hcl:"version"`
	Result       cty.Value `attribute:"result"`
}

func (r *ResourceSchemasData) Type() string {
	return "resource_schemas"
}

func (r *ResourceSchemasData) ExecuteDuringPlan() error {
	tfPath, err := r.getTerraformPath()
	if err != nil || tfPath == "" {
		tmpTf, err := os.MkdirTemp("", "terraform*")
		if err != nil {
			return fmt.Errorf("cannot make temp dir for terraform: %+v", err)
		}
		defer func() {
			_ = os.RemoveAll(tmpTf)
		}()
		v := &releases.LatestVersion{
			Product:    product.Terraform,
			InstallDir: tmpTf,
		}
		tfPath, err = v.Install(r.Context())
		if err != nil {
			return fmt.Errorf("cannot install terraform: %+v", err)
		}
	}
}

func (r *ResourceSchemasData) getTerraformPath() (string, error) {
	var cmd *exec.Cmd

	if r.isWindows() {
		cmd = exec.Command("where", "terraform")
	} else {
		cmd = exec.Command("which", "terraform")
	}

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (r ResourceSchemasData) isWindows() bool {
	return runtime.GOOS == "windows"
}
