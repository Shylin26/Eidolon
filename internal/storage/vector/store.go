package vector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

const dsn = "postgres://localhost:5432/eidolon?sslmode=disable"

type Chunk struct {
	ID        string
	FilePath  string
	ChunkIdx  int
	Content   string
	Language  string
	Embedding []float32
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Upsert(ctx context.Context, chunk Chunk) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO code_chunks (file_path, chunk_index, content, language_id, embedding)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (file_path, chunk_index)
		DO UPDATE SET content = EXCLUDED.content,
		              embedding = EXCLUDED.embedding,
		              indexed_at = NOW()
	`, chunk.FilePath, chunk.ChunkIdx, chunk.Content, chunk.Language,
		pgvector.NewVector(chunk.Embedding))
	return err
}

func (s *Store) Search(ctx context.Context, embedding []float32, limit int) ([]Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, chunk_index, content, language_id
		FROM code_chunks
		ORDER BY embedding <=> $1
		LIMIT $2
	`, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.FilePath, &c.ChunkIdx, &c.Content, &c.Language); err != nil {
			continue
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func (s *Store) Close() {
	s.pool.Close()
}
