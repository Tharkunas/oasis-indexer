# Oasis Indexer

[![ci-lint](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-lint.yaml)
[![ci-test](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml/badge.svg)](https://github.com/oasislabs/oasis-indexer/actions/workflows/ci-test.yaml)

The official indexer for the Oasis Network.

## Docker Development

You can create and run the Oasis Indexer with [`docker compose`](https://docs.docker.com/compose/).
Keep reading to get started, or take a look at our [Docker docs](https://github.com/oasislabs/oasis-indexer/blob/main/docker/README.md) for more detail.

**Configuration**

Download the current network's [genesis document](https://docs.oasis.dev/oasis-core/consensus/genesis)
to the `docker/node/etc` directory. You will need this to run the Oasis Node container.

**Build**

## We are doing a Linux system update.
```
sudo apt-get update && apt-get upgrade -y
```

## We install the necessary libraries
```
sudo apt install curl tar wget clang pkg-config libssl-dev jq build-essential bsdmainutils git make ncdu gcc git jq chrony unzip liblz4-tool -y
```

From the repository root, you can run:
```
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

## We are installing Docker.
```
curl -SL https://github.com/docker/compose/releases/download/v2.5.0/docker-compose-linux-x86_64 -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
sudo ln -s /usr/local/bin/docker-compose /usr/bin/docker-compose
```


**Run**

From the repository root, you can run:
```
sudo systemctl enable --now docker
```

The analyzer will run migrations on start based on files in `storage/migrations`.
See [Generating Migrations](#generating-migrations) for information on generating new migrations.

**Query**

Now you can query the Oasis Indexer API
```sh
$ curl -X GET http://0.0.0.0:8008/v1
```

For a full list of endpoints see our [API docs](https://github.com/oasislabs/oasis-indexer/blob/main/api/README.md).

## Local Development

Below are instructions for running the Oasis Indexer locally, without Docker.

### Oasis Node

You will need to run a local [node](https://docs.oasis.dev/general/run-a-node/set-up-your-node/run-non-validator) for development purposes.
You will need to set the Unix socket in the `config/local-dev.yaml` file while running an instance of the Oasis Indexer.
For example, this will be `unix:/node/data/internal.sock` in Docker.

### Database

You will need to run a local [PostgreSQL DB](https://www.postgresql.org/).

For example, you can start a local [Docker](https://hub.docker.com/_/postgres) instance of Postgres with:
```
make postgres
```

### Indexer

You should be able to `make oasis-indexer` and run `./oasis-indexer --config config/local-dev.yml` from the repository root.
This will start the entire indexer, but you can start each of its constituent services independently as well.
See `./oasis-indexer --help` for more details.

Once the indexer has started, you can query the Oasis Indexer API
```sh
$ curl -X GET http://localhost:8008/v1
```

## Generating Migrations

The Oasis Indexer supports generating SQL migrations from a genesis document to initialize indexed state.
You can do so as follows:

```sh
oasis-indexer generate \
  --generator.genesis_file path/to/your/genesis.json
  --generator.migration_file storage/migrations/nnnn_example.up.sql
```

or directly from a running node

```sh
oasis-indexer generate \
  --generator.network_config_file path/to/your/config.yaml
  --generator.migration_file storage/migrations/nnnn_example.up.sql
```

See our [naming convention](https://github.com/oasislabs/oasis-indexer/blob/main/storage/migrations/README.md#naming-convention) for how to aptly name your migrations.
