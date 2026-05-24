package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"github.com/eidolon/eidolon/internal/embedding"
	"github.com/eidolon/eidolon/internal/storage/vector"
)

var supportedExts = map[string]string{
	".go":  "go",
	".py":  "python",
	".ts":  "typescript",
	".tsx": "typescript",
	".js":  "javascript",
	".jsx": "javascript",
}

const chunkSize = 60 // lines per chunk

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	repoPath := "."
	if len(os.Args) > 1 {
		repoPath = os.Args[1]
	}

	ctx := context.Background()

	store, err := vector.NewStore(ctx)
	if err != nil {
		logger.Fatal("failed to connect to pgvector", zap.Error(err))
	}
	defer store.Close()

	embedClient := embedding.NewClient()

	total := 0
	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" || name == "models" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		lang, ok := supportedExts[ext]
		if !ok {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		chunks := chunkLines(lines, chunkSize)

		for i, chunk := range chunks {
			if strings.TrimSpace(chunk) == "" {
				continue
			}

			vec, err := embedClient.EmbedOne(chunk)
			if err != nil {
				logger.Warn("embed failed", zap.String("file", path), zap.Error(err))
				continue
			}

			if err := store.Upsert(ctx, vector.Chunk{
				FilePath:  path,
				ChunkIdx:  i,
				Content:   chunk,
				Language:  lang,
				Embedding: vec,
			}); err != nil {
				logger.Warn("upsert failed", zap.String("file", path), zap.Error(err))
				continue
			}
			total++
		}

		fmt.Printf("indexed %s (%d chunks)\n", path, len(chunks))
		return nil
	})

	if err != nil {
		logger.Fatal("walk failed", zap.Error(err))
	}

	logger.Info("indexing complete", zap.Int("total_chunks", total))
}

func chunkLines(lines []string, size int) []string {
	var chunks []string
	for i := 0; i < len(lines); i += size {
		end := i + size
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, strings.Join(lines[i:end], "\n"))
	}
	return chunks
}
