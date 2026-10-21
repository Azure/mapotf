package pkg_test

import (
	"fmt"
	"testing"
)

func TestRemoveObjectAttributesFullPlan(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		options  string
		want     string
		wantErr  string
	}{
		{"preserve aliases and comments", `{
    # Authored alias.
    azapi = azapi.owner # Keep.
    modtm = modtm
    random = random.owner
  }`, "", `{
    # Authored alias.
    azapi = azapi.owner # Keep.

    random = random.owner
  }`, ""},
		{"inline", `{ azapi = azapi.owner, modtm = modtm }`, "", `{ azapi = azapi.owner, }`, ""},
		{"retain empty default", `{ modtm = modtm }`, "", `{}`, ""},
		{"remove empty opt in", `{ modtm = modtm }`, "remove_empty_attribute = true", "", ""},
		{"already empty", `{}`, "remove_empty_attribute = true", "", ""},
		{"missing attribute", "", "remove_empty_attribute = true", "", ""},
		{"missing key", `{ azapi = azapi.owner }`, "", `{ azapi = azapi.owner }`, ""},
		{"exact dotted keys", `{ "modtm.alias" = modtm.alias, modtm = modtm }`, "", `{ "modtm.alias" = modtm.alias, }`, ""},
		{"computed map rejected", `local.providers`, "", "", "literal object"},
		{"function rejected", `merge({}, local.providers)`, "", "", "literal object"},
		{"computed key rejected", `{ modtm = modtm, (var.key) = azapi.owner }`, "", "", "literal name or string"},
		{"duplicate key rejected", `{ modtm = modtm, "modtm" = modtm.owner }`, "", "", "duplicate object key"},
		{"comprehension rejected", `{ for k, v in local.providers : k => v }`, "", "", "literal object"},
		{"conditional rejected", `var.enabled ? { modtm = modtm } : {}`, "", "", "literal object"},
		{"flag must be bool", `{ modtm = modtm }`, `remove_empty_attribute = "true"`, "", "non-null bool"},
		{"flag cannot be null", `{ modtm = modtm }`, `remove_empty_attribute = null`, "", "non-null bool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := func(provider string) string {
				if provider != "" {
					provider = "  providers = " + provider + "\n"
				}
				return fmt.Sprintf("module \"example\" {\n  source = \"./child\"\n%s  untouched = var.value\n}", provider)
			}
			rule := fmt.Sprintf(`transform "remove_object_attributes" this {
  target_block_address = "module.example"
  attribute_path = ["providers"]
  keys = ["modtm"]
  %s
}`, tc.options)
			runObjectMergePlan(t, code(tc.provider), rule, code(tc.want), tc.wantErr)
		})
	}
}

func TestRemoveObjectAttributesTargetValidation(t *testing.T) {
	for _, target := range []string{`null`, `""`, `1`, `"module.missing"`} {
		t.Run(target, func(t *testing.T) {
			wantErr := "target_block_address"
			if target == `"module.missing"` {
				wantErr = "cannot find block"
			}
			runObjectMergePlan(t, `module "example" {
  source = "./child"
  providers = { modtm = modtm }
}`, fmt.Sprintf(`transform "remove_object_attributes" this {
  target_block_address = %s
  attribute_path = ["providers"]
  keys = ["modtm"]
}`, target), "", wantErr)
		})
	}
}

func TestRemoveObjectAttributesFailureDoesNotSaveEarlierTransforms(t *testing.T) {
	runObjectMergePlan(t, `module "example" {
  source = "./child"
  providers = local.providers
}`, `transform "update_in_place" first {
  target_block_address = "module.example"
  asraw {
    enabled = true
  }
}

transform "remove_object_attributes" this {
  depends_on = [transform.update_in_place.first]
  target_block_address = "module.example"
  attribute_path = ["providers"]
  keys = ["modtm"]
}`, "", "literal object")
}

func TestRemoveObjectAttributesProfileInspectionAndMerge(t *testing.T) {
	runObjectMergePlan(t, `module "example" {
  source = "./child"
  providers = {
    # Owner alias.
    azapi = azapi.owner # Keep.
    modtm = modtm
  }
}
module "empty" {
  source = "./child"
  providers = {}
}
module "absent" {
  source = "./child"
}
terraform {
  required_providers {
    azapi = { source = "Azure/azapi", version = "~> 2.0", configuration_aliases = [azapi.owner] }
  }
}`, `data "module" this {}

transform "remove_object_attributes" cleanup {
  for_each = data.module.this.result
  target_block_address = each.value.mptf.block_address
  attribute_path = ["providers"]
  keys = [for key, expression in object_entries(try(each.value.providers, {})) : key if key == "modtm"]
  remove_empty_attribute = true
}

transform "update_in_place" add {
  depends_on = [transform.remove_object_attributes.cleanup]
  target_block_address = "module.example"
  merge_object_attributes = true
  asraw {
    providers = { random = random.owner }
  }
}

transform "update_in_place" requirement {
  target_block_address = "terraform"
  merge_object_attributes = true
  asraw {
    required_providers {
      azapi = { version = "~> 2.12" }
    }
  }
}`, `module "example" {
  source = "./child"
  providers = {
    # Owner alias.
    azapi = azapi.owner # Keep.
    random = random.owner

  }
}

module "empty" {
  source = "./child"
}

module "absent" {
  source = "./child"
}

terraform {
  required_providers {
    azapi = { source = "Azure/azapi", version = "~> 2.12", configuration_aliases = [azapi.owner] }
  }
}`, "")
}

func TestRemoveObjectAttributesLiteralPathsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    string
		wantErr string
	}{
		{"nested literal path", `attribute_path = ["settings", "a.b"]
keys = ["modtm"]`, ""},
		{"empty path", `attribute_path = []
keys = ["modtm"]`, "at least one segment"},
		{"nested empty attribute option", `attribute_path = ["settings", "a.b"]
keys = ["modtm"]
remove_empty_attribute = true`, "top-level attribute_path"},
		{"missing path", `keys = ["modtm"]`, "attribute_path"},
		{"missing keys", `attribute_path = ["settings"]`, "keys"},
		{"null keys", `attribute_path = ["settings"]
keys = null`, "non-null list"},
		{"nonstring keys", `attribute_path = ["settings"]
keys = [1]`, "non-null strings"},
		{"null path segment", `attribute_path = [null]
keys = ["modtm"]`, "non-null strings"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `module "example" {
  source = "./child"
  settings = { "a.b" = { modtm = modtm, azapi = azapi.owner }, a = { b = { modtm = modtm } } }
}`
			want := `module "example" {
  source = "./child"
  settings = { "a.b" = { azapi = azapi.owner }, a = { b = { modtm = modtm } } }
}`
			runObjectMergePlan(t, source, fmt.Sprintf(`transform "remove_object_attributes" this {
  target_block_address = "module.example"
  %s
}`, tc.args), want, tc.wantErr)
		})
	}
}
