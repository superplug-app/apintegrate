package main

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type HubApis struct {
	Apis []HubApi `json:"apis"`
}

type HubApi struct {
	Name          string               `json:"name"`
	DisplayName   string               `json:"displayName"`
	Description   string               `json:"description"`
	Documentation *HubApiDocumentation `json:"documentation,omitempty"`
	Owner         *HubApiOwner         `json:"owner,omitempty"`
	Versions      *[]string            `json:"versions,omitempty"`
}

type HubApiDocumentation struct {
	ExternalUri string `json:"externalUri"`
}

type HubApiOwner struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type HubApiDeployments struct {
	Deployments []HubApiDeployment `json:"deployments"`
}

type HubApiDeployment struct {
	Name           string              `json:"name"`
	DisplayName    string              `json:"displayName"`
	Description    string              `json:"description"`
	Documentation  HubApiDocumentation `json:"documentation"`
	DeploymentType HubAttribute        `json:"deploymentType"`
	ResourceUri    string              `json:"resourceUri"`
	Endpoints      []string            `json:"endpoints"`
	ApiVersions    []string            `json:"apiVersions"`
}

type HubApiVersions struct {
	Versions []HubApiVersion `json:"versions"`
}

type HubApiVersion struct {
	Name          string              `json:"name"`
	DisplayName   string              `json:"displayName"`
	Description   string              `json:"description"`
	Documentation HubApiDocumentation `json:"documentation"`
	Deployments   []string            `json:"deployments"`
}

type HubAttribute struct {
	Attribute  string                 `json:"attribute"`
	EnumValues HubAttributeEnumValues `json:"enumValues"`
}

type HubAttributeEnumValues struct {
	Values []HubAttributeValue `json:"values"`
}

type HubAttributeValue struct {
	Id          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Immutable   bool   `json:"immutable"`
}

type HubApiVersionSpecs struct {
	Specs []HubApiVersionSpec `json:"specs"`
}

type HubApiVersionSpec struct {
	Name          string              `json:"name"`
	DisplayName   string              `json:"displayName"`
	SpecType      HubAttribute        `json:"specType"`
	Contents      HubContents         `json:"contents"`
	Documentation HubApiDocumentation `json:"documentation"`
}

type HubContents struct {
	MimeType string `json:"mimeType"`
	Contents string `json:"contents"`
}

func apiHubStatus(flags *ApigeeFlags) PlatformStatus {
	var status PlatformStatus
	if flags.Project == "" {
		status.Connected = false
		status.Message = "No project given, cannot connect to API Hub. Please specify a --project YOUR_PROJECT_ID flag."
		return status
	} else if flags.Region == "" {
		status.Connected = false
		status.Message = "No region given, cannot connect to API Hub. Please specify a --region YOUR_REGION flag."
		return status
	}

	if flags.Token == "" {
		var token *oauth2.Token
		scopes := []string{
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, scopes...)

		if err == nil {
			token, err = credentials.TokenSource.Token()

			if err == nil {
				flags.Token = token.AccessToken
			}
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+flags.Project+"/locations/"+flags.Region+"/apis", nil)
	req.Header.Add("Authorization", "Bearer "+flags.Token)

	var apis HubApis
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &apis)
		}

		if resp.StatusCode == 200 {
			status.Connected = true
			status.Message = "Connected to API Hub, " + strconv.Itoa(len(apis.Apis)) + " APIs found in project " + flags.Project + " and region " + flags.Region + "."
		} else {
			status.Connected = false
			status.Message = resp.Status
		}
	} else {
		status.Connected = false
		status.Message = err.Error()
	}

	return status
}

func apiHubOnramp(flags *ApigeeFlags) error {
	generalBaseDir := "src/main/general/apiproxies"
	baseDir := "src/main/apihub/apiproxies"

	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	} else if flags.Region == "" {
		fmt.Println("No region given. Please specify a --region YOUR_REGION flag.")
		return nil
	}

	if flags.Token == "" {
		var token *oauth2.Token
		scopes := []string{
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, scopes...)

		if err == nil {
			token, err = credentials.TokenSource.Token()

			if err == nil {
				flags.Token = token.AccessToken
			}
		}
	}

	entries, err := os.ReadDir(generalBaseDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		if flags.ApiName == "" || flags.ApiName == e.Name() {
			fmt.Println(e.Name())

			var generalApi GeneralApi
			apiFile, err := os.Open(generalBaseDir + "/" + e.Name() + "/" + e.Name() + ".json")
			if err != nil {
				log.Fatal(err)
				return err
			} else {
				byteValue, _ := io.ReadAll(apiFile)
				json.Unmarshal(byteValue, &generalApi)
			}
			defer apiFile.Close()

			if generalApi.Name != "" {
				os.MkdirAll(baseDir+"/"+e.Name(), 0755)

				var apiName = e.Name()

				// create API
				var hubApi HubApi
				hubApi.Name = "projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + apiName
				hubApi.DisplayName = generalApi.DisplayName
				hubApi.Description = generalApi.Description
				if generalApi.DocumentationUrl != "" {
					var doc HubApiDocumentation
					doc.ExternalUri = generalApi.DocumentationUrl
					hubApi.Documentation = &doc
				}

				if generalApi.OwnerName != "" {
					var owner HubApiOwner
					owner.DisplayName = generalApi.OwnerName
					owner.Email = generalApi.OwnerEmail
					hubApi.Owner = &owner
				}

				bytes, _ := json.MarshalIndent(hubApi, "", "  ")
				os.WriteFile(baseDir+"/"+apiName+"/"+apiName+".json", bytes, 0644)

				var apiVersions map[string][]HubApiDeployment = make(map[string][]HubApiDeployment)

				// read all files
				fileEntries, _ := os.ReadDir(generalBaseDir + "/" + e.Name())
				for _, f := range fileEntries {
					if strings.HasSuffix(f.Name(), "-aws.json") || strings.HasSuffix(f.Name(), "-azure.json") {
						fmt.Println(f.Name())
						apiVersionName := strings.ReplaceAll(f.Name(), "-aws.json", "")
						apiVersionName = strings.ReplaceAll(apiVersionName, "-azure.json", "")

						// create deployment
						var generalDeploymentApi GeneralApi
						apiFile, err := os.Open(generalBaseDir + "/" + apiName + "/" + f.Name())
						if err != nil {
							log.Fatal(err)
							return err
						} else {
							byteValue, _ := io.ReadAll(apiFile)
							json.Unmarshal(byteValue, &generalDeploymentApi)
						}
						defer apiFile.Close()

						if generalDeploymentApi.Name != "" {
							fmt.Println(generalDeploymentApi.Name)

							// create deployment
							var hubApiDeployment HubApiDeployment
							hubApiDeployment.Name = "projects/" + flags.Project + "/locations/" + flags.Region + "/deployments/" + generalDeploymentApi.Name
							hubApiDeployment.DisplayName = generalDeploymentApi.DisplayName
							hubApiDeployment.Description = generalDeploymentApi.Description
							hubApiDeployment.Documentation.ExternalUri = generalDeploymentApi.DocumentationUrl
							hubApiDeployment.DeploymentType.Attribute = "projects/" + flags.Project + "/locations/" + flags.Region + "/attributes/system-deployment-type"
							apiDeploymentType := HubAttributeValue{Id: generalDeploymentApi.PlatformId, DisplayName: generalDeploymentApi.PlatformName, Description: generalDeploymentApi.PlatformName, Immutable: true}
							hubApiDeployment.DeploymentType.EnumValues.Values = append(hubApiDeployment.DeploymentType.EnumValues.Values, apiDeploymentType)
							hubApiDeployment.ResourceUri = generalDeploymentApi.PlatformResourceUri
							hubApiDeployment.Endpoints = append(hubApiDeployment.Endpoints, generalDeploymentApi.GatewayUrl)
							hubApiDeployment.ApiVersions = append(hubApiDeployment.ApiVersions, generalDeploymentApi.Version)
							bytes, _ := json.MarshalIndent(hubApiDeployment, "", "  ")
							os.WriteFile(baseDir+"/"+apiName+"/"+generalDeploymentApi.Name+".json", bytes, 0644)

							// record deployment for version
							_, ok := apiVersions[apiVersionName]
							if ok {
								apiVersions[apiVersionName] = append(apiVersions[apiVersionName], hubApiDeployment)
							} else {
								apiVersions[apiVersionName] = []HubApiDeployment{hubApiDeployment}
							}

							// create API spec, if available
							b, err := os.ReadFile(generalBaseDir + "/" + apiName + "/" + generalDeploymentApi.Name + "-oas.json")
							if err == nil {
								// we have a spec file
								var hubApiVersionSpec HubApiVersionSpec
								hubApiVersionSpec.Name = "projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + apiName + "/versions/" + apiVersionName + "/specs/" + generalDeploymentApi.Name
								hubApiVersionSpec.DisplayName = generalDeploymentApi.DisplayName + " (" + generalDeploymentApi.PlatformName + ")"
								apiSpecType := HubAttributeValue{Id: "openapi", DisplayName: "OpenAPI Spec", Description: "OpenAPI Spec", Immutable: true}
								hubApiVersionSpec.SpecType.EnumValues.Values = append(hubApiVersionSpec.SpecType.EnumValues.Values, apiSpecType)
								hubApiVersionSpec.Contents.MimeType = "application/json"
								hubApiVersionSpec.Contents.Contents = b64.StdEncoding.EncodeToString(b)
								hubApiVersionSpec.Documentation.ExternalUri = generalApi.DocumentationUrl
								bytes, _ = json.MarshalIndent(hubApiVersionSpec, "", "  ")
								os.WriteFile(baseDir+"/"+apiName+"/"+generalDeploymentApi.Name+"-oas.json", bytes, 0644)
							}
						}
					}
				}

				for k, v := range apiVersions {
					// create API version
					var hubApiVersion HubApiVersion
					hubApiVersion.Name = "projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + apiName + "/versions/" + k
					hubApiVersion.DisplayName = v[0].DisplayName
					hubApiVersion.Description = generalApi.Description
					hubApiVersion.Documentation.ExternalUri = generalApi.DocumentationUrl

					for _, d := range v {
						hubApiVersion.Deployments = append(hubApiVersion.Deployments, d.Name)
					}

					bytes, _ := json.MarshalIndent(hubApiVersion, "", "  ")
					os.WriteFile(baseDir+"/"+apiName+"/"+k+".json", bytes, 0644)
				}
			}
		}
	}

	return nil
}

func apiHubImport(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	} else if flags.Region == "" {
		fmt.Println("No region given. Please specify a --region YOUR_REGION flag.")
		return nil
	}

	fmt.Println("Importing APIs to API Hub in project " + flags.Project + "...")
	var baseDir = "src/main/apihub/apiproxies"
	if flags.Token == "" {
		var token *oauth2.Token
		scopes := []string{
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, scopes...)

		if err == nil {
			token, err = credentials.TokenSource.Token()

			if err == nil {
				flags.Token = token.AccessToken
			}
		}
	}

	apis, err := os.ReadDir(baseDir)
	if err == nil {
		for _, e := range apis {
			if flags.ApiName == "" || flags.ApiName == e.Name() {
				fmt.Println("Importing " + e.Name() + "...")
				// Create API
				apiFile, err := os.Open(baseDir + "/" + e.Name() + "/" + e.Name() + ".json")
				if err == nil {
					var hubApi HubApi
					byteValue, _ := io.ReadAll(apiFile)
					json.Unmarshal(byteValue, &hubApi)

					requestBody := bytes.NewBuffer(byteValue)
					r, _ := http.NewRequest(http.MethodPost, "https://apihub.googleapis.com/v1/projects/"+flags.Project+"/locations/"+flags.Region+"/apis?apiId="+e.Name(), requestBody)
					r.Header.Add("Content-Type", "application/json")
					r.Header.Add("Authorization", "Bearer "+flags.Token)
					client := &http.Client{}
					fmt.Println("Creating API " + e.Name() + "...")
					resp, _ := client.Do(r)

					if resp.StatusCode != 200 {
						fmt.Println("  >> Error creating " + e.Name() + ": " + resp.Status)
						//Read the response body
						respBody, _ := io.ReadAll(resp.Body)
						sb := string(respBody)
						fmt.Println(sb)
						defer resp.Body.Close()
					}
				} else {
					fmt.Println("  >> Error, cloud not create API in API Hub because the definition file could not be found: " + e.Name() + ".json")
				}
				defer apiFile.Close()

				var apiVersions map[string][]string = make(map[string][]string)
				// read all files
				fileEntries, _ := os.ReadDir(baseDir + "/" + e.Name())
				for _, f := range fileEntries {
					if strings.HasSuffix(f.Name(), "-aws.json") || strings.HasSuffix(f.Name(), "-azure.json") {
						apiDeploymentName := strings.ReplaceAll(f.Name(), ".json", "")
						apiVersionName := strings.ReplaceAll(f.Name(), "-aws.json", "")
						apiVersionName = strings.ReplaceAll(apiVersionName, "-azure.json", "")

						// Create Deployment
						deploymentFile, deployErr := os.Open(baseDir + "/" + e.Name() + "/" + f.Name())
						if deployErr == nil {
							var apiDeployment HubApiDeployment
							byteValue, _ := io.ReadAll(deploymentFile)
							json.Unmarshal(byteValue, &apiDeployment)
							requestBody := bytes.NewBuffer(byteValue)
							deploymentUrl := "https://apihub.googleapis.com/v1/projects/" + flags.Project + "/locations/" + flags.Region + "/deployments?deploymentId=" + apiDeploymentName
							r, _ := http.NewRequest(http.MethodPost, deploymentUrl, requestBody)
							r.Header.Add("Content-Type", "application/json")
							r.Header.Add("Authorization", "Bearer "+flags.Token)
							client := &http.Client{}
							fmt.Println("Creating deployment " + apiDeploymentName + "..." + deploymentUrl)
							resp, _ := client.Do(r)

							if resp.StatusCode != 200 {
								fmt.Println("  >> Error creating deployment " + apiDeploymentName + ": " + resp.Status)
								//Read the response body
								respBody, _ := io.ReadAll(resp.Body)
								sb := string(respBody)
								fmt.Println(sb)
								defer resp.Body.Close()
							}
						}
						defer deploymentFile.Close()

						// record deployment for version
						_, ok := apiVersions[apiVersionName]
						if ok {
							apiVersions[apiVersionName] = append(apiVersions[apiVersionName], apiDeploymentName)
						} else {
							apiVersions[apiVersionName] = []string{apiDeploymentName}
						}
					}
				}

				for k, v := range apiVersions {
					// create API version
					versionFile, err := os.Open(baseDir + "/" + e.Name() + "/" + k + ".json")
					if err == nil {
						var apiVersion HubApiVersion
						byteValue, _ := io.ReadAll(versionFile)
						json.Unmarshal(byteValue, &apiVersion)
						bodyBytes, _ := json.Marshal(apiVersion)
						requestBody := bytes.NewBuffer(bodyBytes)

						versionUrl := "https://apihub.googleapis.com/v1/projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + e.Name() + "/versions?versionId=" + k
						r, _ := http.NewRequest(http.MethodPost, versionUrl, requestBody)
						r.Header.Add("Content-Type", "application/json")
						r.Header.Add("Authorization", "Bearer "+flags.Token)
						client := &http.Client{}
						fmt.Println("Creating API version " + k + "...")
						resp, _ := client.Do(r)

						if resp.StatusCode != 200 {
							fmt.Println("  >> Error creating version " + e.Name() + ": " + resp.Status)
							defer resp.Body.Close()
							//Read the response body
							respBody, _ := io.ReadAll(resp.Body)
							sb := string(respBody)
							fmt.Println(sb)

							// update if it already exists, maybe we have a new version deployment...
							if resp.StatusCode == 409 {
								requestBody = bytes.NewBuffer(bodyBytes)
								versionUrl = "https://apihub.googleapis.com/v1/projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + e.Name() + "/versions/" + k + "?updateMask=deployments"
								r, _ := http.NewRequest(http.MethodPatch, versionUrl, requestBody)
								r.Header.Add("Content-Type", "application/json")
								r.Header.Add("Authorization", "Bearer "+flags.Token)
								client := &http.Client{}
								fmt.Println("Patching API version " + k + "...")
								resp, _ := client.Do(r)
								if resp.StatusCode != 200 {
									fmt.Println("  >> Error patching version " + k + ": " + resp.Status)
									defer resp.Body.Close()
									//Read the response body
									respBody, _ := io.ReadAll(resp.Body)
									sb := string(respBody)
									fmt.Println(sb)
								}
							}
						}
					}
					defer versionFile.Close()

					for _, d := range v {
						// Create API Version Spec
						versionSpecFile, err := os.Open(baseDir + "/" + e.Name() + "/" + d + "-oas.json")
						if err == nil {
							var apiVersionSpec HubApiVersionSpec
							byteValue, _ := io.ReadAll(versionSpecFile)
							json.Unmarshal(byteValue, &apiVersionSpec)
							requestBody := bytes.NewBuffer(byteValue)

							versionUrl := "https://apihub.googleapis.com/v1/projects/" + flags.Project + "/locations/" + flags.Region + "/apis/" + e.Name() + "/versions/" + k + "/specs?specId=" + d
							r, _ := http.NewRequest(http.MethodPost, versionUrl, requestBody)
							r.Header.Add("Content-Type", "application/json")
							r.Header.Add("Authorization", "Bearer "+flags.Token)
							client := &http.Client{}
							fmt.Println("Creating API version spec " + e.Name() + "...")
							resp, _ := client.Do(r)

							if resp.StatusCode != 200 {
								fmt.Println("  >> Error deploying version spec " + e.Name() + ": " + resp.Status)
								defer resp.Body.Close()
								//Read the response body
								// respBody, _ := io.ReadAll(resp.Body)
								// sb := string(respBody)
								// fmt.Println(sb)
							}
						}
						defer versionSpecFile.Close()
					}
				}
			}
		}
	}

	return nil
}

func apiHubExport(flags *ApigeeFlags) error {
	baseDir := "src/main/apihub/apiproxies"

	if flags.Project == "" && flags.Region == "" {
		fmt.Println("Missing ' --project YOUR_PROJECT_ID --region YOUR_REGION'")
		return nil
	} else if flags.Project == "" {
		fmt.Println("Missing ' --project YOUR_PROJECT_ID'")
		return nil
	} else if flags.Region == "" {
		fmt.Println("Missing ' --region YOUR_REGION'")
		return nil
	}

	fmt.Println("Exporting all API Hub APIs for project " + flags.Project + "...")

	if flags.Token == "" {
		var token *oauth2.Token
		scopes := []string{
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, scopes...)

		if err == nil {
			token, err = credentials.TokenSource.Token()

			if err == nil {
				flags.Token = token.AccessToken
			}
		}
	}

	apis := getApiHubApis(flags.Project, flags.Region, flags.Token)
	for _, api := range apis.Apis {
		if flags.ApiName == "" || strings.HasSuffix(api.Name, "/"+flags.ApiName) {
			s := strings.Split(api.Name, "/")
			apiName := s[len(s)-1]
			fmt.Println("Exporting " + apiName + "...")

			bytes, _ := json.MarshalIndent(api, "", "  ")
			os.MkdirAll(baseDir+"/"+apiName, 0755)
			os.WriteFile(baseDir+"/"+apiName+"/"+apiName+".json", bytes, 0644)

			versions := getApiHubApiVersions(flags.Project, flags.Region, apiName, flags.Token)
			for _, version := range versions.Versions {
				s := strings.Split(version.Name, "/")
				versionName := s[len(s)-1]
				fmt.Println("Exporting " + api.Name + " Version " + versionName + "...")

				bytes, _ := json.MarshalIndent(version, "", "  ")
				os.WriteFile(baseDir+"/"+apiName+"/"+versionName+".json", bytes, 0644)

				// get version specs
				specs := getApiHubApiVersionSpecs(flags.Project, flags.Region, apiName, versionName, flags.Token)
				for _, spec := range specs.Specs {
					s := strings.Split(spec.Name, "/")
					specName := s[len(s)-1]
					fmt.Println("Exporting " + api.Name + " Spec " + specName + "...")

					spec.Contents = getApiHubApiVersionSpecContents(flags.Project, flags.Region, apiName, versionName, specName, flags.Token)
					bytes, _ := json.MarshalIndent(spec, "", "  ")
					os.WriteFile(baseDir+"/"+apiName+"/"+specName+"-oas.json", bytes, 0644)
				}
			}
		}
	}

	deployments := getApiHubDeployments(flags.Project, flags.Region, flags.Token)
	for _, deployment := range deployments.Deployments {
		s := strings.Split(deployment.Name, "/")
		deploymentName := s[len(s)-1]
		fmt.Println("Exporting deployment " + deploymentName + "...")

		apiVersionName := strings.ReplaceAll(deploymentName, "-aws", "")
		apiVersionName = strings.ReplaceAll(apiVersionName, "-azure", "")

		var re = regexp.MustCompile(`(-v\d+)$`)
		apiName := re.ReplaceAllString(apiVersionName, "")
		os.MkdirAll(baseDir+"/"+apiName, 0755)

		bytes, _ := json.MarshalIndent(deployment, "", "  ")
		os.WriteFile(baseDir+"/"+apiName+"/"+deploymentName+".json", bytes, 0644)
	}

	return nil
}

func apiHubCleanLocal(flags *ApigeeFlags) error {
	var baseDir = "src/main/apihub"
	os.RemoveAll(baseDir)
	return nil
}

func apiHubClean(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	} else if flags.Region == "" {
		fmt.Println("No region given. Please specify a --region YOUR_REGION flag.")
		return nil
	}

	fmt.Println("Removing all API Hub APIs for project " + flags.Project + "...")

	if flags.Token == "" {
		var token *oauth2.Token
		scopes := []string{
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, scopes...)

		if err == nil {
			token, err = credentials.TokenSource.Token()

			if err == nil {
				flags.Token = token.AccessToken
			}
		}
	}

	apis := getApiHubApis(flags.Project, flags.Region, flags.Token)
	for _, api := range apis.Apis {
		if flags.ApiName == "" || strings.HasSuffix(api.Name, "/"+flags.ApiName) {
			fmt.Println("Deleting " + api.Name + "...")
			deleteApiHubApi(api.Name, flags.Token)
		}
	}

	deployments := getApiHubDeployments(flags.Project, flags.Region, flags.Token)
	for _, deployment := range deployments.Deployments {
		fmt.Println("Deleting " + deployment.Name + "...")
		deleteApiHubDeployment(deployment.Name, flags.Token)
	}

	return nil
}

func getApiHubApis(project string, region string, token string) HubApis {
	var apis HubApis

	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+project+"/locations/"+region+"/apis", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &apis)
			//fmt.Println(string(body))
		}
	}

	return apis
}

func getApiHubApiVersions(project string, region string, api string, token string) HubApiVersions {
	var versions HubApiVersions

	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+project+"/locations/"+region+"/apis/"+api+"/versions", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &versions)
			//fmt.Println(string(body))
		}
	}

	return versions
}

func getApiHubApiVersionSpecs(project string, region string, api string, version string, token string) HubApiVersionSpecs {
	var specs HubApiVersionSpecs

	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+project+"/locations/"+region+"/apis/"+api+"/versions/"+version+"/specs", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &specs)
			//fmt.Println(string(body))
		}
	}

	return specs
}

func getApiHubApiVersionSpecContents(project string, region string, api string, version string, spec string, token string) HubContents {
	var contents HubContents

	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+project+"/locations/"+region+"/apis/"+api+"/versions/"+version+"/specs/"+spec+":contents", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &contents)
			//fmt.Println(string(body))
		}
	}

	return contents
}

func deleteApiHubApi(api string, token string) {
	req, _ := http.NewRequest(http.MethodDelete, "https://apihub.googleapis.com/v1/"+api+"?force=true", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error deleting Apigee API: " + err.Error())
	}
}

func getApiHubDeployments(project string, region string, token string) HubApiDeployments {
	var deployments HubApiDeployments

	req, _ := http.NewRequest(http.MethodGet, "https://apihub.googleapis.com/v1/projects/"+project+"/locations/"+region+"/deployments", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &deployments)
			//fmt.Println(string(body))
		}
	}

	return deployments
}

func deleteApiHubDeployment(deployment string, token string) {
	req, _ := http.NewRequest(http.MethodDelete, "https://apihub.googleapis.com/v1/"+deployment, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error deleting API Hub Deployment: " + err.Error())
	}
}
