package main

import (
	"log"

	"github.com/leaanthony/clir"
)

type GeneralApi struct {
	Name                string `json:"name"`
	DisplayName         string `json:"displayName"`
	Version             string `json:"version"`
	Description         string `json:"description"`
	OwnerEmail          string `json:"ownerEmail"`
	OwnerName           string `json:"ownerName"`
	DocumentationUrl    string `json:"documentationUrl"`
	GatewayUrl          string `json:"gatewayUrl"`
	BasePath            string `json:"basePath"`
	PlatformId          string `json:"platformId"`
	PlatformName        string `json:"platformName"`
	PlatformResourceUri string `json:"platformResourceUri"`
}

type PlatformStatus struct {
	Connected bool   `json:"connected"`
	Message   string `json:"message"`
}

type GeneralFlags struct {
	ApiName string `name:"api" description:"A specific Azure API Management API."`
}

func main() {
	// Create new cli
	cli := clir.NewCli("oasync", "A sync tool for open APIs.", "v0.2.0")

	generalCommand := cli.NewSubCommand("general", "'apis cleanlocal'...")
	generalApisCommand := generalCommand.NewSubCommand("apis", "Functions for General API resources.")
	generalApisCommand.NewSubCommandFunction("cleanlocal", "Removes all APIs from offramped general definitions in local storage.", generalCleanLocal)

	webServerCommand := cli.NewSubCommand("ws", "'start'...")
	webServerCommand.NewSubCommandFunction("start", "Start a web server to listen for commands.", webServerStart)

	apigeeCommand := cli.NewSubCommand("apigee", "'apis export', 'apis import', 'apis deploy', 'apis clean'...")
	apigeeApisCommand := apigeeCommand.NewSubCommand("apis", "'apis export', 'apis import', 'apis deploy', 'apis clean'...")
	apigeeApisCommand.NewSubCommandFunction("export", "Exports Apigee APIs from a given project.", apigeeExport)
	apigeeApisCommand.NewSubCommandFunction("import", "Imports APIs to an Apigee project.", apigeeImport)
	apigeeApisCommand.NewSubCommandFunction("deploy", "Deploys APIs to an Apigee project and environment.", apigeeDeploy)
	apigeeApisCommand.NewSubCommandFunction("clean", "Removes all of the Apigee APIs from a given project.", apigeeClean)
	apigeeTestCommand := apigeeCommand.NewSubCommand("test", "Local test commands.")
	apigeeTestCommand.NewSubCommandFunction("init", "Initializes local test data for an environment.", initApigeeTest)
	apigeeProductsCommand := apigeeCommand.NewSubCommand("products", "Functions for Apigee products.")
	apigeeProductsCommand.NewSubCommandFunction("clean", "Removes all products from a given project.", apigeeProductsClean)
	apigeeDevelopersCommand := apigeeCommand.NewSubCommand("developers", "Functions for Apigee developers.")
	apigeeDevelopersCommand.NewSubCommandFunction("clean", "Removes all developers and apps from a given project.", apigeeDevelopersClean)
	apigeeTestCommand.NewSubCommandFunction("init", "Initializes local test data for an environment.", initApigeeTest)

	apiHubCommand := cli.NewSubCommand("apihub", "'apis export', 'apis import', 'apis onramp', 'apis clean'...")
	apiHubApisCommand := apiHubCommand.NewSubCommand("apis", "'apis export', 'apis import', 'apis onramp', 'apis clean'...")
	apiHubApisCommand.NewSubCommandFunction("onramp", "Onramps APIs from general to API Hub.", apiHubOnramp)
	apiHubApisCommand.NewSubCommandFunction("import", "Imports APIs to API Hub.", apiHubImport)
	apiHubApisCommand.NewSubCommandFunction("export", "Exports APIs from API Hub.", apiHubExport)
	apiHubApisCommand.NewSubCommandFunction("clean", "Removes all APIs from API Hub.", apiHubClean)
	apiHubApisCommand.NewSubCommandFunction("cleanlocal", "Removes all API Hub APIs from local storage.", apiHubCleanLocal)

	azureCommand := cli.NewSubCommand("azure", "'apis export', 'apis offramp', 'apis cleanlocal'...")
	azureCommand.NewSubCommandFunction("export", "'apis export', 'apis offramp', 'apis cleanlocal'...", azureServiceExport)
	azureApisCommand := azureCommand.NewSubCommand("apis", "Functions for Azure API Management API resources.")
	azureApisCommand.NewSubCommandFunction("export", "Exports Azure API Management APIs.", azureExportMin)
	azureApisCommand.NewSubCommandFunction("offramp", "Migrates Azure API Management APIs out to general.", azureOfframp)
	azureApisCommand.NewSubCommandFunction("cleanlocal", "Removes all exported Azure APIs from local storage.", azureCleanLocal)

	awsCommand := cli.NewSubCommand("aws", "'apis export', 'apis offramp', 'apis cleanlocal'...")
	awsApisCommand := awsCommand.NewSubCommand("apis", "'apis export', 'apis offramp', 'apis cleanlocal'...")
	awsApisCommand.NewSubCommandFunction("export", "Exports AWS API Gateway APIs.", awsExportMin)
	awsApisCommand.NewSubCommandFunction("offramp", "Offramp AWS API Gateway APIs.", awsOfframp)
	awsApisCommand.NewSubCommandFunction("cleanlocal", "Removes all exported AWS APIs from local storage.", awsCleanLocal)

	err := cli.Run()

	if err != nil {
		// We had an error
		log.Fatal(err)
	}
}
