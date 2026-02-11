package ingest

import (
	"context"
	"graphdb/internal/embedding"
	"graphdb/internal/storage"
	"io/fs"
	"path/filepath"
)

type Walker struct {
	WorkerPool *WorkerPool
}

func NewWalker(workers int, embedder embedding.Embedder, emitter storage.Emitter) *Walker {
	return &Walker{
		WorkerPool: NewWorkerPool(workers, embedder, emitter),
	}
}

func (w *Walker) Run(ctx context.Context, dirPath string) error {
	w.WorkerPool.Start()
	defer w.WorkerPool.Stop()

	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.IsDir() {
			w.WorkerPool.Submit(path)
		}
		return nil
	})
}
