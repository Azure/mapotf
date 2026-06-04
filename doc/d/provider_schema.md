# Data `"provider_schema"` block

The `provider_schema` data source retrieves the schema from a specified Terraform provider. This schema includes information about the provider's resources, data sources, their attributes, and nested blocks.

Both resource and data source schemas are exported, and the data source additionally exposes pre-sorted required / optional attribute name lists per type, intended to feed directly into `reorder_attributes.body_attributes`.

## Example Usage

```hcl
data "provider_schema" azurerm {
  provider_source  = "hashicorp/azurerm"
  provider_version = "~> 4.0"
}

locals {
  resources_support_tags = toset([for name, r in data.provider_schema.azurerm.resources : name if try(r.block.attributes["tags"].type == ["map", "string"], false)])
}
```

## Arguments

- `provider_source` (String, Required): The source of the provider, typically in the format `hashicorp/azurerm`.
- `provider_version` (String, Required): The version constraint for the provider, e.g., `~> 4.0`.

## Attributes

- `resources` (Map): A map of resource schemas provided by the specified provider. Each resource schema includes:
    - `version` (`number`): The version of the particular resource schema.
    - `block` (Object): The block schema of the resource, which includes:
        - `attributes` (Map): The attributes defined at the particular level of this block.
        - `block_types` (Map): Any nested blocks within this particular block.
        - `description` (String): The description for this block and format of the description. If no kind is provided, it can be assumed to be plain text.
- `data_sources` (Map): Same shape as `resources`, but for data source schemas.
- `resources_required_attributes` (Map of List of String): For every resource type the provider declares, the alphabetically-sorted list of attribute names whose schema marks them `required: true`. Empty list if the resource has no required attributes.
- `resources_optional_attributes` (Map of List of String): For every resource type, the alphabetically-sorted list of attribute names whose schema marks them `optional: true`. Includes attributes that are also `computed`; the boundary that matters for body ordering is "the user is allowed to set this value".
- `data_sources_required_attributes` (Map of List of String): Same shape, for data source types.
- `data_sources_optional_attributes` (Map of List of String): Same shape, for data source types.

## Schema Details

The `resources` and `data_sources` attributes contain detailed information about each resource / data source's schema. This includes the attributes and nested blocks defined for them. Each attribute schema includes the type, description, and other metadata.

## Example - Schema-driven body ordering with `reorder_attributes`

Feed the pre-sorted required / optional name lists directly into `body_attributes` so every resource of a given type has its body emitted as `required (alphabetical) → optional (alphabetical) → anything-else (alphabetical)`:

```terraform
data "provider_schema" "azurerm" {
  provider_source  = "hashicorp/azurerm"
  provider_version = "~> 4.0"
}

data "resource" "all" {}

locals {
  resource_addresses = merge([
    for t, rs in data.resource.all.result : {
      for n, _ in rs : "resource.${t}.${n}" => t
    }
  ]...)
}

transform "reorder_attributes" "resource_body" {
  for_each             = local.resource_addresses
  target_block_address = each.key
  head_attributes      = ["for_each", "count", "provider"]
  body_attributes = concat(
    try(data.provider_schema.azurerm.resources_required_attributes[each.value], []),
    try(data.provider_schema.azurerm.resources_optional_attributes[each.value], []),
  )
  foot_attributes = ["lifecycle", "depends_on"]
}
```

Each `module.<address>` value in `each.value` is the resource type (e.g. `azurerm_resource_group`), used as the map key into the schema's required / optional attribute lists. `try(..., [])` keeps the transform safe if the provider schema doesn't know about the type (for example after a provider upgrade).

The same pattern works for data sources by substituting `data.data.all` and `data_sources_required_attributes` / `data_sources_optional_attributes`.

## Under the Hood

Mapotf retrieves the schema by running `terraform init` followed by `terraform providers schema -json -no-color` in a temporary directory. The temp directory is reused across data blocks within a single `mapotf transform` run, so multiple `data "provider_schema"` blocks for the same provider source pay the init cost only once.
