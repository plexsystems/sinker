package commands

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/genuinetools/reg/registry"
	"github.com/spf13/cobra"
)

func newCheckCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the remote registry",

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := runCheckCommand(ctx, "."); err != nil {
				return fmt.Errorf("run check: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runCheckCommand(ctx context.Context, path string) error {
	dockerAuth, err := newRegistryAuth("https://index.docker.io")
	if err != nil {
		return fmt.Errorf("new registry auth: %w", err)
	}

	dockerAuthConfig := types.AuthConfig{
		Username: dockerAuth.Username,
		Password: dockerAuth.Password,
	}

	dockerOpts := registry.Opt{
		Insecure: true,
		Domain:   "https://index.docker.io",
	}

	dockerRegistry, err := registry.New(ctx, dockerAuthConfig, dockerOpts)
	if err != nil {
		return fmt.Errorf("new registry: %w", err)
	}

	dockerTags, err := dockerRegistry.Tags(ctx, "library/nginx")
	if err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}

	fmt.Println(dockerTags)

	return nil
}
