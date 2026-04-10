// Package vectorstore provides helpers that wire vector-store backends
// (pgvector, qdrant) into the DI container via Configure.
//
// Ingestor and Retriever were previously defined here and have been moved to
// internal/interfaces so that store packages (pgvectorstore, qdrantstore) and
// higher-level packages (responders, injectors) all share a single canonical
// definition without introducing import cycles.
//
// The type aliases below keep every existing call-site compiling unchanged:
// any code that writes vectorstore.Ingestor or vectorstore.Retriever continues
// to work because a Go type alias is the identical type, not a wrapper.
package vectorstore

import "github.com/lechgu/tichy/internal/interfaces"

// Ingestor is a type alias for interfaces.Ingestor.
// Retained for backward compatibility; prefer interfaces.Ingestor in new code.
type Ingestor = interfaces.Ingestor

// Retriever is a type alias for interfaces.Retriever.
// Retained for backward compatibility; prefer interfaces.Retriever in new code.
type Retriever = interfaces.Retriever
