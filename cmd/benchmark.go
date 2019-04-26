package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

// macroBenchCmd represents the router command
var testInstanceCmd = &cobra.Command{
	Use:   "test-instance",
	Short: "Start test instance",
	Long: `Starts the macro benchmarks, involving run the benchmark
	on the set of all instances to be benchmarked.`,
	Run: func(cmd *cobra.Command, args []string) {
		testInstance()
	},
}

var (
	imageLink       string
	numOfIterations int
)

const (
	gCloudCommand      = "gcloud"
	gcloudContainerArg = "container"
	gcloudClusterArg   = "clusters"
	machineTypeFlag    = "--machine-type"
	quietFlag          = "--quiet"
	defaultMem         = 12288
)

func init() {
	rootCmd.AddCommand(macroBenchCmd)
	macroBenchCmd.Flags().StringVarP(&imageLink, "image", "i", "", "The image to deploy")
	macroBenchCmd.Flags().IntVarP(&numOfIterations, "iterations", "r", 3, "Number of iterations to carry out")

	rootCmd.AddCommand(testInstanceCmd)
	testInstanceCmd.Flags().StringVarP(&imageLink, "image", "i", "", "The image to deploy")
}

func testInstance() {
	var (
		err error
		//cmd         *exec.Cmd
		fullPodName string
		cpuAvg      float64
	)

	//	machineType := constructCustomMachineType(2)
	//
	//	if cmd, err = startCluster("test-cluster", machineType); err != nil {
	//		log.Fatal(err)
	//	}
	//	cmd.Wait()

	//	if cmd, err = createPod("test-pod", imageLink); err != nil {
	//		log.Fatal(err)
	//	}
	//	cmd.Wait()
	//
	if fullPodName, err = getFullPodsName(); err != nil {
		log.Fatal(err)
	}

	log.Println(fullPodName)
	log.Println("waiting 3 mins to get CPU avg")
	log.Println("getting cpu avg")

	if cpuAvg, err = getCpuAvg(fullPodName); err != nil {
		log.Fatal(err)
	}

	log.Println(cpuAvg)
}

func runMacroBenchmarks() {
	var (
		score     float64
		bestScore float64 = 0
		bestCores int     = 0
		err       error
		cmd       *exec.Cmd
	)

	for i := 0; i < numOfIterations; i++ {
		score = runMicroBenchmark(4)
		if score > bestScore {
			bestScore = score
			bestCores = 4
		}
	}

	log.Printf("Best cores: %d\n", bestCores)
	log.Printf("Starting instance for image with best containers.")

	machineType := constructCustomMachineType(bestCores)

	if cmd, err = startCluster("test-cluster", machineType); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	if cmd, err = createPod("test-pod", imageLink); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()
}

func runMicroBenchmark(cores int) float64 {
	var (
		cmd         *exec.Cmd
		err         error
		cpuAvg      float64
		fullPodName string
	)

	machineType := constructCustomMachineType(cores)

	if cmd, err = startCluster("test-cluster", machineType); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	if cmd, err = createPod("test-pod", imageLink); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	if fullPodName, err = getFullPodsName(); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	time.Sleep(5 * time.Minute)

	if cpuAvg, err = getCpuAvg(fullPodName); err != nil {
		log.Fatal(err)
	}

	if cmd, err = stopCluster("test-cluster"); err != nil {
		log.Fatal(err)
	}

	cmd.Wait()

	return calculateScore(cpuAvg, 30, cores)
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

func constructCustomMachineType(cores int) string {
	var sb strings.Builder
	sb.WriteString("custom-")
	sb.WriteString(strconv.Itoa(cores))
	sb.WriteString("-")
	sb.WriteString(strconv.Itoa(defaultMem))
	return sb.String()
}

func getCpuAvg(podName string) (float64, error) {
	var (
		output []byte
		err    error
	)

	if output, err = exec.Command("kubectl",
		"top",
		"pod",
		podName).Output(); err != nil {
		return 0, err
	}
	return parseCpuAvg(output)
}

func calculateScore(cpuAvg float64, price float64, numOfCores int) float64 {
	return cpuAvg * float64(numOfCores) / price
}

func createPod(podName string, imageName string) (*exec.Cmd, error) {
	cmd := exec.Command("kubectl",
		"create",
		"deployment",
		podName,
		getImageFlag(imageName))
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil

}

func getFullPodsName() (string, error) {
	var (
		output []byte
		err    error
	)

	if output, err = exec.Command("kubectl",
		"get",
		"pods").Output(); err != nil {
		return "", err
	}

	return parseFullPodName(output)
}

func getImageFlag(imageName string) string {
	var sb strings.Builder
	sb.WriteString("--image=")
	sb.WriteString(imageName)
	return sb.String()
}

func parseCpuAvg(output []byte) (float64, error) {
	var (
		avg float64
		err error
	)
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	avgline := lines[1]
	avgString := strings.Fields(avgline)[1]
	avgStringWithoutUnit := strings.TrimSuffix(avgString, "m")
	if avg, err = strconv.ParseFloat(avgStringWithoutUnit, 64); err != nil {
		return 0, err
	}
	return avg, nil
}

func parseFullPodName(output []byte) (string, error) {
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	nameline := lines[1]
	podName := strings.Fields(nameline)[0]
	return podName, nil
}
