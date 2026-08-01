# contrib

- **Terraform provider** has moved to its own repo: [Sovforge/terraform-provider-scutum](https://github.com/Sovforge/terraform-provider-scutum) (`registry.terraform.io/Sovforge/scutum`). The Terraform Registry requires providers to live in a repo named `terraform-provider-<name>`, so it no longer lives under `contrib/`.
- **Pulumi provider** has moved to its own repo: [Sovforge/pulumi-scutum](https://github.com/Sovforge/pulumi-scutum), published to npm/PyPI/NuGet/Maven/Go. It's a bridged provider built on top of `terraform-provider-scutum`.
- **`grafana/`**: pre-built Grafana dashboard (`scutum-dashboard.json`) for the Prometheus metrics exposed at `/api/metrics` — see the main [README](../README.md) for setup.
