# deploy/k8s — cluster target (starter)

- Probes, resource requests/limits, non-root + seccomp, secrets via
  `secretRef` (Sealed-Secrets in prod — never plain values).
- `networkpolicy.yaml`: default-deny ingress/egress, DNS-only egress.
- Gates: `kubeconform` + `kube-linter` where wrapped; pin image digests
  before prod (tag above is a placeholder).
