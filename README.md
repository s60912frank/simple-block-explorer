# Simple Block Explorer
This is a simple block explorer that should work with any EVM-compatible JSON RPC, it will index blocks and transactions, let you get block or transaction data by hash

## Available restful APIs
* `GET /blocks`
    * Get latest `n` blocks in database
    * Accept query parameter `limit`, e.g `/blocks?limit=15`, default is `1`
    * Does not contains any transaction in this block

* `GET /blocks/:id`
    * Get block using block hash or block number, for example:
        * `GET /blocks/12345`
        * `GET /blocks/0x4d0431358bfd87ddb62f38c50dc311d5fa10e5b37da83bdea2d0987e9ac75073`
    * Will list all transaction hash in this block

* `GET /transaction/:txHash`
    * Get transaction using transaction hash, for example:
        * `GET /transaction/0x4d0431358bfd87ddb62f38c50dc311d5fa10e5b37da83bdea2d0987e9ac75073`
    * Will include event logs in the transaction
    * If `receipt_ready` in response is `false`, it means that this transaction does not include event logs yet, please wait event logs to be updated

## How to run the explorer
* Prerequisite
    * Docker engine
    * Docker compose
* Configure
    * copy `.env.example` to `.env`
    * change environment variables as needed, default one already working
* Build
    ```
    docker compose build
    ```
* Run
    ```
    docker compose up -d
    ```
* Test API
    * If you did not change `PORT` in `.env`, you should able to get latest blocks by open `http://localhost:8080/blocks`

## Development
* Run postgres and redis server
    ```
    docker compose -f ./development/docker-compose=dev.yml up -d
    ```
* Run application
    ```
    env $(cat ./.env | xargs) go run ./pkg/main.go
    ```

