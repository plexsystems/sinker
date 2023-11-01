package commands

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/containers/image/v5/copy"
	dockerv5 "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCopyCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "copy",
		Short: "Copy the images in the manifest directly from source to target repository",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			flags := []string{"dryrun", "images", "target", "force", "override-arch", "override-os", "all-variants", "jobs"}
			for _, flag := range flags {
				if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
					return fmt.Errorf("bind flag: %w", err)
				}
			}

			if len(viper.GetStringSlice("images")) > 0 && viper.GetString("target") == "" {
				return errors.New("target must be specified when using the images flag")
			}

			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCopyCommand(); err != nil {
				return fmt.Errorf("copy: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dryrun", false, "Print a list of images that would be copied to the target")
	cmd.Flags().StringSliceP("images", "i", []string{}, "List of images to copy to target")
	cmd.Flags().StringP("target", "t", "", "Registry the images will be copied to")
	cmd.Flags().Bool("force", false, "Force the copy of the image even if already exists at the target")
	cmd.Flags().StringP("override-arch", "a", "", "Architecture variant of the image if it is a multi-arch image")
	cmd.Flags().StringP("override-os", "o", "", "Operating system variant of the image if it is a multi-os image")
	cmd.Flags().Bool("all-variants", false, "Copy all variants of the image")
	cmd.Flags().IntP("jobs", "j", 1, "Allow N jobs at once; if 0, unlimited. Only applied to remote calls.")

	return &cmd
}

func runCopyCommand() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Use Docker client for queries that do not require access to docker socket.
	client, err := docker.New(log.Infof)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	var sources []manifest.Source
	if len(viper.GetStringSlice("images")) > 0 {
		sources = manifest.GetSourcesFromImages(viper.GetStringSlice("images"), viper.GetString("target"))
	} else {
		imageManifest, err := manifest.Get(viper.GetString("manifest"))
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}

		sources = imageManifest.Sources
	}

	log.Infof("Finding images that need to be copied ...")

	errs, errCtx := errgroup.WithContext(ctx)
	errs.SetLimit(viper.GetInt("jobs"))
	var mu sync.Mutex
	var sourcesToCopy []manifest.Source
	for _, source := range sources {
		source := source
		errs.Go(func() error {
			exists, err := client.ImageExistsAtRemote(errCtx, source.TargetImage())
			if err != nil {
				return fmt.Errorf("image exists at remote: %w", err)
			}

			if !exists || viper.GetBool("force") {
				mu.Lock()
				sourcesToCopy = append(sourcesToCopy, source)
				mu.Unlock()
			}

			return nil
		})
	}

	if err := errs.Wait(); err != nil {
		return err
	}

	if len(sourcesToCopy) == 0 {
		log.Infof("All images are up to date!")
		return nil
	}

	if viper.GetBool("dryrun") {
		for _, source := range sourcesToCopy {
			log.Infof("Image %s would be copied to %s", source.Image(), source.TargetImage())
		}

		return nil
	}

	// Create a default image policy accepting unsigned images.
	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("new policy context: %w", err)
	}

	var copyOptions copy.Options
	if viper.GetBool("all-variants") {
		copyOptions.ImageListSelection = copy.CopyAllImages
	} else {
		copyOptions.ImageListSelection = copy.CopySystemImage
	}

	if viper.GetString("override-os") != "" || viper.GetString("override-arch") != "" {
		copyOptions.SourceCtx = &types.SystemContext{
			ArchitectureChoice: viper.GetString("override-arch"),
			OSChoice:           viper.GetString("override-os"),
		}
	}

	imageTransport := dockerv5.Transport
	for _, source := range sourcesToCopy {
		log.Infof("Copying image %s to %s", source.Image(), source.TargetImage())
		destRef, err := imageTransport.ParseReference(fmt.Sprintf("//%s", source.TargetImage()))
		if err != nil {
			return fmt.Errorf("unable to parse target image reference: %w", err)
		}

		srcRef, err := imageTransport.ParseReference(fmt.Sprintf("//%s", source.Image()))
		if err != nil {
			return fmt.Errorf("unable to parse source image reference: %w", err)
		}

		if _, err := copy.Image(ctx, policyContext, destRef, srcRef, &copyOptions); err != nil {
			return fmt.Errorf("copy image: %w", err)
		}
	}

	log.Infof("All images have been copied!")
	return nil
}
