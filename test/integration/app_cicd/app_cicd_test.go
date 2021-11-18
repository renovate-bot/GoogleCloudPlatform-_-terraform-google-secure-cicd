/**
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// define test package name
package app_cicd

import (
	"fmt"
	"testing"

	// import the blueprints test framework modules for testing and assertions
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/gcloud"
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/tft"
	"github.com/stretchr/testify/assert"
)

// name the function as Test*
func TestAppCICDExample(t *testing.T) {
	// define constants for all required assertions in the test case

	const sourceTriggerName    = "app-source-trigger"
	const garRepoNameSuffix    = "app-image-repo"
	const primaryLocation      = "us-central1"
	const appSourceRepoName    = "app-source"
	const manifestDryRepoName  = "app-dry-manifests"
	const manifestWetRepoName  = "app-wet-manifests"

	// initialize Terraform test from the Blueprints test framework
	appCICDT := tft.NewTFBlueprintTest(t)

	// define and write a custom verifier for this test case call the default verify for confirming no additional changes
	appCICDT.DefineVerify(func(assert *assert.Assertions) {
		// perform default verification ensuring Terraform reports no additional changes on an applied blueprint
		appCICDT.DefaultVerify(assert)

		// invoke the gcloud module in the Blueprints test framework to run a gcloud command that will output resource properties in a JSON format
		// the tft struct can be used to pull output variables of the TF module being invoked by this test and use the op object (a gjson struct)
		// to parse through the JSON results and assert the values of the resource against the constants defined above

		projectID := appCICDT.GetStringOutput("project_id")

		/////// SECURE-CI ///////
		// Cloud Build Trigger - App Source
		gcbCI := gcloud.Run(t, fmt.Sprintf("beta builds triggers describe %s --project %s", sourceTriggerName, projectID))

		assert.Equal(sourceTriggerName, gcbCI.Get("name").String(), "Cloud Build Trigger name is valid")
		assert.Equal(manifestDryRepoName, gcbCI.Get("substitutions._MANIFEST_DRY_REPO").String(), "Manifest Dry Repo trigger substitution is valid")
		assert.Equal(manifestWetRepoName, gcbCI.Get("substitutions._MANIFEST_WET_REPO").String(), "Manifest Wet Repo trigger substitution is valid")
		assert.Contains(gcbCI.Get("substitutions._DEFAULT_REGION").String(), primaryLocation, "Default Region trigger substitution is valid")
		assert.Equal(appSourceRepoName, gcbCI.Get("triggerTemplate.repoName").String(), "Attached CSR repo is valid")

		// Artifact Registry repository
		gar := gcloud.Run(t, fmt.Sprintf("artifacts repositories describe %s-%s --project %s --location %s", projectID, garRepoNameSuffix, projectID, primaryLocation))
		garFullname := "projects/" + projectID + "/locations/" + primaryLocation + "/repositories/" + projectID + "-" + garRepoNameSuffix
		assert.Equal(garFullname, gar.Get("name").String(), "GAR Repo is valid")

		// BinAuthz Attestors
		attestors := [3]string{"build-attestor", "quality-attestor", "security-attestor"}

		for _, attestor := range attestors {
			binAuthZAttestor := gcloud.Run(t, fmt.Sprintf("container binauthz attestors describe %s --project %s", attestor, projectID))
			attestorFullName := "projects/" + projectID + "/attestors/" + attestor
			assert.Equal(attestorFullName, binAuthZAttestor.Get("name").String(), attestor + " is valid")
		}

		// CSR
		repos := [3]string{appSourceRepoName, manifestDryRepoName, manifestWetRepoName}

		for _, repo := range repos {
			csr := gcloud.Run(t, fmt.Sprintf("source repos describe %s --project %s", repo, projectID))
			csrFullName := "projects/" + projectID + "/repos/" + repo
			csrURL := "https://source.developers.google.com/p/" + projectID + "/r/" + repo
			assert.Equal(csrFullName, csr.Get("name").String(), "CSR " + repo + " repo name is valid")
			assert.Equal(csrURL, csr.Get("url").String(), "CSR " + repo + " URL is valid")
		}

		/////// SECURE-CD ///////
		// Deploy Triggers
		cdTriggers := [3]string{"deploy-trigger-dev", "deploy-trigger-qa", "deploy-trigger-prod"}

		for _, cdTrigger := range cdTriggers {
			gcbCD := gcloud.Run(t, fmt.Sprintf("beta builds triggers describe %s --project %s", cdTrigger, projectID))
			assert.Equal(cdTrigger, gcbCD.Get("name").String(), "Trigger name is valid")
			assert.Equal(manifestWetRepoName, gcbCD.Get("triggerTemplate.repoName").String(), "repoName triggerTemplate is valid")
			assert.Equal(projectID, gcbCD.Get("triggerTemplate.projectId").String(), "Trigger is in correct project")
			assert.Equal(manifestWetRepoName, gcbCD.Get("substitutions._MANIFEST_WET_REPO").String(), "_MANIFEST_WET_REPO trigger substitution is valid")
			assert.Contains(gcbCD.Get("substitutions._CLUSTER_PROJECT").String(), "secure-cicd-gke-", "_CLUSTER_PROJECT trigger substitution is valid")
		}

		// BinAuthz Policy


	})
	// call the test function to execute the integration test
	appCICDT.Test()
}
