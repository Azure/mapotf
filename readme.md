# Meta Programming for Terraform

`mapotf` stands for `MetA PrOgramming for TerraForm`.

`mapotf` is a meta programming tool designed to work with Terraform.

As Terraform module's author, you might meet such scenario: an user required a `ignore_changes` setting in your resource block because they've used their own customized Azure Policy (or AWS config), these remediation services could modify the resources out of band, which would bring config drift to their state. Different users need to ignore different attributes, but unfortunately, Terraform doesn't support `var` in some arguments, such as `prevent_destroy` or `ignore_changes`.

Another scenario is, there are some common design patterns, such as creating private endpoint for RDS, S3 bucket and so on. Different users might work on their own modules, but the patterns are the same. If we can provide a common pattern library, then the module's author won't need to search or the examples or tutorials, all they need to do is search for the patterns library, and apply.

`mapotf` tools has two phases, match and transform. You can use `data` block to match the Terraform blocks you're interested in, then you can define `transform` blocks in instruct how to mutate the original Terraform code, you can update the block in place, or insert new blocks, or remove the given parts inside a Terraform block.

## An example

1. Clone [terraform-azurerm-aks](https://github.com/Azure/terraform-azurerm-aks.git)
2. Switch into one of it's example, like `cd example/startup`
3. Run `mapotf init` or `terraform init`
3. Run `mapotf apply -r --mptf-dir git::https://github.com/Azure/mapotf.git//example/customize_aks_ignore_changes`

`mapotf` would:

1. Download `example/customize_aks_ignore_changes` folder from `https://github.com/Azure/mapotf`, store the folder in a temp folder.
2. Match all `azurerm_kubernetes_cluster` resource blocks, patch them by adding `microsoft_defender[0].log_analytics_workspace_id` into it's `ignore_changes` list.
3. Run `terraform apply` for you

You'll be asked for permission to carry the plan, this is output by Terraform:

```text
Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value:
```

Meanwhile, you would see `*.tf.mptfbackup` files for each `.tf` file, which contains the original content from those `.tf` files. If you check `../../main.tf` file (referenced via `module` block's `source` `../../`), you would see the `ignore_changes` list of `azurerm_kubernetes_cluster` has been changed as expected.

```hcl
lifecycle {
    ignore_changes = [
      microsoft_defender[0].log_analytics_workspace_id,
      http_application_routing_enabled,
      http_proxy_config[0].no_proxy,
      kubernetes_version,
      public_network_access_enabled,
      # we might have a random suffix in cluster's name so we have to ignore it here, but we've traced user supplied cluster name by `null_resource.kubernetes_cluster_name_keeper` so when the name is changed we'll recreate this resource.
      name,
    ]
    ...
  }
```

If you press `no`, Terraform would quit, and all `.tf` file would be reverted, all `.tf.mptfbackup` files would be removed.

You can also use `transform` command to carry the transforms without invoke Terraform `mapotf transform -r --mptf-dir git::https://github.com/Azure/mapotf.git//example/customize_aks_ignore_changes`, then like `apply`, but we'll leave transformed `.tf` files along with `.tf.mptfbacup` files there for you, you can check them, apply them by calling `terraform` command, or revert all changes by `mapotf reset`. If you decide to keep these changes and remove all backup files, you can run `mapotf clean-backup`.

This tool is still in development, but you're welcome to give it a try.