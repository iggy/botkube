package getter

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-getter/v2"
)

// Download downloads data from a given source to local file system under a given destination path.
func Download(ctx context.Context, src, dst string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("while getting current dir: %w", err)
	}

	// Build the client
	client := &getter.Client{
		Getters:       getter.Getters,
		Decompressors: getter.Decompressors,
	}
	_, err = client.Get(ctx, &getter.Request{
		Src:     src,
		Dst:     dst,
		Pwd:     pwd,
		GetMode: getter.ModeDir,
	})
	if err != nil {
		return fmt.Errorf("while downloading %q: %w", src, err)
	}
	return nil
}
