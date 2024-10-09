# Data "Data" Block

The `data` data block is used to query and retrieve `data` blocks from a Terraform configuration. This block allows you to filter and collect data blocks based on specific criteria such as the data source type, and whether `count` or `for_each` is used.

## Arguments

- `data_source_type`: This optional argument specifies the type of the data source to filter. It is a string attribute.
- `use_count`: This optional argument is a boolean that, when set to `true`, filters data blocks that use the `count` attribute. The default value is `false`.
- `use_for_each`: This optional argument is a boolean that, when set to `true`, filters data blocks that use the `for_each` attribute. The default value is `false`.

## Attributes

- `result`: This attribute contains the filtered data blocks, all expressions assigned to arguments are evaluated as strings.

## Example - Querying Data Blocks

Here is an example of how to use the `data "data"` block to query data blocks of a specific type:

```terraform
data "data" "example" {
  data_source_type = "azurerm_client_config"
}
```

In this example, the `data "data"` block queries all data blocks of type `azurerm_client_config` and stores the result in the `example` data source.

## Example - Filtering Data Blocks with `count`

Here is an example of how to use the `data "data"` block to filter data blocks that use the `count` attribute:

```terraform
data "data" "example" {
  data_source_type = "fake_data"
  use_count        = true
}
```

In this example, the `data "data"` block filters data blocks of type `fake_data` that use the `count` attribute and stores the result in the `example` data source.

## Example - Filtering Data Blocks with `for_each`

Here is an example of how to use the `data "data"` block to filter data blocks that use the `for_each` attribute:

```terraform
data "data" "example" {
  data_source_type = "fake_data"
  use_for_each     = true
}
```

In this example, the `data "data"` block filters data blocks of type `fake_data` that use the `for_each` attribute and stores the result in the `example` data source.

## Example - Retrieving Results

Assuming we have such Terraform config:

```terraform
data "azurerm_resource_group" "example" {
  name = "existing"
}
```

And our Mapotf config is:

```terraform
data "data" "example" {
  data_source_type = "azurerm_resource_group"
}
```

Here is an example of how to retrieve the results from the `data "data"` block:

```terraform
locals {
  azurerm_resource_group_name_exp = data.data.example.result
}
```

In this example, the object stored in `local.azurerm_resource_group_name_exp` looks like the following hcl object:

```text
{
  azurerm_resource_group: {
    example: {
      mptf: {
        block_address: data.azurerm_resource_group.example,
        block_labels: [
          azurerm_resource_group,
          example
        ],
        block_type: data,
        module: {
          abs_dir: xxx,
          dir: .,
          git_hash: xxx,
          key:,
          source:,
          version:
        },
        range: {
          end_column: 2,
          end_line: 29,
          file_name: main.tf,
          start_column: 1,
          start_line: 27
        },
        terraform_address: data.azurerm_resource_group.example
      },
      name: "existing"
    }
  }
}
```

The results would be aggregated by data type first, then by the block labels.
