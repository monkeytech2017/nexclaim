package extractor

import "context"

// TMTResolverFactory builds a per-hospital HIS-drug-code → TMT resolver.
//
// It is invoked lazily by an extractor once the hospital code is known (from
// the batch or the share-file manifest). The returned function maps a HIS
// internal drug code → TMT code, returning ok=false when no active mapping
// exists. A nil factory, or a nil function it returns, means "no mapping
// available" and the extractor falls back to byte-for-byte legacy behaviour.
//
// Defining the seam as a factory (rather than importing store here) keeps the
// extractor package free of a store→pipeline→extractor import cycle: the
// store-backed implementation lives in the server/wiring layer.
type TMTResolverFactory func(ctx context.Context, hcode string) func(hisCode string) (tmtCode string, ok bool)

// resolverFor invokes the factory defensively: nil factory → nil resolver.
func resolverFor(f TMTResolverFactory, ctx context.Context, hcode string) func(string) (string, bool) {
	if f == nil || hcode == "" {
		return nil
	}
	return f(ctx, hcode)
}
