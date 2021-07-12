#!/usr/bin/env bash

set -e

RUNTIME_FEATURE=$1
TMP_DIR=$(mktemp -d -t artemis-benchmark-XXX)

if [[ "$RUNTIME_FEATURE" == "with-snowbridge-runtime" ]]
then
    RUNTIME_DIR="runtime/snowbridge"
elif [[ "$RUNTIME_FEATURE" == "with-rococo-runtime" ]]
then
    RUNTIME_DIR="runtime/rococo"
else
    echo "Missing or invalid runtime feature argument. Pass either \"with-snowbridge-runtime\" or \"with-rococo-runtime\"."
    exit 1
fi

benchmark_pallets()
{
    echo "Building runtime with features $RUNTIME_FEATURE,runtime-benchmarks"

    cargo build --release \
        --no-default-features \
        --features runtime-benchmarks,$RUNTIME_FEATURE

    echo "Generating benchmark spec at $TMP_DIR/spec.json"

    target/release/artemis build-spec > $TMP_DIR/spec.json
    # Initialize dot-app account with enough DOT for benchmarks
    DOT_MODULE_ENDOWMENT="[
        \"5EYCAe5jHEQsVPTRQqy6NCeG71Hz1EVXikZxTkr67fM8j2Rd\",
        1152921504606846976
    ]"
    node ../test/scripts/helpers/overrideParachainSpec.js $TMP_DIR/spec.json \
        genesis.runtime.palletBalances.balances.0 "$DOT_MODULE_ENDOWMENT"

    PALLETS="assets basic_channel::inbound dot_app erc20_app erc721_app eth_app frame_system
        incentivized_channel::inbound incentivized_channel::outbound pallet_balances
        pallet_collective pallet_timestamp pallet_utility verifier_lightclient"

    echo "Generating weights module for $RUNTIME_DIR with pallets $PALLETS"

    echo "pub mod constants;" >> $TMP_DIR/mod.rs
    echo "" >> $TMP_DIR/mod.rs

    for pallet in $PALLETS
    do
        MODULE_NAME="$(tr -s [:] _ <<< $pallet)_weights"
        target/release/artemis benchmark \
            --chain $TMP_DIR/spec.json \
            --execution wasm \
            --wasm-execution compiled \
            --pallet "${pallet}" \
            --extrinsic "*" \
            --repeat 20 \
            --steps 50 \
            --output $RUNTIME_DIR/src/weights/$MODULE_NAME.rs
        echo "pub mod $MODULE_NAME;" >> $TMP_DIR/mod.rs
    done

    mv $TMP_DIR/mod.rs $RUNTIME_DIR/src/weights/mod.rs

    echo "Done generating extrinsic weights"
}

benchmark_node()
{
    echo "Benchmarking node. This can take 1 to 2 hours"

    cargo install node-bench --version 0.8.0
    yarn global add handlebars-cmd@0.1.4

    echo "[
        $(node-bench -j ::trie::read::large),
        $(node-bench -j ::trie::write::large),
        $(node-bench -j ::node::import::wasm::sr25519::noop::rocksdb::custom --transactions 5000),
        $(node-bench -j ::node::import::wasm::sr25519::noop::rocksdb::empty)
    ]" | node scripts/helpers/parseNodeBenchOutput.js 5000 > $TMP_DIR/node_bench_results.json

    echo "Generating weight constants for node"

    handlebars $TMP_DIR/node_bench_results.json \
        < scripts/helpers/weight-constants-template.hbs \
        > $RUNTIME_DIR/src/weights/constants.rs

    echo "Done generating node weights"
}

benchmark_pallets
benchmark_node
