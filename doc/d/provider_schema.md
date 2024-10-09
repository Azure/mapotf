# Data `"provider_schema"` block

The `provider_schema` data source retrieves the schema from a specified Terraform provider. This schema includes information about the provider's resources, their attributes, and nested blocks.

Only resource schemas would be exported this time.

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

## Schema Details

The `resources` attribute contains detailed information about each resource's schema. This includes the attributes and nested blocks defined for the resource. Each attribute schema includes the type, description, and other metadata.

## Under the Hood

Mapotf uses the Terraform provider schema to retrieve the schema for the specified provider source and version. The schema is retrieved by `terraform providers schema -json -no-color` command.
