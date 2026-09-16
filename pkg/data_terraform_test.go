package pkg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Azure/golden"
	"github.com/Azure/mapotf/pkg"
	filesystem "github.com/Azure/mapotf/pkg/fs"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
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
		BaseBlock:         golden.NewBaseBlock(cfg, nil),
		RequiredProviders: map[string]pkg.RequiredProvider{"stale": {}},
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
		expectedError           string
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
			desc: "required_providers with configuration aliases",
			config: `terraform {
  required_providers {
    mycloud = {
      source                = "mycorp/mycloud"
      version               = "~> 1.0"
      configuration_aliases = [mycloud.primary, mycloud.secondary]
    }
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": {
					Source:  p("mycorp/mycloud"),
					Version: p("~> 1.0"),
				},
			},
		},
		{
			desc: "required_providers with aliases only",
			config: `terraform {
  required_providers {
    mycloud = {
      configuration_aliases = [mycloud.primary]
    }
  }
}`,
			wantedRequiredProviders: map[string]pkg.RequiredProvider{
				"mycloud": {},
			},
		},
		{
			desc: "required_providers with aliases and invalid source expression",
			config: `terraform {
  required_providers {
    mycloud = {
      source                = var.provider_source
      configuration_aliases = [mycloud.primary]
    }
  }
}`,
			expectedError: "required_providers.mycloud.source",
		},
		{
			desc: "required_providers with aliases and invalid version expression",
			config: `terraform {
  required_providers {
    mycloud = {
      version               = var.provider_version
      configuration_aliases = [mycloud.primary]
    }
  }
}`,
			expectedError: "required_providers.mycloud.version",
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
			if c.expectedError != "" {
				require.ErrorContains(t, err, c.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.wantedTerraformVersion, data.RequiredVersion)
			require.Equal(t, c.wantedRequiredProviders, data.RequiredProviders)

			exported := golden.Values([]*pkg.TerraformData{data}).GetAttr("terraform").GetAttr("")
			require.Equal(t, data.Id(), exported.GetAttr("id").AsString())
			providers := exported.GetAttr("required_providers")
			require.True(t, providers.Type().Equals(cty.Map(cty.Object(map[string]cty.Type{
				"source":  cty.String,
				"version": cty.String,
			}))))
			require.Equal(t, len(c.wantedRequiredProviders), providers.LengthInt())
			for name, provider := range c.wantedRequiredProviders {
				value := providers.Index(cty.StringVal(name))
				for attribute, expected := range map[string]*string{
					"source": provider.Source, "version": provider.Version,
				} {
					actual := value.GetAttr(attribute)
					if expected == nil {
						require.True(t, actual.RawEquals(cty.NullVal(cty.String)))
					} else {
						require.True(t, actual.RawEquals(cty.StringVal(*expected)))
					}
				}
			}
		})
	}
}

func TestTerraformData_RequiredProvidersFullPlan(t *testing.T) {
	cases := []struct {
		desc               string
		providerAttributes string
		wantedSource       cty.Value
		wantedVersion      cty.Value
		expectedError      string
	}{
		{
			desc: "source and version",
			providerAttributes: `source = "Azure/azapi"
version = "~> 2.0"`,
			wantedSource:  cty.StringVal("Azure/azapi"),
			wantedVersion: cty.StringVal("~> 2.0"),
		},
		{
			desc:               "source only",
			providerAttributes: `source = "Azure/azapi"`,
			wantedSource:       cty.StringVal("Azure/azapi"),
			wantedVersion:      cty.NullVal(cty.String),
		},
		{
			desc:               "version only",
			providerAttributes: `version = "~> 2.0"`,
			wantedSource:       cty.NullVal(cty.String),
			wantedVersion:      cty.StringVal("~> 2.0"),
		},
		{
			desc:          "empty metadata",
			wantedSource:  cty.NullVal(cty.String),
			wantedVersion: cty.NullVal(cty.String),
		},
		{
			desc:               "aliases only",
			providerAttributes: `configuration_aliases = [azapi.primary, azapi.secondary]`,
			wantedSource:       cty.NullVal(cty.String),
			wantedVersion:      cty.NullVal(cty.String),
		},
		{
			desc: "invalid source expression",
			providerAttributes: `source = var.provider_source
configuration_aliases = [azapi.primary]`,
			expectedError: "required_providers.azapi.source",
		},
		{
			desc: "invalid version expression",
			providerAttributes: `version = var.provider_version
configuration_aliases = [azapi.primary]`,
			expectedError: "required_providers.azapi.version",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fs := fakeFs(map[string]string{
				filepath.Join("terraform", "main.tf"): fmt.Sprintf(`terraform {
  required_providers {
    azapi = {
      %s
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}`, c.providerAttributes),
				filepath.Join("mptf", "main.mptf.hcl"): `data "terraform" this {}

locals {
  metadata = {
    providers            = data.terraform.this.required_providers
    can_source           = can(data.terraform.this.required_providers.azapi.source)
    can_version          = can(data.terraform.this.required_providers.azapi.version)
    try_source           = try(data.terraform.this.required_providers.azapi.source, "fallback")
    try_version          = try(data.terraform.this.required_providers.azapi.version, "fallback")
    default_source       = coalesce(data.terraform.this.required_providers.azapi.source, "fallback")
    default_version      = coalesce(data.terraform.this.required_providers.azapi.version, "fallback")
    can_missing_provider = can(data.terraform.this.required_providers.missing.source)
    try_missing_provider = try(data.terraform.this.required_providers.missing.source, "fallback")
    neighboring_source   = data.terraform.this.required_providers.random.source
    neighboring_version  = data.terraform.this.required_providers.random.version
  }
}

transform "ensure_local" metadata {
  name               = "provider_metadata"
  fallback_file_name = "metadata.tf"
  value_as_string    = tohcl(local.metadata)
}`,
			})
			stub := gostub.Stub(&filesystem.Fs, fs)
			defer stub.Reset()

			hclBlocks, err := pkg.LoadMPTFHclBlocks(false, "mptf")
			require.NoError(t, err)
			cfg, err := pkg.NewMetaProgrammingTFConfig(&pkg.TerraformModuleRef{
				Dir: "terraform", AbsDir: "terraform",
			}, nil, hclBlocks, nil, context.TODO())
			require.NoError(t, err)
			plan, err := pkg.RunMetaProgrammingTFPlan(cfg)
			if c.expectedError != "" {
				require.ErrorContains(t, err, c.expectedError)
				require.Nil(t, plan)
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Transforms, 1)

			wantedProviders := cty.MapVal(map[string]cty.Value{
				"azapi": cty.ObjectVal(map[string]cty.Value{
					"source": c.wantedSource, "version": c.wantedVersion,
				}),
				"random": cty.ObjectVal(map[string]cty.Value{
					"source":  cty.StringVal("hashicorp/random"),
					"version": cty.StringVal("~> 3.6"),
				}),
			})
			defaultSource, defaultVersion := c.wantedSource, c.wantedVersion
			if defaultSource.IsNull() {
				defaultSource = cty.StringVal("fallback")
			}
			if defaultVersion.IsNull() {
				defaultVersion = cty.StringVal("fallback")
			}
			wantedMetadata := cty.ObjectVal(map[string]cty.Value{
				"providers":            wantedProviders,
				"can_source":           cty.True,
				"can_version":          cty.True,
				"try_source":           c.wantedSource,
				"try_version":          c.wantedVersion,
				"default_source":       defaultSource,
				"default_version":      defaultVersion,
				"can_missing_provider": cty.False,
				"try_missing_provider": cty.StringVal("fallback"),
				"neighboring_source":   cty.StringVal("hashicorp/random"),
				"neighboring_version":  cty.StringVal("~> 3.6"),
			})
			ctx := cfg.EvalContext()
			exported := ctx.Variables["data"].GetAttr("terraform").GetAttr("this")
			require.True(t, wantedProviders.RawEquals(exported.GetAttr("required_providers")))
			require.True(t, wantedMetadata.RawEquals(ctx.Variables["local"].GetAttr("metadata")))
			require.True(t, cfg.TerraformBlock().EvalContext().RawEquals(exported.GetAttr("block")))

			require.NoError(t, plan.Apply())
			content, err := afero.ReadFile(fs, filepath.Join("terraform", "metadata.tf"))
			require.NoError(t, err)
			wantedFile := fmt.Sprintf("locals {\nprovider_metadata = %s\n}", hclwrite.TokensForValue(wantedMetadata).Bytes())
			require.Equal(t, formatHcl(wantedFile), formatHcl(string(content)))
		})
	}
}

func p[T any](v T) *T {
	return &v
}
