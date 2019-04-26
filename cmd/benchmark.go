package cmd

import (
	"errors"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// macroBenchCmd represents the router command
var macroBenchCmd = &cobra.Command{
	Use:   "bench-deploy",
	Short: "Benchmark and deploy the given image",
	Long: `Test the given image on containers with
	different attributes and deploy the image to the best performing one.`,
	Run: func(cmd *cobra.Command, args []string) {
		runBenchDeploy()
	},
}

// testInstanceCmd test command for deploying an instance
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
		memAvg      float64
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
	if fullPodName, err = getFullPodsName(0); err != nil {
		log.Fatal(err)
	}

	log.Println(fullPodName)
	log.Println("waiting 3 mins to get CPU avg")
	log.Println("getting cpu avg")

	if cpuAvg, memAvg, err = getTopAvg(fullPodName); err != nil {
		log.Fatal(err)
	}

	log.Println(cpuAvg)
	log.Println(memAvg)
}

func runBenchDeploy() {
	var (
		bestScore     float64
		err           error
		bestResult    *result
		cmd           *exec.Cmd
		endGroup      sync.WaitGroup
		instanceMutex = &sync.Mutex{}
	)

	log.Println("Starting benchdeploy")
	instancesToTry := getDefaultInstanceConfigsChan()
	log.Println("Instances fetched")
	resultsChan := make(chan *result, MaxContainerCreators+1)
	endGroup.Add(MaxContainerCreators)

	log.Println("Resources created")
	for i := 0; i < MaxContainerCreators; i++ {
		go startClusterCreator(i,
			instancesToTry,
			resultsChan,
			&endGroup,
			instanceMutex)
	}

	log.Println("workers created")

	endGroup.Wait()
	log.Println("endgroup finished")
	close(instancesToTry)

	for res := range resultsChan {
		if res.score > bestScore {
			bestResult = res
		}
	}

	log.Println("best score selected")
	close(resultsChan)

	log.Printf("Best cores: %d\n", bestResult.inst.cores)
	log.Printf("Best mem: %d\n", bestResult.inst.mem)
	log.Printf("Starting instance for image with best containers.")

	machineType := bestResult.inst.constructCustomMachineType()

	if cmd, err = startCluster("test-cluster", machineType); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	log.Println("starting pod selected")
	if cmd, err = createPod("test-pod", imageLink); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()
}

func startClusterCreator(workerIndex int, instances chan (*instance), resultChan chan (*result), endGroup *sync.WaitGroup, instanceMutex *sync.Mutex) {
	defer endGroup.Done()
	var (
		instanceToCheck *instance
		score           float64
		bestScore       float64
		bestResult      *result
	)

	for len(instances) > 0 {
		instanceMutex.Lock()
		instanceToCheck = <-instances
		instanceMutex.Unlock()
		log.Printf("Worker %d checking instance %d %d\n",
			workerIndex,
			instanceToCheck.cores,
			instanceToCheck.mem)
		score = runMicroBenchmark(workerIndex, instanceToCheck)
		log.Printf("Score %f from worker %d\n",
			score,
			workerIndex)
		if score > bestScore {
			bestResult = &result{score, instanceToCheck}
		}
	}
	resultChan <- bestResult
	log.Printf("Worker %d finished\n", workerIndex)
}

func runMicroBenchmark(workerIndex int, inst *instance) float64 {
	var (
		cmd         *exec.Cmd
		err         error
		cpuAvg      float64
		memAvg      float64
		fullPodName string
	)

	machineType := inst.constructCustomMachineType()
	machineName := constructMachineName(workerIndex)
	podName := constructPodName(workerIndex)

	log.Printf("Worker %d starting cluster\n", workerIndex)
	if cmd, err = startCluster(machineName, machineType); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	log.Printf("Worker %d starting pod\n", workerIndex)
	if cmd, err = createPod(podName, imageLink); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	if fullPodName, err = getFullPodsName(workerIndex); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	log.Printf("Worker %d sleeping\n", workerIndex)
	time.Sleep(3 * time.Minute)

	log.Printf("Worker %d get avg\n", workerIndex)
	if cpuAvg, memAvg, err = getTopAvg(fullPodName); err != nil {
		log.Fatal(err)
	}

	log.Printf("Worker %d stopping cluster\n", workerIndex)
	if cmd, err = stopCluster("test-cluster"); err != nil {
		log.Fatal(err)
	}

	cmd.Wait()

	return calculateScore(cpuAvg, memAvg, 30, inst)
}

func startCluster(clusterName string, machineType string) (*exec.Cmd, error) {
	cmd := exec.Command(gCloudCommand,
		gcloudContainerArg,
		gcloudClusterArg,
		"create",
		clusterName,
		getMachineTypeFlag(machineType),
		"--num-nodes=1")
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

func constructMachineName(workerIndex int) string {
	var sb strings.Builder
	sb.WriteString("cluster")
	sb.WriteString(strconv.Itoa(workerIndex))
	return sb.String()
}

func constructPodName(workerIndex int) string {
	var sb strings.Builder
	sb.WriteString("pod")
	sb.WriteString(strconv.Itoa(workerIndex))
	return sb.String()
}

func getTopAvg(podName string) (float64, float64, error) {
	var (
		output []byte
		err    error
	)

	if output, err = exec.Command("kubectl",
		"top",
		"pod",
		podName).Output(); err != nil {
		return 0, 0, err
	}

	return parseTopMetrics(output)
}

func calculateScore(cpuAvg float64, memAvg float64, price float64, inst *instance) float64 {
	return cpuAvg * memAvg * float64(inst.cores) * float64(inst.mem) / price
}

func createPod(podName string, imageName string) (*exec.Cmd, error) {
	cmd := exec.Command("kubectl",
		"create",
		"deployment",
		podName,
		getImageFlag(imageName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil

}

func getFullPodsName(workerIndex int) (string, error) {
	var (
		output []byte
		err    error
	)

	if output, err = exec.Command("kubectl",
		"get",
		"pods").Output(); err != nil {
		return "", err
	}

	return parseFullPodName(workerIndex, output)
}

func getImageFlag(imageName string) string {
	var sb strings.Builder
	sb.WriteString("--image=")
	sb.WriteString(imageName)
	return sb.String()
}

func parseTopMetrics(output []byte) (float64, float64, error) {
	var (
		cpu float64
		mem float64
		err error
	)
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	avgline := lines[1]
	cpuString := strings.Fields(avgline)[1]
	memString := strings.Fields(avgline)[2]
	cpuStringWithoutUnit := strings.TrimSuffix(cpuString, "m")
	memStringWithoutUnit := strings.TrimSuffix(memString, "%")
	if cpu, err = strconv.ParseFloat(cpuStringWithoutUnit, 64); err != nil {
		return 0, 0, err
	}
	if mem, err = strconv.ParseFloat(memStringWithoutUnit, 64); err != nil {
		return 0, 0, err
	}
	return cpu, mem, nil
}

func parseFullPodName(workerIndex int, output []byte) (string, error) {
	var (
		podName string
	)
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	log.Printf("Printing output for worker %d\n", workerIndex)
	log.Println(outputStr)
	for _, l := range lines[1:len(lines)] {
		name := strings.Fields(l)[0]
		if strings.Contains(name, constructPodName(workerIndex)) {
			return podName, nil
		}
	}

	return "", errors.New("pod not found")
}
