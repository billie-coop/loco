package analysis

import "context"

// Progress represents progress information for analysis phases.
type Progress struct {
	Phase          string
	TotalFiles     int
	CompletedFiles int
	CurrentFile    string
}

// progressCallbackKey is the context key for the progress callback.
type progressCallbackKey struct{}

// ProgressCallback is a function that receives progress updates.
type ProgressCallback func(Progress)

// WithProgressCallback stores a progress callback in the context.
func WithProgressCallback(ctx context.Context, cb ProgressCallback) context.Context {
	if ctx == nil || cb == nil {
		return ctx
	}
	return context.WithValue(ctx, progressCallbackKey{}, cb)
}

// ReportProgress invokes the progress callback in the context if present.
func ReportProgress(ctx context.Context, p Progress) {
	if ctx == nil {
		return
	}
	if v := ctx.Value(progressCallbackKey{}); v != nil {
		if cb, ok := v.(ProgressCallback); ok && cb != nil {
			cb(p)
		}
	}
}
