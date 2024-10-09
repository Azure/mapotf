# New Block Transform Block

The `new_block` transform block is a powerful tool in Mapotf that allows you to create new blocks dynamically. This is particularly useful when you want to add new resources, variables, or other Terraform blocks programmatically.

## Arguments

- `new_block_type`: This argument specifies the type of the new block (e.g., `resource`, `variable`, etc.). It is a required string attribute.
- `filename`: This argument indicates the file where the new block will be added. It must end with `.tf` and is a required string attribute.
- `labels`: This optional argument allows you to specify labels for the new block. It is a list of strings.
- `body`: This optional argument allows you to specify the body content for the new block as a string of HCL code.
- `asstring`: This nested block is used to specify the transformation that will be applied to the new block. The transformation is defined as a string of Terraform code.
- `asraw`: This nested block is used to specify the transformation that will be applied to the new block. The transformation is defined as raw HCL code. The code is not parsed or evaluated, but is directly inserted into the Terraform configuration. This allows you to write complex transformations that cannot be expressed as a single Terraform expression.

## Example - Creating a New Resource Block

Here is an example of how to use the `new_block` transform block to create a new resource block:

```terraform
transform "new_block" example {
  new_block_type = "resource"
  filename       = "main.tf"
  labels         = ["azurerm_resource_group", "rg"]
  asraw {
    name     = "example"
    location = "East US"
  }
}
```

In this example, a new `resource` block of type `azurerm_resource_group` with the label `rg` is added to the `main.tf` file. The body of the block includes the `name` and `location` attributes.

## Example - Creating a New Variable Block, With `body` Argument

Here is an example of how to use the `new_block` transform block to create a new variable block:

```terraform
transform "new_block" example {
  new_block_type = "variable"
  filename       = "variables.tf"
  labels         = ["example"]
  body           = <<-BODY
    type        = string
    description = "This is an example variable"
  BODY
}
```

In this example, a new `variable` block with the label `example` is added to the `variables.tf` file. The body of the block includes the `type` and `description` attributes.

## Example - Using `asstring` to Define the Block

Here is an example of how to use the `asstring` nested block to define the new block:

```terraform
transform "new_block" example {
  new_block_type = "resource"
  filename       = "main.tf"
  labels         = ["azurerm_resource_group", "example"]
  asstring {
    name     = var.resource_group_name
    location = "East US"
  }
}
```

In this example, the `asstring` nested block is used to define the body of the new `resource` block. `var.resource_group_name` refers to a variable defined elsewhere in the Mapotf configuration file, it would be evaluated as a string, and emitted as tokens in the resulting Terraform configuration.

## Example - Error Handling

The `new_block` transform block includes error handling to ensure that only one of `asraw`, `asstring`, or `body` is set. If more than one is set, an error will be raised.

```terraform
transform "new_block" example {
  new_block_type = "resource"
  filename       = "main.tf"
  labels         = ["azurerm_resource_group", "example"]
  body           = <<-BODY
    name           = "example"
  BODY
  asraw {
    location = "East US"
  }
}
```

In this example, an error will be raised because both `body` and `asstring` are set. Only one of these attributes should be set to avoid conflicts.