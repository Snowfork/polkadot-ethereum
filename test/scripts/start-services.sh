#!/usr/bin/env bash
set -eu

# config directory
configdir=/tmp/snowbridge-e2e-config
rm -rf $configdir
mkdir $configdir

# kill all potentially old processes
kill $(ps -aux | grep -e polkadot/target -e ganache-cli -e release/artemis | awk '{print $2}') || true

start_ganache()
{
    echo "Starting Ganache"

    yarn run ganache-cli \
        --port=8545 \
        --networkId=344 \
        --deterministic \
        --db $configdir/ganachedb \
        --mnemonic='stone speak what ritual switch pigeon weird dutch burst shaft nature shove' \
        --gasLimit=8000000 \
        >ganache.log 2>&1 &

    scripts/wait-for-it.sh -t 32 localhost:8545
    sleep 5
}

restart_ganache()
{
    echo "Restarting Ganache with a slower block time"

    kill $(ps -aux | grep -e ganache-cli | awk '{print $2}') || true

    yarn run ganache-cli \
        --port=8545 \
        --blockTime=6 \
        --networkId=344 \
        --deterministic \
        --db $configdir/ganachedb \
        --mnemonic='stone speak what ritual switch pigeon weird dutch burst shaft nature shove' \
        --gasLimit=8000000 \
        >ganache.log 2>&1 &

    scripts/wait-for-it.sh -t 32 localhost:8545
    sleep 5
}

deploy_contracts()
{
    echo "Deploying contracts"
    pushd ../ethereum

    truffle deploy --network e2e_test

    echo "Generating configuration from contracts"
    truffle exec scripts/dumpTestConfig.js $configdir --network e2e_test
    popd

    echo "Wrote configuration to $configdir"
}


start_polkadot_launch()
{
    echo "Building parachain and starting polkadot launch"
    pushd ../parachain
    bin=$(pwd)/target/release/artemis

    cargo build --release --no-default-features --features with-local-runtime

    echo "Generating Parachain spec"
    target/release/artemis build-spec --disable-default-bootnode > $configdir/spec.json

    echo "Inserting Ganache chain info into genesis spec"
    ethereum_initial_header=$(curl http://localhost:8545 \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params": ["latest", false],"id":1}' \
        | node ../test/scripts/helpers/transformEthHeader.js)
    node ../test/scripts/helpers/overrideParachainSpec.js $configdir/spec.json \
        genesis.runtime.verifierLightclient.initialDifficulty 0x0 \
        genesis.runtime.verifierLightclient.initialHeader "$ethereum_initial_header" \
        genesis.runtime.parachainInfo.parachainId 1000 \
        para_id 1000

    if [ $# -eq 1 ] && [ $1 = "malicious" ]; then
        jq '.genesis.runtime.dotApp.address = "0x433488cec14C4478e5ff18DDC7E7384Fc416f148"' \
          $configdir/spec.json > spec.malicious.json && \
          mv spec.malicious.json $configdir/spec.json
    fi

    echo "Writing Polkadot configuration"
    polkadotbinary=/tmp/polkadot/target/release/polkadot
    if [[ -f ../test/.env ]]; then
        source ../test/.env
    fi

    if [ $# -eq 1 ] && [ $1 = "duplicate" ];
    then
        echo "Generating second parachain spec"
        target/release/artemis build-spec --disable-default-bootnode > $configdir/spec2.json

        node ../test/scripts/helpers/overrideParachainSpec.js $configdir/spec2.json \
            genesis.runtime.verifierLightclient.initialDifficulty 0x0 \
            genesis.runtime.verifierLightclient.initialHeader "$ethereum_initial_header" \
            genesis.runtime.parachainInfo.parachainId 1001 \
            para_id 1001
        jq  -s '.[0] * .[1]' config-dup.json ../test/config/launchConfigOverridesDup.json \
            | jq ".parachains[0].bin = \"$bin\"" \
            | jq ".parachains[0].chain = \"$configdir/spec.json\"" \
            | jq ".parachains[1].bin = \"$bin\"" \
            | jq ".parachains[1].chain = \"$configdir/spec2.json\"" \
            | jq ".relaychain.bin = \"$polkadotbinary\"" \
            > $configdir/polkadotLaunchConfig.json
    else
        jq  -s '.[0] * .[1]' config.json ../test/config/launchConfigOverrides.json \
            | jq ".parachains[0].bin = \"$bin\"" \
            | jq ".parachains[0].chain = \"$configdir/spec.json\"" \
            | jq ".relaychain.bin = \"$polkadotbinary\"" \
            > $configdir/polkadotLaunchConfig.json
    fi

    polkadot-launch $configdir/polkadotLaunchConfig.json &

    popd

    scripts/wait-for-it.sh -t 120 localhost:11144
}

start_relayer()
{
    echo "Starting Relay"
    logfile=$(pwd)/relay.log
    pushd ../relayer

    mage build

    export SNOWBRIDGE_BEEFY_KEY="0x935b65c833ced92c43ef9de6bff30703d941bd92a2637cb00cfad389f5862109"
    export SNOWBRIDGE_MESSAGE_KEY="0x8013383de6e5a891e7754ae1ef5a21e7661f1fe67cd47ca8ebf4acd6de66879a"
    export SNOWBRIDGE_PARACHAIN_KEY="//Relay"
    export SNOWBRIDGE_RELAYCHAIN_KEY="//Alice"

    build/artemis-relay run --config $configdir/config.toml >$logfile 2>&1 &

    popd
    echo "Relay PID: $!"

}

cleanup() {
    kill $(jobs -p)
    kill $(ps -aux | grep -e polkadot/target -e ganache-cli -e release/artemis | awk '{print $2}') || true
}

trap cleanup SIGINT SIGTERM EXIT

start_ganache
deploy_contracts
if [ $# -eq 1 ];
then
    start_polkadot_launch $1
else
    start_polkadot_launch
fi
restart_ganache
echo "Waiting for consensus between polkadot and parachain"
sleep 60
start_relayer

echo "Process Tree:"
pstree $$

sleep 3
until $(grep "Syncing headers starting..." $(pwd)/relay.log > /dev/null); do
    echo "Waiting for relayer to generate the DAG cache. This can take up to 20 minutes."
    sleep 20
done

until $(grep "Done retrieving finalized headers" $(pwd)/relay.log > /dev/null); do
    echo "Waiting for relayer to sync headers..."
    sleep 5
done

echo "System has been initialized"

wait
