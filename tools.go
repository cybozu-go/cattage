//go:build tools
// +build tools

package cattage

import (
	// These are to declare dependency on tools
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
