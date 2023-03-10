/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package wrapper

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Options exposes the configuration options
// used when wrapping test execution
type Options struct {
	// Args is the process and args to run
	Args []string `json:"args,omitempty"`

	// ContainerName will contain the name of the container
	// for the wrapped test process
	ContainerName string `json:"container_name,omitempty"`

	// ProcessLog will contain std{out,err} from the
	// wrapped test process
	ProcessLog string `json:"process_log"`

	// MarkerFile will be written with the exit code
	// of the test process or an internal error code
	// if the entrypoint fails.
	MarkerFile string `json:"marker_file"`

	// MetadataFile is a file generated by the job,
	// and contains job metadata info like node image
	// versions for rendering in other tools like
	// testgrid/gubernator.
	// Prow will parse the file and merge it into
	// the `metadata` field in finished.json
	MetadataFile string `json:"metadata_file"`
}

type MarkerResult struct {
	ReturnCode int
	Err        error
}

// AddFlags adds flags to the FlagSet that populate
// the wrapper options struct provided.
func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.ProcessLog, "process-log", "", "path to the log where stdout and stderr are streamed for the process we execute")
	fs.StringVar(&o.MarkerFile, "marker-file", "", "file we write the return code of the process we execute once it has finished running")
	fs.StringVar(&o.MetadataFile, "metadata-file", "", "path to the metadata file generated from the job")
}

// Validate ensures that the set of options are
// self-consistent and valid
func (o *Options) Validate() error {
	if o.ProcessLog == "" {
		return errors.New("no log file specified with --process-log")
	}

	if o.MarkerFile == "" {
		return errors.New("no marker file specified with --marker-file")
	}

	return nil
}

func WaitForMarkers(ctx context.Context, paths ...string) map[string]MarkerResult {

	results := make(map[string]MarkerResult)

	if len(paths) == 0 {
		return results
	}

	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			results[path] = readMarkerFile(path)
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		populateMapWithError(results, fmt.Errorf("new fsnotify watch: %w", err), paths...)
		return results
	}
	defer watcher.Close()

	// we are assuming that all marker files will be written to the same directory.
	// this should be the case since all marker files are written to the "logs" VolumeMount
	dir := filepath.Dir(paths[0])
	for _, path := range paths {
		if filepath.Dir(path) != dir {
			populateMapWithError(results, fmt.Errorf("marker files are not all written to the same directory"), paths...)
			return results
		}
	}

	if err := watcher.Add(dir); err != nil {
		populateMapWithError(results, fmt.Errorf("add %s to fsnotify watch: %w", dir, err), paths...)
		return results
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for len(results) < len(paths) {
		select {
		case <-ctx.Done():
			populateMapWithError(results, fmt.Errorf("cancelled: %w", ctx.Err()), paths...)
			return results
		case event := <-watcher.Events:
			for _, path := range paths {
				if event.Name == path && event.Op&fsnotify.Create == fsnotify.Create {
					results[path] = readMarkerFile(path)
				}
			}
		case err := <-watcher.Errors:
			logrus.WithError(err).Warn("fsnotify watch error")
		case <-ticker.C:
			for _, path := range paths {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					results[path] = readMarkerFile(path)
				}
			}
		}
	}
	return results

}

func populateMapWithError(markerMap map[string]MarkerResult, err error, paths ...string) {
	for _, path := range paths {
		if _, exists := markerMap[path]; !exists {
			markerMap[path] = MarkerResult{-1, err}
		}
	}
}

func readMarkerFile(path string) MarkerResult {
	returnCodeData, err := os.ReadFile(path)
	if err != nil {
		return MarkerResult{-1, fmt.Errorf("bad read: %w", err)}
	}
	returnCode, err := strconv.Atoi(strings.TrimSpace(string(returnCodeData)))
	if err != nil {
		return MarkerResult{-1, fmt.Errorf("invalid return code: %w", err)}
	}
	return MarkerResult{returnCode, nil}
}
