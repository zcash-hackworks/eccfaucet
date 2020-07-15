# zfaucet

ZFaucet is software to request zcash testnet funds.

## Requirements

A fully synced zcashd node is required with RPC access.

## Configuration

All configuration is done through environmental variables.

Copy the template file and edit the values.

`cp .envtemplate .env`

## Build a binary

`make build`

## Build Docker iamge

`docker build .`

## Run binary

From the direcotry with the `.env` file.
`./zfaucet`

## Run a Docker image

From the direcotry with the `.env` file.
```
docker build . -t zfaucet \
&&  docker run --env-file ./.env --rm -ti -p 3000:3000 zfaucet
```