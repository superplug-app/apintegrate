package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type ApigeeProxies struct {
	Proxies []ApigeeApi `json:"proxies"`
}

type ApigeeApi struct {
	Name         string   `json:"name"`
	Revision     []string `json:"revision"`
	ApiProxyType string   `json:"apiProxyType"`
}

type ApigeeEnvironment struct {
	Proxies     []ApigeeEnvironmentProxy `json:"proxies"`
	SharedFlows []ApigeeEnvironmentProxy `json:"sharedflows"`
}

type ApigeeEnvironmentProxy struct {
	Name string `json:"name"`
}

type ApigeeDevelopers struct {
	Developers []ApigeeDeveloper `json:"developer"`
}

type ApigeeDeveloper struct {
	Email     string `json:"email"`
	UserName  string `json:"userName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type ApigeeDeveloperApp struct {
	DeveloperEmail string   `json:"developerEmail"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	ApiProducts    []string `json:"apiProducts"`
	ExpiryType     string   `json:"expiryType"`
}

type ApigeeProducts struct {
	Products []ApigeeProduct `json:"apiProduct"`
}

type ApigeeProduct struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Scopes       []string `json:"scopes"`
	Environments []string `json:"environments"`
	ApiResources []string `json:"apiResources"`
	Proxies      []string `json:"proxies"`
}

type ApigeeFlags struct {
	Project        string `name:"project" description:"The Google Cloud project that Apigee is running in."`
	Region         string `name:"region" description:"The Google Cloud region for a command."`
	Token          string `name:"token" description:"The Google access token to call Apigee with."`
	ApiName        string `name:"api" description:"A specific Apigee API."`
	Environment    string `name:"environment" description:"A specific Apigee environment."`
	ApiProduct     string `name:"product" description:"A specific Apigee product."`
	DeveloperEmail string `name:"developerEmail" description:"A specific Apigee developer email."`
	ServiceAccount string `name:"serviceAccount" description:"A service account email to use for Apigee deployments."`
}

func apigeeStatus(flags *ApigeeFlags) PlatformStatus {
	var status PlatformStatus
	if flags.Project == "" {
		status.Connected = false
		status.Message = "No project given, cannot connect to Apigee. Please specify a --project YOUR_PROJECT_ID flag."
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
	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+flags.Project+"/apis?includeRevisions=true", nil)
	req.Header.Add("Authorization", "Bearer "+flags.Token)

	var apis ApigeeProxies
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &apis)
		}

		if resp.StatusCode == 200 {
			status.Connected = true
			status.Message = "Connected to Apigee, " + strconv.Itoa(len(apis.Proxies)) + " APIs found in project " + flags.Project + "."
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

func apigeeExport(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given, cannot export Apigee APIs. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	fmt.Println("Exporting Apigee APIs for project " + flags.Project + "...")
	var baseDir = "src/main/apigee/apiproxies"

	var environment ApigeeEnvironment
	if flags.Environment != "" {
		// Create dir if it does not exist
		os.MkdirAll("src/main/apigee/environments/"+flags.Environment, 0755)

		// Open deployments.json file
		deploymentsFile, err := os.Open("src/main/apigee/environments/" + flags.Environment + "/deployments.json")
		if err != nil {
			environment = ApigeeEnvironment{Proxies: []ApigeeEnvironmentProxy{}, SharedFlows: []ApigeeEnvironmentProxy{}}
		} else {
			byteValue, _ := io.ReadAll(deploymentsFile)
			json.Unmarshal(byteValue, &environment)
		}
		defer deploymentsFile.Close()
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

	apis := getApigeeApis(flags.Project, flags.Token)
	// apisOutput, _ := json.Marshal(apis)
	// fmt.Println(string(apisOutput))

	if apis.Proxies != nil {
		os.MkdirAll(baseDir, 0755)
		for _, api := range apis.Proxies {
			if flags.ApiName == "" || flags.ApiName == api.Name {
				fmt.Println("Exporting " + api.Name + "...")
				bundle := getApigeeApiBundle(flags.Project, api.Name, api.Revision[0], flags.Token)

				if bundle != nil {
					err := os.WriteFile(baseDir+"/"+api.Name+".zip", bundle, 0644)
					if err != nil {
						panic(err)
					}

					// extract zip file
					unzipApigeeBundle(baseDir, api.Name)

					err = os.Remove(baseDir + "/" + api.Name + ".zip")
					if err != nil {
						panic(err)
					}

					// add to deployments.json if not already there
					foundProxy := false
					for _, value := range environment.Proxies {
						if value.Name == api.Name {
							foundProxy = true
						}
					}
					if !foundProxy {
						// add to deployments.json
						environment.Proxies = append(environment.Proxies, ApigeeEnvironmentProxy{Name: api.Name})
					}
				}
			}
		}

		if flags.Environment != "" {
			// write deployments.json
			bytes, _ := json.MarshalIndent(environment, "", "  ")
			os.WriteFile("src/main/apigee/environments/"+flags.Environment+"/deployments.json", bytes, 0644)
		}
	}

	return nil
}

func apigeeImport(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	fmt.Println("Importing Apigee APIs to project " + flags.Project + "...")
	var baseDir = "src/main/apigee/apiproxies"
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
				os.Chdir(baseDir + "/" + e.Name())
				zipApigeeBundle(e.Name())
				err := createApigeeApi(flags.Project, flags.Token, e.Name())
				if err != nil {
					fmt.Println("Error importing Apigee API: " + err.Error())
				}
				os.Remove(e.Name() + ".zip")
				os.Chdir("../../../../..")
			}
		}
	}

	return nil
}

func apigeeDeploy(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	} else if flags.Environment == "" {
		fmt.Println("No Apigee environment given. Please specify an --environment YOUR_ENVIRONMENT flag.")
		return nil
	}

	fmt.Println("Deploying Apigee APIs to project " + flags.Project + "...")
	var baseDir = "src/main/apigee/apiproxies"
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
				latestVersion := getApigeeApiLatestVersion(flags.Project, flags.Token, e.Name())

				fmt.Println("Deploying " + e.Name() + " version " + latestVersion + " to environment " + flags.Environment + "...")

				deployApigeeApi(flags.Project, flags.Token, flags.Environment, e.Name(), latestVersion, flags.ServiceAccount)
			}
		}
	}

	return nil
}

func apigeeClean(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	fmt.Println("Removing all Apigee APIs for project " + flags.Project + "...")

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

	apis := getApigeeApis(flags.Project, flags.Token)
	for _, api := range apis.Proxies {
		if flags.ApiName == "" || flags.ApiName == api.Name {
			fmt.Println("Deleting " + api.Name + "...")
			deleteApigeeApi(flags.Project, flags.Token, api.Name)
		}
	}

	return nil
}

func apigeeDevelopersClean(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	fmt.Println("Removing all Apigee Developers for project " + flags.Project + "...")

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

	developers := getApigeeDevelopers(flags.Project, flags.Token)
	for _, developer := range developers.Developers {
		if flags.DeveloperEmail == "" || flags.DeveloperEmail == developer.Email {
			fmt.Println("Deleting " + developer.Email + "...")
			deleteApigeeDeveloper(flags.Project, flags.Token, developer.Email)
		}
	}

	return nil
}

func apigeeProductsClean(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	fmt.Println("Removing all Apigee Products for project " + flags.Project + "...")

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

	products := getApigeeApiProducts(flags.Project, flags.Token)

	fmt.Println("Found " + strconv.Itoa(len(products.Products)) + " products.")

	for _, product := range products.Products {
		if flags.ApiProduct == "" || flags.ApiProduct == product.Name {
			fmt.Println("Deleting " + product.Name + "...")
			deleteApigeeProduct(flags.Project, flags.Token, product.Name)
		}
	}

	return nil
}

func getApigeeApis(org string, token string) ApigeeProxies {
	var apis ApigeeProxies
	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+org+"/apiproducts", nil)
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

func getApigeeApiProducts(org string, token string) ApigeeProducts {
	var result ApigeeProducts
	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+org+"/apiproducts", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &result)
			//fmt.Println(string(body))
		}
	}

	return result
}

func getApigeeDevelopers(org string, token string) ApigeeDevelopers {
	var result ApigeeDevelopers
	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+org+"/developers", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &result)
			//fmt.Println(string(body))
		}
	}

	return result
}

func getApigeeApiBundle(org string, api string, revision string, token string) []byte {
	var bundle []byte

	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+org+"/apis/"+api+"/revisions/"+revision+"?format=bundle", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		bundle, _ = io.ReadAll(resp.Body)
	}

	return bundle
}

func unzipApigeeBundle(basePath string, name string) {
	zipBaseBath := basePath + "/" + name
	os.Mkdir(basePath+"/"+name, 0755)
	archive, err := zip.OpenReader(basePath + "/" + name + ".zip")
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(zipBaseBath, f.Name)

		if !strings.HasPrefix(filePath, filepath.Clean(zipBaseBath)+string(os.PathSeparator)) {
			fmt.Println("invalid file path")
			return
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			panic(err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			panic(err)
		}

		fileInArchive, err := f.Open()
		if err != nil {
			panic(err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			panic(err)
		}

		dstFile.Close()
		fileInArchive.Close()
	}
}

func zipApigeeBundle(name string) {
	file, err := os.Create(name + ".zip")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Ensure that `path` is not absolute; it should not start with "/".
		// This snippet happens to work because I don't use
		// absolute paths, but ensure your real-world code
		// transforms path into a zip-root relative path.
		f, err := w.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}

	err = filepath.Walk("apiproxy", walker)
	if err != nil {
		panic(err)
	}
}

func deleteApigeeApi(org string, token string, api string) {
	req, _ := http.NewRequest(http.MethodDelete, "https://apigee.googleapis.com/v1/organizations/"+org+"/apis/"+api, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error deleting Apigee API: " + err.Error())
	}
}

func createApigeeApi(org string, token string, name string) error {

	fileDir, _ := os.Getwd()
	fileName := name + ".zip"
	filePath := path.Join(fileDir, fileName)

	file, _ := os.Open(filePath)
	defer file.Close()

	//fmt.Println(filepath.Base(file.Name()))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filepath.Base(file.Name()))
	io.Copy(part, file)
	writer.Close()

	r, _ := http.NewRequest(http.MethodPost, "https://apigee.googleapis.com/v1/organizations/"+org+"/apis?name="+name+"&action=import", body)
	r.Header.Add("Content-Type", writer.FormDataContentType())
	r.Header.Add("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(r)

	if resp.StatusCode != 200 {
		fmt.Println("Error creating Apigee API: " + resp.Status)
	}

	return err
}

func deployApigeeApi(org string, token string, env string, name string, version string, serviceAccount string) error {

	url := "https://apigee.googleapis.com/v1/organizations/" + org + "/environments/" + env + "/apis/" + name + "/revisions/" + version + "/deployments?override=true"
	if serviceAccount != "" {
		url = url + "&serviceAccount=" + serviceAccount
	}

	r, _ := http.NewRequest(http.MethodPost, url, nil)
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(r)

	if resp.StatusCode != 200 {
		fmt.Println("Error deploying Apigee API: " + resp.Status)
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			fmt.Println(string(body))
		}
	}

	return err
}

func getApigeeApiLatestVersion(org string, token string, name string) string {
	var result string
	var apigeeApi ApigeeApi
	req, _ := http.NewRequest(http.MethodGet, "https://apigee.googleapis.com/v1/organizations/"+org+"/apis/"+name, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			json.Unmarshal(body, &apigeeApi)
			//fmt.Println(string(body))

			for _, value := range apigeeApi.Revision {
				v, _ := strconv.Atoi(value)
				r, _ := strconv.Atoi(result)

				if v > r {
					result = value
				}
			}
		}
	}

	return result
}

func deleteApigeeDeveloper(org string, token string, email string) {
	req, _ := http.NewRequest(http.MethodDelete, "https://apigee.googleapis.com/v1/organizations/"+org+"/developers/"+email, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error deleting Apigee developer: " + err.Error())
	}
}

func deleteApigeeProduct(org string, token string, name string) {
	req, _ := http.NewRequest(http.MethodDelete, "https://apigee.googleapis.com/v1/organizations/"+org+"/apiproducts/"+name, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error deleting Apigee product: " + err.Error())
	}
}

func initApigeeTest(flags *ApigeeFlags) error {
	if flags.Project == "" {
		fmt.Println("No project given, cannot init test data. Please specify a --project YOUR_PROJECT_ID flag.")
		return nil
	}

	if flags.Environment == "" {
		fmt.Println("No environment given, cannot init test data. Please specify an environment with the --environment YOUR_ENVIRONMENT flag.")
		return nil
	}

	// create test developer
	developer := ApigeeDeveloper{Email: "test@example.com", UserName: "testUser", FirstName: "Test", LastName: "User"}
	developers := []ApigeeDeveloper{developer}

	// create test product
	product := ApigeeProduct{Name: "test_product", DisplayName: "Test Product", Scopes: []string{}, Environments: []string{}, ApiResources: []string{"/"}, Proxies: []string{}}
	products := []ApigeeProduct{product}

	// create test developerapp
	app := ApigeeDeveloperApp{DeveloperEmail: developer.Email, Name: "test_app", DisplayName: "Test App", ApiProducts: []string{"test_product"}, ExpiryType: "never"}
	apps := []ApigeeDeveloperApp{app}

	// load environment deployments.json
	var environment ApigeeEnvironment
	deploymentsFile, err := os.Open("src/main/apigee/environments/" + flags.Environment + "/deployments.json")
	if err != nil {
		environment = ApigeeEnvironment{Proxies: []ApigeeEnvironmentProxy{}, SharedFlows: []ApigeeEnvironmentProxy{}}
	} else {
		byteValue, _ := io.ReadAll(deploymentsFile)
		json.Unmarshal(byteValue, &environment)
	}
	defer deploymentsFile.Close()

	// create test directory
	os.MkdirAll("src/main/apigee/tests/"+flags.Environment, 0755)

	// write developers
	bytes, _ := json.MarshalIndent(developers, "", "  ")
	os.WriteFile("src/main/apigee/tests/"+flags.Environment+"/developers.json", bytes, 0644)

	for _, proxy := range environment.Proxies {
		products[0].Proxies = append(products[0].Proxies, proxy.Name)
	}

	// write products
	bytes, _ = json.MarshalIndent(products, "", "  ")
	os.WriteFile("src/main/apigee/tests/"+flags.Environment+"/products.json", bytes, 0644)

	// write apps
	bytes, _ = json.MarshalIndent(apps, "", "  ")
	os.WriteFile("src/main/apigee/tests/"+flags.Environment+"/developerapps.json", bytes, 0644)

	return nil
}
