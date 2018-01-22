package deploy

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/voormedia/kdeploy/pkg/config"
	"github.com/voormedia/kdeploy/pkg/kinflate"
	"github.com/voormedia/kdeploy/pkg/kubectl"
	"github.com/voormedia/kdeploy/pkg/util"
)

func Run(verbose bool, log *util.Logger, apps []*config.ResolvedApp, target *config.ResolvedTarget) error {
	for _, app := range apps {
		log.Note("Retrieving latest image for", app.Name)
		img, err := getLatestImage(app.Tag())
		if err != nil {
			return err
		}

		log.Note("Applying configuration")
		buf := &bytes.Buffer{}
		err = kinflate.Run(filepath.Join(app.Path, target.Path), buf)
		if err != nil {
			return err
		}

		/* HACK to set deployment image. */
		buf = bytes.NewBuffer(bytes.Replace(buf.Bytes(), []byte("image: "+app.Name), []byte("image: "+img), -1))

		/* HACK to remove empty annotations so that kubectl apply does not
		   incorrectly believe that a configuration has been made. */
		buf = bytes.NewBuffer(bytes.Replace(buf.Bytes(), []byte("annotations: {}\n"), []byte("\n"), -1))

		// os.Stdout.Write(buf.Bytes())
		err = kubectl.Apply(target.Context, target.Namespace, buf, os.Stdout, os.Stderr, &kubectl.ApplyOptions{})
		if err != nil {
			return err
		}

		log.Note("Tagging deployed image")
		err = tagImage(img, app.Tag()+":"+target.Name)
		if err != nil {
			return err
		}

		log.Success("Successfully deployed", app.Tag())
	}

	return nil
}

func getLatestImage(image string) (string, error) {
	cmd := exec.Command("gcloud", "container", "images", "describe",
		image+":latest",
		"--format=value(image_summary.fully_qualified_digest)",
	)

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", errors.Errorf("Failed to get latest image: %s", errOut.String())
	}

	return strings.TrimSpace(out.String()), nil
}

func tagImage(image string, tag string) error {
	cmd := exec.Command("gcloud", "container", "images", "add-tag",
		"--quiet",
		image, tag,
	)

	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return errors.Errorf("Failed to tag image: %s", errOut.String())
	}

	return nil
}
