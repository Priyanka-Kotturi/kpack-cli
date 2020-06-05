package image

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pivotal/build-service-cli/pkg/image"
	"github.com/pivotal/build-service-cli/pkg/k8s"
)

func NewCreateCommand(clientSetProvider k8s.ClientSetProvider, factory *image.Factory, newImageWaiter func(k8s.ClientSet) ImageWaiter) *cobra.Command {
	var (
		namespace string
		wait      bool
	)

	cmd := &cobra.Command{
		Use:   "create <name> <tag>",
		Short: "Create an image configuration",
		Long: `Create an image configuration by providing command line arguments.
This image will be created only if it does not exist in the provided namespace.

namespace defaults to the kubernetes current-context namespace.

The flags for this command determine how the build will retrieve source code:

  "--git" and "--git-revision" to use Git based source
  "--blob" to use source code hosted in a blob store
  "--local-path" to use source code from the local machine

Local source code will be pushed to the same registry provided for the image tag.
Therefore, you must have credentials to access the registry on your machine.

Environment variables may be provided by using the "--env" flag.
For each environment variable, supply the "--env" flag followed by the key value pair.
For example, "--env key1=value1 --env key2=value2 ...".`,
		Example: `kp image create my-image my-registry.com/my-repo --git https://my-repo.com/my-app.git --git-revision my-branch
kp image create my-image my-registry.com/my-repo  --blob https://my-blob-host.com/my-blob
kp image create my-image my-registry.com/my-repo  --local-path /path/to/local/source/code
kp image create my-image my-registry.com/my-repo  --local-path /path/to/local/source/code --custom-builder my-builder -n my-namespace
kp image create my-image my-registry.com/my-repo  --blob https://my-blob-host.com/my-blob --env foo=bar --env color=red --env food=apple`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := clientSetProvider.GetClientSet(namespace)
			if err != nil {
				return err
			}

			img, err := factory.MakeImage(args[0], cs.Namespace, args[1])
			if err != nil {
				return err
			}

			originalImageCfg, err := json.Marshal(img)
			if err != nil {
				return err
			}

			if img.Annotations == nil {
				img.Annotations = map[string]string{}
			}
			img.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = string(originalImageCfg)

			img, err = cs.KpackClient.BuildV1alpha1().Images(cs.Namespace).Create(img)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "\"%s\" created\n", img.Name)
			if err != nil {
				return err
			}

			if wait {
				_, err := newImageWaiter(cs).Wait(cmd.Context(), cmd.OutOrStdout(), img)
				if err != nil {
					return err
				}

			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace")
	cmd.Flags().StringVar(&factory.GitRepo, "git", "", "git repository url")
	cmd.Flags().StringVar(&factory.GitRevision, "git-revision", "master", "git revision")
	cmd.Flags().StringVar(&factory.Blob, "blob", "", "source code blob url")
	cmd.Flags().StringVar(&factory.LocalPath, "local-path", "", "path to local source code")
	cmd.Flags().StringVar(&factory.SubPath, "sub-path", "", "build code at the sub path located within the source code directory")
	cmd.Flags().StringVarP(&factory.Builder, "custom-builder", "b", "", "custom builder name")
	cmd.Flags().StringVarP(&factory.ClusterBuilder, "custom-cluster-builder", "c", "", "custom cluster builder name")
	cmd.Flags().StringArrayVar(&factory.Env, "env", []string{}, "build time environment variables")

	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "wait for image create to be reconciled and tail resulting build logs")

	return cmd
}
