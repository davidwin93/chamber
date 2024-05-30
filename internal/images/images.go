package images

import (
	"context"
	"fmt"
	"io"
	"os"

	"git.jakstys.lt/motiejus/undocker/rootfs"
	"github.com/Microsoft/hcsshim/ext4/tar2ext4"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
)

func PullImage(img string) error {
	name, err := reference.ParseNamed("docker.io/davidwin/alpine")
	if err != nil {
		fmt.Println("Error creating policy context:", err)
		return err
	}
	tags, err := getImageTags(context.Background(), &types.SystemContext{OSChoice: "linux"}, name)
	if err != nil {
		fmt.Println("Error creating policy context:", err)
		return err
	}

	ctx := context.Background()

	policyContext, err := signature.NewPolicyContext(&signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}})
	if err != nil {
		fmt.Println("Error creating policy context:", err)
		return err
	}
	defer policyContext.Destroy()
	taggedRef, err := reference.WithTag(name, tags[0])
	if err != nil {
		fmt.Println("Error creating policy context:", err)
		return err
	}
	srcRef, err := docker.NewReference(taggedRef)
	if err != nil {
		fmt.Println("Error parsing source reference:", err)
		return err
	}

	destRef, err := alltransports.ParseImageName("docker-archive:./output.tar:alpine:latest")
	if err != nil {
		fmt.Println("Error parsing destination reference:", err)
		return err
	}

	os.Remove("./output.tar")

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{RemoveSignatures: true})
	if err != nil {
		fmt.Println("Error copying image:", err)
		return err
	}
	tarImage, err := os.Open("./output.tar")
	if err != nil {
		return err
	}
	tarFlatImage, err := os.Create("./output-flat.tar")
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer tarFlatImage.Close()
	defer tarImage.Close()

	err = rootfs.Flatten(tarImage, tarFlatImage)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	ext4Output, err := os.Create("./output.ext4")
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer ext4Output.Close()
	var opts []tar2ext4.Option
	tarFlatImage.Seek(0, io.SeekStart)
	err = tar2ext4.Convert(tarFlatImage, ext4Output, opts...)
	fmt.Println("Image successfully saved as tarball.")
	_, _ = io.Copy(io.Discard, tarFlatImage)
	return err
}
func getImageTags(ctx context.Context, sysCtx *types.SystemContext, repoRef reference.Named) ([]string, error) {
	name := repoRef.Name()

	// Ugly: NewReference rejects IsNameOnly references, and GetRepositoryTags ignores the tag/digest.
	// So, we use TagNameOnly here only to shut up NewReference
	dockerRef, err := docker.NewReference(reference.TagNameOnly(repoRef))
	if err != nil {
		return nil, err // Should never happen for a reference with tag and no digest
	}
	tags, err := docker.GetRepositoryTags(ctx, sysCtx, dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Error determining repository tags for repo %s: %w", name, err)
	}

	return tags, nil
}
