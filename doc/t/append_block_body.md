# `append_block_body` Transform Block

The `append_block_body` transform block in Mapotf is designed to append additional content to an existing block in a Terraform configuration. This is useful when you need to dynamically add attributes or nested blocks to an existing block.

## Arguments

- `target_block_address`: This argument specifies the address of the block to which the content will be appended. The block address is a string that uniquely identifies a block in a Terraform configuration.
- `block_body`: This argument is a string of HCL code representing the content to be appended to the target block.

## Example - Appending Attributes and Nested Blocks

Here is an example of how to use the `append_block_body` transform block to add attributes and nested blocks to an existing resource block:

```terraform
transform "append_block_body" example {
  target_block_address = "resource.fake_resource.example"
  block_body           = <<-BODY
    tags = {
      environment = "production"
    }
    nested_block {
      id = 123
    }
BODY
}
```

In this example, the `target_block_address` is set to the block address of the `fake_resource` resource. The `block_body` argument specifies the content to be appended, which includes a `tags` attribute and a `nested_block`.

## Detailed Behavior

The `append_block_body` transform block works by parsing the `block_body` content and appending it to the target block. The content can include both attributes and nested blocks. If the target block is a one-line block, it will be converted to a multi-line block before appending the content.

### Example Scenarios

1. **Appending Attributes**:
```terraform
transform "append_block_body" example {
  target_block_address = "resource.fake_resource.example"
  block_body           = "tags = { environment = \"production\" }"
}
```

```terraform
resource "fake_resource" "example" {
  name = "example"
}
```

After applying the transform:
```terraform
resource "fake_resource" "example" {
  name = "example"
  tags = { environment = "production" }
}
```

2. **Appending Nested Blocks**:

```terraform
transform "append_block_body" example {
  target_block_address = "resource.fake_resource.example"
  block_body           = <<-BODY
  nested_block { 
    id = 123 
  }
BODY
}
```

```terraform
resource "fake_resource" "example" {
  name = "example"
}
```

After applying the transform:
```terraform
resource "fake_resource" "example" {
  name = "example"
  nested_block {
    id = 123
  }
}
```

In summary, the `append_block_body` transform block is a powerful tool for dynamically modifying existing Terraform blocks by appending additional content. This allows for flexible and programmatic updates to Terraform configurations.