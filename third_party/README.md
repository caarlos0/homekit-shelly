# HAP dependency

`hap/` contains the runtime Go sources of `github.com/brutella/hap` v0.0.35,
copied with `go mod vendor`. Its original license is retained.

The local patch requires successful PIN-proof verification before the final
pairing step. Failed verification clears that authorization, and a final-step
attempt consumes it.

No upstream release with this check was available when this copy was added.
The module replacement keeps the application on the patched sources. Remove
the replacement and this copy only after an upstream update passes
`TestPairingRequiresPINProof`.
