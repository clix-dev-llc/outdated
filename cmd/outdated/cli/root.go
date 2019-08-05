package cli

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/replicatedhq/outdated/pkg/logger"
	"github.com/replicatedhq/outdated/pkg/outdated"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tj/go-spin"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "outdated",
		Short:         "",
		Long:          `.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()
			log := logger.NewLogger()
			log.Info("")

			o := outdated.Outdated{}

			s := spin.New()
			finishedCh := make(chan bool, 1)
			foundImageName := make(chan string, 1)
			go func() {
				lastImageName := ""
				for {
					select {
					case <-finishedCh:
						fmt.Printf("\r")
						return
					case i := <-foundImageName:
						lastImageName = i
					case <-time.After(time.Millisecond * 100):
						if lastImageName == "" {
							fmt.Printf("\r  \033[36mSearching for images\033[m %s", s.Next())
						} else {
							fmt.Printf("\r  \033[36mSearching for images\033[m %s (%s)", s.Next(), lastImageName)
						}
					}
				}
			}()
			defer func() {
				finishedCh <- true
			}()

			images, err := o.ListImages(v.GetString("kubeconfig"), foundImageName)
			if err != nil {
				log.Error(err)
				log.Info("")
				os.Exit(1)
				return nil
			}
			finishedCh <- true

			head, imageColumnWidth, tagColumnWidth := headerLine(images)
			log.Header(head)

			for _, image := range images {
				log.StartImageLine(runningImage(image, imageColumnWidth, tagColumnWidth))
				checkResult, err := o.ParseImage(image.Image, image.PullableImage)
				if err != nil {
					log.Error(err)
					log.Info("")
					os.Exit(1)
					return nil
				}

				if checkResult.VersionsBehind != -1 {
					log.FinalizeImageLine(checkResult.VersionsBehind, completedImage(image, checkResult, imageColumnWidth, tagColumnWidth))
				} else {
					log.FinalizeImageLineWithError(erroredImage(image, checkResult, imageColumnWidth, tagColumnWidth))
				}
			}

			log.Info("")

			return nil
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.Flags().String("kubeconfig", path.Join(homeDir(), ".kube", "config"), "path to the kubeconfig to use")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("OUTDATED")
	viper.AutomaticEnv()
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}