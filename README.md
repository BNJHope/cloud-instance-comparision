# cloud-instance-comparision
Compares different GCP cluster configurations and finds the best one for a given image by benchmarking the image on the clusters automatically.

## Build and Run
Local Google Cloud Platform account needs to be setup.
The project needs [dep](https://github.com/golang/dep) for dependency management. Run
```bash
dep ensure
```
to install dependencies.
To compare different GCP clusters that would be best suited for a given image, for example to find the best cluster for an Nginx server, and start a container of the image using the best cluster foumd, run
```bash
go run main.go bench-deploy --image=nginx
```

## Future Changes to be Made
* Parallel benchmarking - work has been done on this but needs to be debugged.
* Better metrics - statistics are not normalised, which puts more significance on certain metrics when evaluating the benchmark.
* Read configurations to compare for external source - will allow user to more easily adjust the configurations to compare.
* Add more metrics to be configured.
