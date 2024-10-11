package pkg_test

import (
	"context"
	"github.com/zclconf/go-cty/cty"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"
)

func TestTerraformData_BlockMptfInfo(t *testing.T) {
	stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
		"/main.tf": `terraform {
  required_providers {
    mycloud = {
      source  = "mycorp/mycloud"
      version = "~> 1.0"
    }
  }
}`,
	}))
	defer stub.Reset()
	cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
		Dir:    "/",
		AbsDir: "/",
	}, nil, nil, nil, context.TODO())
	require.NoError(t, err)

	data := &pkg.TerraformData{
		BaseBlock: golden.NewBaseBlock(cfg, nil),
	}

	err = data.ExecuteDuringPlan()
	require.NoError(t, err)
	require.NotEqual(t, cty.NilVal, data.Block)
}

func TestTerraformData_RequiredProviders(t *testing.T) {

	cases := []struct {
		desc                    string
		config                  string
		wantedTerraformVersion  *string
		wantedRequiredProviders map[string]pkg.RequiredProvider
	}{
		{
			desc: "required_providers only",
			config: `terraform {
  required_providers {
    mycloud = {
      source  = "mycorp/mycloud"
      version = "~> 1.0"
    }
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": pkg.RequiredProvider{
					Source:  p("mycorp/mycloud"),
					Version: p("~> 1.0"),
				},
			},
		},
		{
			desc: "required_version only",
			config: `terraform {
  required_version = ">= 1.2"
}`,
			wantedTerraformVersion: p(">= 1.2"),
		},
		{
			desc: "required_providers with version only",
			config: `terraform {
  required_providers {
    mycloud = {
      version = "~> 1.0"
    }
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": {
					Version: p("~> 1.0"),
				},
			},
		},
		{
			desc: "required_providers with source only",
			config: `terraform {
  required_providers {
    mycloud = {
      source  = "mycorp/mycloud"
    }
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": {
					Source: p("mycorp/mycloud"),
				},
			},
		},
		{
			desc: "required_providers with all attributes and multiple providers",
			config: `terraform {
  required_version = ">= 1.2"
  required_providers {
    mycloud = {
      source  = "mycorp/mycloud"
	  version = ">= 1.0"
    }
	mycloud2 = {
      source  = "mycorp/mycloud2"
	  version = ">= 2.0"
    }
  }
}`,
			wantedTerraformVersion: p(">= 1.2"),
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": {
					Source:  p("mycorp/mycloud"),
					Version: p(">= 1.0"),
				},
				"mycloud2": {
					Source:  p("mycorp/mycloud2"),
					Version: p(">= 2.0"),
				},
			},
		},
		{
			desc: "empty terraform block",
			config: `terraform {
}`,
		},
		{
			desc: "terraform block with empty required providers block",
			config: `terraform {
  required_providers {
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{},
		},
		{
			desc:   "no terraform block",
			config: ``,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stub := gostub.Stub(&filesystem.Fs, fakeFs(map[string]string{
				"/main.tf": c.config,
			}))
			defer stub.Reset()
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir:    "/",
				AbsDir: "/",
			}, nil, nil, nil, context.TODO())
			require.NoError(t, err)

			data := &pkg.TerraformData{
				BaseBlock: golden.NewBaseBlock(cfg, nil),
			}

			err = data.ExecuteDuringPlan()
			require.NoError(t, err)
			require.Equal(t, c.wantedTerraformVersion, data.RequiredVersion)
			require.Equal(t, c.wantedRequiredProviders, data.RequiredProviders)
		})
	}
}

func p[T any](v T) *T {
	return &v
}
