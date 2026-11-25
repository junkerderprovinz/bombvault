package dockercli

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/moby/moby/client"
)

// FilePeeker looks at a path the way a container sees it. Docker resolves the
// path in its own mount namespace, so a disk the host mounted after this
// container started shows up here even when it does not in this container.
type FilePeeker interface {
	HoldsFiles(ctx context.Context, containerID, path string) (bool, error)
}

var _ FilePeeker = (*Client)(nil)

// HoldsFiles reports whether path in the container has a file below it.
// It reads the archive only up to the first file, so a large folder costs
// little more than an empty one.
func (c *Client) HoldsFiles(ctx context.Context, containerID, path string) (bool, error) {
	res, err := c.api.CopyFromContainer(ctx, containerID, client.CopyFromContainerOptions{SourcePath: path})
	if err != nil {
		return false, fmt.Errorf("dockercli: read %s in %s: %w", path, containerID, err)
	}
	defer func() { _ = res.Content.Close() }()
	return tarHoldsFiles(res.Content)
}

func tarHoldsFiles(r io.Reader) (bool, error) {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("dockercli: read archive: %w", err)
		}
		if h.Typeflag != tar.TypeDir {
			return true, nil
		}
	}
}
