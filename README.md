# oasync
A tool to help synchronize open APIs between platforms. 

## Example usage

These commands can offramp APIs from an Azure APIM service, and then onramp them into Apigee API Hub.

```sh
# source env variables
source .env

# 1. azure export all apis from an Azure APIM service to the local filesystem (./data directory will be created)
oasync azure apis export --subscription $AZURE_SUBSCRIPTION_ID --resourcegroup $AZURE_RESOURCE_GROUP --name $AZURE_SERVICE_NAME

# 2. azure apis offramp exported APIs to a generic format that can be onramped to API Hub (./data directory will be created)
oasync azure apis offramp  --subscription $AZURE_SUBSCRIPTION_ID --resourcegroup $AZURE_RESOURCE_GROUP --name $AZURE_SERVICE_NAME

# 3. apihub apis onramp from generic format to API Hub format (./data directory will be created)
oasync apihub apis onramp --project $APIGEE_PROJECT_ID --region $APIGEE_REGION

# 4. apihub apis import from onramped files to API Hub
oasync apihub apis import --project $APIGEE_PROJECT_ID --region $APIGEE_REGION
```

You can also start a web server to run the commands, for example deployed in Cloud Run and triggered through a Cloud Scheduler timer to keep the services in sync.

```sh
# start web service
oasync ws start

# open http://localhost:8080/docs to see API docs

# call the v1/apim/sync API to do a complete sync from Azure to API Hub (equivalent of the four commands above)
curl --request POST \
  --url http://localhost:8080/v1/oasync/sync \
  --header 'Accept: application/json, application/problem+json' \
  --header 'Content-Type: application/json' \
  --data '{
  "offramp": "azure",
  "onramp": "apihub"
}'
```
The docs are available at http://0:8080/docs after starting the web server.

## Getting started

Install the binary `oasync` to your `/usr/bin` directory.

```sh
curl -L https://raw.githubusercontent.com/superplug-app/oasync/main/install.sh | sh -
```

## Deploy service to Google Cloud Run

It's quite simple to deploy the service to Google Cloud Run using the `gcloud` CLI.

```sh
# first update the 1.env.sh file with your own environment variables to authenticate to Azure & AWS,
# and then source the updated file.
source 1.env.sh

# then make sure you are authenticated to gcloud, and call the deployment script.
./2.deploy.service.sh
```