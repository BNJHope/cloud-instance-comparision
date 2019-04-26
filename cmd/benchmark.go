package cmd

import (
	"errors"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"regexp"
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
}

func runBenchDeploy() {
	var (
		bestScore     float64
		err           error
		bestResult    *result
		cmd           *exec.Cmd
		endGroup      sync.WaitGroup
		instanceMutex = &sync.Mutex{}
		contextMutex  = &sync.Mutex{}
	)

	instancesToTry := getDefaultInstanceConfigsChan()
	resultsChan := make(chan *result, MaxContainerCreators+1)
	endGroup.Add(MaxContainerCreators)

	for i := 0; i < MaxContainerCreators; i++ {
		go startClusterCreator(i,
			instancesToTry,
			resultsChan,
			&endGroup,
			instanceMutex,
			contextMutex)
	}

	endGroup.Wait()
	log.Println("Tests finished")
	close(instancesToTry)

	for len(resultsChan) > 0 {
		res := <-resultsChan
		if res.score > bestScore {
			bestScore = res.score
			bestResult = res
		}
	}

	log.Println("Best score selected")
	close(resultsChan)

	log.Printf("Best score: %f\n", bestResult.score)
	log.Printf("Best cores: %d\n", bestResult.inst.cores)
	log.Printf("Best mem: %d\n", bestResult.inst.mem)
	log.Printf("Starting instance for image with best attributes.")

	machineType := bestResult.inst.constructCustomMachineType()

	if cmd, err = startCluster("test-cluster", machineType); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

	log.Println("starting pod")
	if cmd, err = createPod("test-pod", imageLink); err != nil {
		log.Fatal(err)
	}
	cmd.Wait()
}

func startClusterCreator(workerIndex int, instances chan (instance), resultChan chan (*result), endGroup *sync.WaitGroup, instanceMutex *sync.Mutex, contextMutex *sync.Mutex) {
	defer endGroup.Done()
	var (
		instanceToCheck instance
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
		score = runMicroBenchmark(workerIndex, instanceToCheck, contextMutex)
		log.Printf("Score %f from worker %d\n",
			score,
			workerIndex)
		if score > bestScore {
			bestScore = score
			bestResult = &result{score, instanceToCheck}
		}
	}
	resultChan <- bestResult
	log.Printf("Worker %d finished\n", workerIndex)
}

func runMicroBenchmark(workerIndex int, inst instance, contextMutex *sync.Mutex) float64 {
	var (
		cmd         *exec.Cmd
		err         error
		cpuAvg      float64
		memAvg      float64
		fullPodName string
		//context     string
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

	//contextMutex.Lock()
	//	if context, err = getContext(machineName); err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	if cmd, err = setContext(context); err != nil {
	//		log.Fatal(err)
	//	}
	//

	if cmd, err = createPod(podName, imageLink); err != nil {
		log.Fatal(err)
	}

	cmd.Wait()

	if fullPodName, err = getFullPodsName(workerIndex); err != nil {
		log.Fatal(err)
	}

	// contextMutex.Unlock()

	log.Printf("Worker %d getting metrics...\n", workerIndex)
	time.Sleep(3 * time.Minute)

	// contextMutex.Lock()
	//if cmd, err = setContext(context); err != nil {
	//	log.Fatal(err)
	//}

	if cpuAvg, memAvg, err = getTopAvg(fullPodName); err != nil {
		log.Fatal(err)
	}

	//contextMutex.Unlock()

	log.Printf("Worker %d fetched metrics!\n", workerIndex)
	log.Printf("Worker %d stopping cluster\n", workerIndex)
	if cmd, err = stopCluster(machineName); err != nil {
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
		cmd    *exec.Cmd
		err    error
		output []byte
	)
	cmd = exec.Command("kubectl",
		"top",
		"pod",
		podName)

	if output, err = cmd.Output(); err != nil {
		return 0, 0, err
	}

	return parseTopMetrics(output)
}

func calculateScore(cpuAvg float64, memAvg float64, price float64, inst instance) float64 {
	if cpuAvg == 0 {
		cpuAvg++
	}

	if memAvg == 0 {
		memAvg++
	}

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

func setContext(context string) (*exec.Cmd, error) {
	cmd := exec.Command("kubectl",
		"config",
		"use-context",
		context)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func getContext(clusterName string) (string, error) {
	var (
		context []byte
		err     error
	)

	if context, err = exec.Command("kubectl",
		"config",
		"get-contexts",
		"--no-headers").Output(); err != nil {
		return "", err
	}

	return parsePodContext(clusterName, context)
}

func getFullPodsName(workerIndex int) (string, error) {
	var (
		output []byte
		err    error
	)

	if output, err = exec.Command("kubectl",
		"get",
		"pods",
		"--no-headers",
		"--all-namespaces").Output(); err != nil {
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

	numberMatching := regexp.MustCompile(`[0-9|\.]+`)
	cpuStringWithoutUnit := string(numberMatching.Find([]byte(cpuString)))
	memStringWithoutUnit := string(numberMatching.Find([]byte(memString)))

	if cpu, err = strconv.ParseFloat(cpuStringWithoutUnit, 64); err != nil {
		return 0, 0, err
	}
	if mem, err = strconv.ParseFloat(memStringWithoutUnit, 64); err != nil {
		return 0, 0, err
	}
	return cpu, mem, nil
}

func parseFullPodName(workerIndex int, output []byte) (string, error) {
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, l := range lines[0 : len(lines)-1] {
		name := strings.Fields(l)[1]
		log.Println(name)
		log.Println(constructPodName(workerIndex))
		if strings.Contains(name, constructPodName(workerIndex)) {
			return name, nil
		}
	}

	return "", errors.New("pod not found")
}

func parsePodContext(clusterName string, output []byte) (string, error) {
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, l := range lines[0 : len(lines)-1] {
		name := strings.Fields(l)[2]
		log.Println(name)
		log.Println(clusterName)
		if strings.Contains(name, clusterName) {
			return name, nil
		}
	}

	return "", errors.New("pod not found")
}
