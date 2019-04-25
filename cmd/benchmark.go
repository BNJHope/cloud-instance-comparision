package cmd

import (
	"github.com/spf13/cobra"
	//	"golang.org/x/build/kubernetes"
	//	"golang.org/x/build/kubernetes/gke"
	"log"
	"os"
	"os/exec"
	"strings"
)

// macroBenchCmd represents the router command
var macroBenchCmd = &cobra.Command{
	Use:   "macro-bench",
	Short: "Start macro benchmarks",
	Long: `Starts the macro benchmarks, involving run the benchmark
	on the set of all instances to be benchmarked.`,
	Run: func(cmd *cobra.Command, args []string) {
		runMacroBenchmarks()
	},
}

const (
	gCloudCommand      = "gcloud"
	gcloudContainerArg = "container"
	gcloudClusterArg   = "clusters"
	machineTypeFlag    = "--machine-type"
	quietFlag          = "--quiet"
)

func init() {
	rootCmd.AddCommand(macroBenchCmd)
}

func benchmark(containerLink string, iterations int) {

	for i := 0; i < iterations; i++ {

	}

}

func runMacroBenchmarks() {
	var (
		cmd *exec.Cmd
		err error
	)

	if cmd, err = startCluster("test-cluster", "n1-standard-1"); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	//	if cmd, err = stopCluster("test-cluster"); err != nil {
	//		log.Fatal(err)
	//	}
	//	cmd.Wait()
}

func startCluster(clusterName string, machineType string) (*exec.Cmd, error) {
	cmd := exec.Command(gCloudCommand,
		gcloudContainerArg,
		gcloudClusterArg,
		"create",
		clusterName,
		getMachineTypeFlag(machineType))
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopCluster(clusterName string) (*exec.Cmd, error) {
	cmd := exec.Command(gCloudCommand,
		gcloudContainerArg,
		gcloudClusterArg,
		"delete",
		clusterName,
		quietFlag)
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func getMachineTypeFlag(machineType string) string {
	var sb strings.Builder
	sb.WriteString(machineTypeFlag)
	sb.WriteString("=")
	sb.WriteString(machineType)
	return sb.String()
}
