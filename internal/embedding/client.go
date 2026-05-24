package embedding

import (
	"encoding/json"
	"fmt"
	"net"
)

const socketPath = "/tmp/eidolon-embed.sock"

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Embed(texts []string) ([][]float32, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial embedding server: %w", err)
	}
	defer conn.Close()

	req, _ := json.Marshal(map[string]interface{}{"texts": texts})
	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// signal end of write
	if tc, ok := conn.(*net.UnixConn); ok {
		tc.CloseWrite()
	}

	var buf []byte
	tmp := make([]byte, 65536)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}

	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("embedding server: %s", resp.Error)
	}
	return resp.Embeddings, nil
}

func (c *Client) EmbedOne(text string) ([]float32, error) {
	vecs, err := c.Embed([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vecs[0], nil
}
