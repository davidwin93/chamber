package images

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"git.jakstys.lt/motiejus/undocker/rootfs"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
	"github.com/google/uuid"
)

func PullImage(img string, extraEnv []string) (string, error) {
	imageID := uuid.NewString()
	var name reference.Named
	var err error
	if strings.Contains(img, "/") {
		name, err = reference.ParseNamed(fmt.Sprintf("docker.io/%s", img))
	} else {
		name, err = reference.ParseNamed(fmt.Sprintf("docker.io/library/%s", img))
	}
	if err != nil {
		return "", err
	}
	tags, err := getImageTags(context.Background(), &types.SystemContext{OSChoice: "linux"}, name)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	policyContext, err := signature.NewPolicyContext(&signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}})
	if err != nil {
		return "", err
	}
	defer policyContext.Destroy()
	taggedRef, err := reference.WithTag(name, tags[0])
	if err != nil {
		return "", err
	}
	srcRef, err := docker.NewReference(taggedRef)
	if err != nil {
		return "", err
	}

	destRef, err := alltransports.ParseImageName(fmt.Sprintf("docker-archive:./%s.tar:img:latest", imageID))
	if err != nil {
		return "", err
	}

	os.Remove(fmt.Sprintf("./%s.tar", imageID))

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{RemoveSignatures: true})
	if err != nil {
		return "", err
	}

	cmd, err := getCommandArgs(fmt.Sprintf("./%s.tar", imageID))
	if err != nil {
		log.Println(err)
	}
	log.Println(cmd)
	if extraEnv != nil {
		cmd.Env = append(cmd.Env, extraEnv...)
	}
	log.Println("Flattening image")
	tarFlatImage, err := flattenDockerTar(fmt.Sprintf("./%s.tar", imageID), fmt.Sprintf("./%s-flat.tar", imageID))
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer tarFlatImage.Close()

	tarFlatImage.Seek(0, io.SeekStart)
	log.Println("Injecting binary")
	err = injectBinary(tarFlatImage, *cmd, "./init-runner")
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	extRootOutput := fmt.Sprintf("./root-micro-%s.ext4", imageID)
	ext4Output, err := os.Create(extRootOutput)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer ext4Output.Close()

	tarFlatImage.Close()
	_, err = createExt4(tarFlatImage.Name(), extRootOutput)
	if err != nil {
		return "", err
	}
	fmt.Println("Image successfully saved as tarball.")
	return extRootOutput, err
}

type commandJSON struct {
	Command []string `json:"command"`
	Env     []string `json:"env"`
}

func createOverlay(sourcePathExt4 string, overlayPath string, imageID string) (string, error) {
	loopBaseDev, err := exec.Command("losetup", "--find", "--show", "--read-only", sourcePathExt4).Output()
	if err != nil {
		return "", err
	}
	loopBaseDevStr := string(bytes.TrimSpace(loopBaseDev))
	log.Println(loopBaseDevStr)
	err = exec.Command("fallocate", "-l", "5G", overlayPath).Run()
	if err != nil {
		return "", err
	}
	overlayRawDev, err := exec.Command("losetup", "--find", "--show", overlayPath).Output()
	if err != nil {
		return "", err
	}
	overlayRawDevStr := string(bytes.TrimSpace(overlayRawDev))
	log.Println(overlayRawDevStr)
	baseSizeRaw, err := exec.Command("blockdev", "--getsz", loopBaseDevStr).Output()
	if err != nil {
		return "", err
	}
	overlaySizeRaw, err := exec.Command("blockdev", "--getsz", overlayRawDevStr).Output()
	if err != nil {
		return "", err
	}
	baseSize := string(bytes.TrimSpace(baseSizeRaw))
	overlaySize := string(bytes.TrimSpace(overlaySizeRaw))
	log.Println(baseSize)
	log.Println(overlaySize)
	dmFile, err := os.CreateTemp("/tmp", "dmsetup")
	if err != nil {
		return "", err
	}
	_, err = dmFile.WriteString(fmt.Sprintf(`0 %s linear %s 0
%s %s zero`, baseSize, loopBaseDevStr, baseSize, overlaySize))
	if err != nil {
		return "", err
	}
	dmFile.Close()
	output, err := exec.Command("dmsetup", "create", fmt.Sprintf("base-%s", imageID), dmFile.Name()).CombinedOutput()
	log.Println(string(output))
	if err != nil {
		return "", err
	}

	overlayStdin, err := os.CreateTemp("/tmp", "dmsetup-overlay")
	if err != nil {
		return "", err
	}
	overlayStdin.Write([]byte(fmt.Sprintf(`0 %s snapshot /dev/mapper/%s %s P 8`, overlaySize, fmt.Sprintf("base-%s", imageID), overlayRawDevStr)))
	overlayDM := exec.Command("dmsetup", "create", fmt.Sprintf("overlay-%s", imageID), overlayStdin.Name())
	output, err = overlayDM.CombinedOutput()
	log.Println(string(output))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/dev/mapper/overlay-%s", imageID), nil
}
func flattenDockerTar(dockerTarFile, outputFlatFile string) (*os.File, error) {

	tarImage, err := os.Open(dockerTarFile)
	if err != nil {
		return nil, err
	}

	tarFlatImage, err := os.Create(outputFlatFile)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}

	defer tarImage.Close()

	err = rootfs.Flatten(tarImage, tarFlatImage)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	return tarFlatImage, nil
}
func injectBinary(tarFlatImage *os.File, cmd commandJSON, binaryPath string) error {
	tmpFile, err := os.CreateTemp("/tmp", "command-*.json")
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	err = json.NewEncoder(tmpFile).Encode(cmd)
	if err != nil {
		return err
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
	return nil
}
func createExt4(inputPath string, outputFile string) (string, error) {
	err := exec.Command("fallocate", "-l", "2G", outputFile).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("mkfs.ext4", outputFile).Run()
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("/tmp", "image-mnt")
	if err != nil {
		return "", err
	}
	err = exec.Command("mount", "-o", "loop", outputFile, tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("tar", "-xf", inputPath, "-C", tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("umount", tmpDir).Run()
	if err != nil {
		return "", err
	}
	err = os.RemoveAll(tmpDir)
	return outputFile, err
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

func getCommandArgs(dockerTar string) (*commandJSON, error) {
	data, err := os.Open(dockerTar)
	if err != nil {
		return nil, err
	}
	tr := findFile("manifest.json", data)
	output := []string{}
	var config []map[string]interface{}
	err = json.NewDecoder(tr).Decode(&config)
	if err != nil {
		log.Println(err)
		return &commandJSON{Command: output}, err
	}
	log.Println(config[0]["Config"])
	data.Seek(0, io.SeekStart)
	tr = findFile(config[0]["Config"].(string), data)
	var cmdData map[string]any
	err = json.NewDecoder(tr).Decode(&cmdData)
	if err != nil {
		log.Println(err)
		return &commandJSON{Command: output}, err
	}
	configExtract := cmdData["config"]
	log.Println(configExtract)
	if configExtract == nil {
		return &commandJSON{Command: output}, errors.New("config not found")
	}
	cmdExtract := configExtract.(map[string]any)["Cmd"]
	if cmdExtract == nil {
		return &commandJSON{Command: output}, errors.New("cmd not found")
	}
	envExtract := configExtract.(map[string]any)["Env"]
	if envExtract == nil {
		envExtract = []any{}
	}
	entryPoint := configExtract.(map[string]any)["Entrypoint"]
	log.Println(configExtract.(map[string]any)["User"])
	if entryPoint != nil {
		return &commandJSON{Env: convertAnyToString(envExtract.([]any)), Command: append(convertAnyToString(entryPoint.([]any)), convertAnyToString(cmdExtract.([]any))...)}, nil
	}
	output = convertAnyToString(cmdExtract.([]any))
	return &commandJSON{Env: convertAnyToString(envExtract.([]any)), Command: output}, nil
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
