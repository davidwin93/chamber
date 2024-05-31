package images

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"git.jakstys.lt/motiejus/undocker/rootfs"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
)

func PullImage(img string) error {
	name, err := reference.ParseNamed("docker.io/library/nginx")
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
	command, err := getCommandArgs(tarImage)
	if err != nil {
		log.Println(err)
	}
	log.Println(command)
	cmd := commandJSON{Command: command}
	tmpFile, err := os.CreateTemp("/tmp", "command-*.json")
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	err = json.NewEncoder(tmpFile).Encode(cmd)
	if err != nil {
		return err
	}

	tarImage.Seek(0, io.SeekStart)
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
	if _, err = tarFlatImage.Seek(-1024, io.SeekEnd); err != nil {
		return err
	}
	tw := tar.NewWriter(tarFlatImage)
	file, err := os.Open("./init-runner")
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}
	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = "/init-runner"

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}
	tmpFile.Seek(0, io.SeekStart)
	// Get FileInfo about our file providing file size, mode, etc.
	tmpInfo, err := tmpFile.Stat()
	if err != nil {
		return err
	}
	// Create a tar Header from the FileInfo data
	tmpHeader, err := tar.FileInfoHeader(tmpInfo, "command.json")
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	tmpHeader.Name = "/command.json"

	// Write file header to the tar archive
	err = tw.WriteHeader(tmpHeader)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, tmpFile)
	if err != nil {
		return err
	}
	tw.Close()
	tarFlatImage.Seek(0, io.SeekStart)

	tarFlatImage.Seek(0, io.SeekStart)
	tarFlatImage.Close()
	_, err = createExt4(tarFlatImage)
	fmt.Println("Image successfully saved as tarball.")
	_, _ = io.Copy(io.Discard, tarFlatImage)
	return err
}

type commandJSON struct {
	Command []string `json:"command"`
}

func createExt4(input *os.File) (string, error) {
	err := exec.Command("fallocate", "-l", "2G", "./output.ext4").Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("mkfs.ext4", "./output.ext4").Run()
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("/tmp", "image-mnt")
	if err != nil {
		return "", err
	}
	err = exec.Command("mount", "-o", "loop", "./output.ext4", tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("tar", "-xf", input.Name(), "-C", tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("umount", tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = os.RemoveAll(tmpDir)
	return "./output.ext4", err
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

func getCommandArgs(data io.ReadSeeker) ([]string, error) {
	tr := findFile("manifest.json", data)
	output := []string{}
	var config []map[string]interface{}
	err := json.NewDecoder(tr).Decode(&config)
	if err != nil {
		log.Println(err)
		return output, err
	}
	log.Println(config[0]["Config"])
	data.Seek(0, io.SeekStart)
	tr = findFile(config[0]["Config"].(string), data)
	var cmdData map[string]any
	err = json.NewDecoder(tr).Decode(&cmdData)
	if err != nil {
		log.Println(err)
		return output, err
	}
	configExtract := cmdData["config"]
	log.Println(configExtract)
	if configExtract == nil {
		return output, errors.New("config not found")
	}
	cmdExtract := configExtract.(map[string]any)["Cmd"]
	if cmdExtract == nil {
		return output, errors.New("cmd not found")
	}
	entryPoint := configExtract.(map[string]any)["Entrypoint"]
	log.Println(configExtract.(map[string]any)["User"])
	if entryPoint != nil {
		return append(convertAnyToString(entryPoint.([]any)), convertAnyToString(cmdExtract.([]any))...), nil
	}
	output = convertAnyToString(cmdExtract.([]any))
	return output, nil
}
func findFile(name string, data io.Reader) io.Reader {
	tr := tar.NewReader(data)

	// get the next file entry
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}

		if h.Name == name {
			return tr
		}

	}
	return nil
}
func convertAnyToString(data []any) []string {
	output := []string{}
	for _, v := range data {
		output = append(output, v.(string))
	}
	return output
}
