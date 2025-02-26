package github

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// SimpleEmbeddingProvider is a basic implementation of an embedding provider
// In a real implementation, this would use a proper embedding model like OpenAI's
type SimpleEmbeddingProvider struct {
	// This is a placeholder for a real embedding provider
}

// NewSimpleEmbeddingProvider creates a new simple embedding provider
func NewSimpleEmbeddingProvider() *SimpleEmbeddingProvider {
	return &SimpleEmbeddingProvider{}
}

// GenerateEmbedding generates a simple embedding for the given text
// This is a very naive implementation and should be replaced with a real embedding model
func (p *SimpleEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// This is a very naive implementation that just counts word frequencies
	// In a real implementation, this would use a proper embedding model

	// Normalize the text
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Split into words
	words := strings.Fields(text)

	// Count word frequencies
	wordCounts := make(map[string]int)
	for _, word := range words {
		wordCounts[word]++
	}

	// Create a simple embedding based on word frequencies
	// This is just a placeholder for a real embedding
	embedding := make([]float32, 100)

	// Fill the embedding with some values based on the word counts
	// This is not a real embedding, just a placeholder
	for word, count := range wordCounts {
		// Use the hash of the word to determine which dimension to update
		hash := 0
		for _, c := range word {
			hash = (hash*31 + int(c)) % len(embedding)
		}

		// Update the embedding
		embedding[hash] += float32(count)
	}

	// Normalize the embedding
	var sum float32
	for _, v := range embedding {
		sum += v * v
	}

	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// InMemoryVectorStore is a simple in-memory vector store
type InMemoryVectorStore struct {
	mu      sync.RWMutex
	vectors map[string]vector
}

type vector struct {
	ID        string
	Content   string
	Embedding []float32
	Metadata  map[string]interface{}
}

// NewInMemoryVectorStore creates a new in-memory vector store
func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		vectors: make(map[string]vector),
	}
}

// Add adds a vector to the store
func (s *InMemoryVectorStore) Add(ctx context.Context, id string, content string, embedding []float32, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vectors[id] = vector{
		ID:        id,
		Content:   content,
		Embedding: embedding,
		Metadata:  metadata,
	}

	return nil
}

// AddWithContent adds a vector to the store with the given content and embedding
func (s *InMemoryVectorStore) AddWithContent(ctx context.Context, id string, content string, embedding []float32) error {
	return s.Add(ctx, id, content, embedding, nil)
}

// Search searches for vectors similar to the given embedding
func (s *InMemoryVectorStore) Search(ctx context.Context, embedding []float32, limit int) ([]RepoContent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.vectors) == 0 {
		return nil, fmt.Errorf("vector store is empty")
	}

	// Calculate similarity scores
	type scoreItem struct {
		ID    string
		Score float32
	}

	scores := make([]scoreItem, 0, len(s.vectors))

	for id, vec := range s.vectors {
		score := cosineSimilarity(embedding, vec.Embedding)
		scores = append(scores, scoreItem{
			ID:    id,
			Score: score,
		})
	}

	// Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Limit results
	if limit > 0 && limit < len(scores) {
		scores = scores[:limit]
	}

	// Convert to search results
	results := make([]RepoContent, len(scores))
	for i, score := range scores {
		vec := s.vectors[score.ID]
		results[i] = RepoContent{
			Path:     vec.ID,
			Type:     "search_result",
			Content:  vec.Content,
			URL:      "",
			Metadata: vec.Metadata,
		}
	}

	return results, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
