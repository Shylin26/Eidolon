package inference

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestComplete(t *testing.T) {
	client := NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tokens, err := client.Complete(ctx, Request{
		RequestID:   "go-test-001",
		Prefix:      "func multiply(a int, b int) int {\n\treturn",
		Suffix:      "}",
		MaxTokens:   20,
		Temperature: 0.2,
		TopP:        0.95,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	completion := ""
	for tok := range tokens {
		if tok.Error != "" {
			t.Fatalf("inference error: %s", tok.Error)
		}
		fmt.Print(tok.Token)
		completion += tok.Token
	}
	fmt.Println()

	if completion == "" {
		t.Fatal("got empty completion")
	}
	t.Logf("✓ completion: %q", completion)
}
