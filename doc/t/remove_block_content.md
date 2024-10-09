# Remove Block Content Transform Block

The `remove_block_element` transform block is a tool in Mapotf that allows you to remove specific content from existing blocks. This is useful when you need to clean up or modify configurations by removing unwanted nested blocks or attributes.

## Arguments

- `target_block_address`: This argument specifies the address of the block from which the content will be removed. The block address is a string that uniquely identifies a block in a Terraform configuration.

- `paths`: This argument is a list of strings, each representing a path to the content that should be removed. The path can point to nested blocks or attributes within the target block.

## Example

Here is an example of how to use the `remove_block_element` transform block to remove nested blocks and attributes from a resource:

```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths                = ["nested_block", "nested_block2/attr"]
}
```

In this example, the `target_block_address` is set to the block address of the `fake_resource` resource. The `paths` argument specifies two paths: `nested_block` and `nested_block2/attr`. The first path removes the `nested_block` block, and the second path removes the `attr` attribute from the `nested_block2` block.

## Detailed Behavior

The `remove_block_element` transform block works by traversing the specified paths and removing the corresponding content from the target block. The paths can point to both nested blocks and attributes. If a path points to a nested block, the entire block is removed. If a path points to an attribute, only the attribute is removed.

### Example Scenarios

1. **Removing a Single Nested Block**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths                = ["nested_block"]
}
```

```terraform
resource "fake_resource" this {
  nested_block {}
  non_target_block {}
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  non_target_block {}
}
```

2. **Removing Multiple Nested Blocks**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block", "nested_block2"]
}
```

```terraform
resource "fake_resource" this {
  nested_block {}
  nested_block2 {}
  non_target_block {}
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  non_target_block {}
}
```

3. **Removing Deeply Nested Blocks**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block/second_nested_block"]
}
```

```terraform
resource "fake_resource" this {
  nested_block {
    non_target_block {}
  }
  nested_block {
    second_nested_block {}
  }
  non_target_block {}
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  nested_block {
    non_target_block {}
  }
  nested_block {}
  non_target_block {}
}
```

4. **Removing Attributes**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["attr"]
}
```

```terraform
resource "fake_resource" this {
  attr = 1
  nested_block {
    attr = "hello"
  }
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  nested_block {
    attr = "hello"
  }
}
```

5. **Removing Attributes in Nested Blocks**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block/attr"]
}
```

```terraform
resource "fake_resource" this {
  attr = 1
  nested_block {
    attr = "hello"
  }
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  attr = 1
  nested_block {}
}
```

6. **Removing Attributes in Dynamic Nested Blocks**:
```terraform
transform "remove_block_element" this {
  target_block_address = "resource.fake_resource.this"
  paths = ["nested_block/attr"]
}
```

```terraform
resource "fake_resource" this {
  attr = 1
  dynamic "nested_block" {
    for_each = [1]
    content {
      attr = "hello"
    }
  }
}
```

After applying the transform:
```terraform
resource "fake_resource" this {
  attr = 1
  dynamic "nested_block" {
    for_each = [1]
    content {}
  }
}
```

In summary, the `remove_block_element` transform block is a versatile tool for cleaning up and modifying Terraform configurations by removing unwanted nested blocks and attributes.