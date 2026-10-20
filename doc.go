// Package clix wires the global flags --quiet, --verbose, --color and --narration, the console, exit codes and signal handling around a cobra command tree.
// A main needs only a few lines: build the root command and call clix.Run(root, clix.Options{...}).
// The house style for what a command prints is in docs/conventions.md and the contract in docs/reference.
package clix
