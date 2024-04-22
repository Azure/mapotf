# Meta Programming for Terraform

`mptf` is a meta programming tool designed to work with Terraform.

As Terraform module's author, you might meet such scenario: an user required a `ignore_changes` setting in your resource block because they've used their own customized Azure Policy (or AWS config), these remediation services could modify the resources out of band, which would bring config drift to their state. Different users need to ignore different attributes, but unfortunately, Terraform doesn't support `var` in some arguments, such as `prevent_destroy` or `ignore_changes`.

Another scenario is, there are some common design patterns, such as creating private endpoint for RDS, S3 bucket and so on. Different users might work on their own modules, but the patterns are the same. If we can provide a common pattern library, then the module's author won't need to search or the examples or tutorials, all they need to do is search for the patterns library, and apply.

`mptf` tools has two phases, match and transform. You can use `data` block to match the Terraform blocks you're interested in, then you can define `transform` blocks in instruct how to mutate the original Terraform code, you can update the block in place, or insert new blocks, or remove the given parts inside a Terraform block.

## An example

1. Clone [terraform-azurerm-aks](https://github.com/Azure/terraform-azurerm-aks.git)
2. Run `mptf apply --mptf-dir example/customize_aks_ignore_changes --tf-dir <AKS_MODULE_PATH>`

You'll be asked for permission to carry the plan, input `yes` to continue:

```shell
Do you want to apply this plan? Only `yes` would be accepted. (yes/no): yes
Plan applied successfully.
```

Then you should see the `lifecycle` in aks module's `main.tf` file like this:

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

This tool is still in development.