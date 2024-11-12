/**
 * Copyright 2024 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package smelter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	goyaml "github.com/go-yaml/yaml"
	"github.com/silogen/cluster-forge/cmd/utils"
	"gopkg.in/yaml.v2"
)

type k8sObject struct {
	Kind       string `yaml:"kind"`
	APIVersion string `yaml:"apiVersion"`
	Metadata   struct {
		Name        string            `yaml:"name"`
		Namespace   string            `yaml:"namespace,omitempty"` // omit empty namespace for cluster-scoped objects
		Labels      map[string]string `yaml:"labels,omitempty"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	} `yaml:"metadata"`
}

func splitYAML(resources []byte) ([][]byte, error) {

	dec := goyaml.NewDecoder(bytes.NewReader(resources))

	var res [][]byte
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// Skip empty documents
		if value == nil {
			continue
		}
		valueBytes, err := goyaml.Marshal(value)
		if err != nil {
			return nil, err
		}
		res = append(res, valueBytes)
	}
	return res, nil
}

func clean(input []byte) ([]byte, error) {
	var output bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(input))

	for scanner.Scan() {
		line := scanner.Text()

		// Skip lines containing '---', starting with '#', or containing helm data
		if strings.Contains(line, "---") || strings.HasPrefix(strings.TrimSpace(line), "#") ||
			strings.Contains(line, "helm.sh/chart") || strings.Contains(line, "app.kubernetes.io/managed-by") {
			continue
		}

		_, err := output.WriteString(line + "\n")
		if err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

// SplitYAML splits a YAML file into multiple documents.
func SplitYAML(config utils.Config) {
	data, err := os.ReadFile(config.Filename)
	if err != nil {
		log.Fatal(err)
	}

	// Call the SplitYAML function
	result, err := splitYAML(data)
	if err != nil {
		log.Fatal(err)
	}

	for _, res := range result {
		cleanres, err := clean(res)
		if err != nil {
			log.Fatal(err)
		}

		// Unmarshal the cleaned data into a map to check and update the namespace
		var objectMap map[string]interface{}
		err = yaml.Unmarshal(cleanres, &objectMap)
		if err != nil {
			log.Fatal(err)
		}
		var metadataObject k8sObject
		err = yaml.Unmarshal(cleanres, &metadataObject)
		if err != nil {
			log.Fatal(err)
		}
		if !utils.IsClusterScoped(metadataObject.Kind, metadataObject.APIVersion) {
			// Check and set the namespace if it's empty
			if metadataObject.Metadata.Namespace == "" {
				metadataObject.Metadata.Namespace = config.Namespace // Set your default namespace here
				objectMap["metadata"] = metadataObject.Metadata
			}

		}

		// Marshal the updated object back to YAML
		updatedCleanres, err := yaml.Marshal(&objectMap)
		if err != nil {
			log.Fatal(err)
		}

		// Use the kind and name to construct the output file name
		// create the directory if it doesn't exist
		err = os.MkdirAll(fmt.Sprintf("working/%s", config.Name), 0755)
		if err != nil {
			log.Fatal(err)
		}

		filename := fmt.Sprintf("working/%s/%s_%s.yaml", config.Name, metadataObject.Kind, metadataObject.Metadata.Name)
		// Write the updated cleaned data to the output file
		err = os.WriteFile(filename, updatedCleanres, 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
}