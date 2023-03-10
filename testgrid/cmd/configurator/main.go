/*
Copyright 2016 The Kubernetes Authors.

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

package main

import (
	"flag"
	"log"
	"os"

	"k8s.io/test-infra/testgrid/pkg/configurator/configurator"
	"k8s.io/test-infra/testgrid/pkg/configurator/options"
)

func main() {
	// Parse flags
	var opt options.Options
	if err := opt.GatherOptions(flag.CommandLine, os.Args[1:]); err != nil {
		log.Fatalf("Bad flags: %v", err)
	}

	if err := configurator.RealMain(&opt); err != nil {
		log.Fatal(err)
	}
}
