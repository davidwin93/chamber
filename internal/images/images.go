package images

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
)

func PullImage(img string) error {
	name, err := reference.ParseNamed("docker.io/library/alpine")
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

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{RemoveSignatures: true})
	if err != nil {
		fmt.Println("Error copying image:", err)
		return err
	}

	fmt.Println("Image successfully saved as tarball.")
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
