# Applying `prevent_destroy = true` to Protect Your Resource

This guide outlines the process of applying the `prevent_destroy = true` configuration to your Terraform resources using the `main.mptf.hcl` file. This configuration is crucial for preventing accidental deletion of critical resources, ensuring that your infrastructure remains stable and secure.

## Purpose

The primary objectives of setting `prevent_destroy = true` are:

- **Prevent Accidental Deletion**: To safeguard against the unintended destruction of resources, which can lead to significant downtime and data loss.
- **Compliance and Governance**: In many cases, regulatory requirements dictate that certain resources must not be easily deletable, making this configuration essential for compliance.

As module author we cannot declare `prevent_destroy = true` in a reusable module, otherwise the consumer could not destroy these resources since they're not able to modify the code inside module. This configuration utilizes the `mapotf` tool's ability to dynamically modify Terraform code, allowing for the seamless enforcement of the `prevent_destroy` policy across your resources.

### How to Apply

Before you begin, check [`main.tf`](./main.tf) file in this directory, which might look like this:

```hcl
resource "random_string" "test" {
  length = 10
}
```

1. **Define the Policy**: In your `main.mptf.hcl` file, specify the `prevent_destroy` variable and set it to `true`. Optionally, use the `root_only` variable to apply the policy only to root module resources.

```terraform
variable "prevent_destroy" {
  type    = bool
  default = true
}

variable "root_only" {
  type    = bool
  default = false
}
```

2. **Identify Resources**: Utilize the `data` and `locals` blocks to dynamically identify the resources within your Terraform configuration that you wish to protect.

```terraform
data "resource" all_resource {
}

locals {
  all_resource_blocks = flatten([
    for resource_type, resource_blocks in data.resource.all_resource.result :resource_blocks
  ])
  mptfs               = flatten([for _, blocks in local.all_resource_blocks : [for b in blocks : b.mptf]])
  addresses           = var.root_only ? [for mptf in local.mptfs : mptf.block_address if mptf.module.dir == "."] : [for mptf in local.mptfs : mptf.block_address]
}
```

3. **Apply the Transformation**: The `transform "update_in_place"` block in `main.mptf.hcl` is configured to iterate over the identified resources and apply the `prevent_destroy = true` setting to each.

```terraform
transform "update_in_place" set_prevent_destroy {
  for_each             = try(local.addresses, [])
  target_block_address = each.value

  asstring {
    lifecycle {
      prevent_destroy = var.prevent_destroy
    }
  }
}
```

4. **Execute the Transformation**: Run the `mapotf init` and `mapotf apply --auto-approve --mptf-dir . --tf-dir . --mptf-var prevent_destroy=true` command to apply the changes. This command reads your `main.mptf.hcl` and `main.tf` files, applying the `prevent_destroy = true` configuration where applicable, then run `terraform apply -auto-approve`, after apply, revert all transformations by running `mapotf reset`.

After the first running, you should have one `random_string` resource with length `10`.

Then, let's change the `length` in `main.tf`:

```hcl
resource "random_string" "test" {
  length = 11
}
```

Let's try again: `mapotf apply -auto-approve --mptf-dir . --tf-dir . --mptf-var prevent_destroy=true`, you would see:

```text
random_string.test: Refreshing state... [id=7b9zNR)$<b]

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
-/+ destroy and then create replacement

Terraform planned the following actions, but then encountered a problem:

  # random_string.test must be replaced
-/+ resource "random_string" "test" {
      ~ id          = "7b9zNR)$<b" -> (known after apply)
      ~ length      = 10 -> 11 # forces replacement
      ~ result      = "7b9zNR)$<b" -> (known after apply)
        # (9 unchanged attributes hidden)
    }

Plan: 1 to add, 0 to change, 1 to destroy.
╷
│ Error: Instance cannot be destroyed
│
│   on main.tf line 1:
│    1: resource "random_string" "test" {
│
│ Resource random_string.test has lifecycle.prevent_destroy set, but the plan calls for this resource to be destroyed. To avoid this error and continue with the plan, either disable lifecycle.prevent_destroy or reduce the scope 
│ of the plan using the -target option.
╵
Error: exit status 1
```

This `random_string` instance has been protected from a re-creation.

## Conclusion

By following the steps outlined in this guide, you can use `prevent_destroy = true` to protect any types of resources including those defined inside modules from accidental deletion. (When you'd like to apply your mapotf policies to resources inside other module, remember use `-r` flag, like `mapotf apply -r ......`)
